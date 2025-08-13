package owctrl

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

/* ───── DTO и контракты ───── */

type DeviceFields struct {
	UUID      string
	Key       string
	Name      string
	Backend   string
	MAC       string
	Status    string
	UpdatedAt time.Time
}

type Store interface {
	UpsertByKey(key string, d DeviceFields) (DeviceFields, bool)
	FindByUUID(id string) (DeviceFields, bool)
	UpdateStatus(id, status string) error
}

type ConfigBuilder interface {
	BuildConfig(d DeviceFields) (map[string]string, error)
}

/* ───── in-memory store (fallback) ───── */

type memStore struct {
	byUUID map[string]DeviceFields
	byKey  map[string]string
	mu     sync.RWMutex
}

func NewMemStore() *memStore {
	return &memStore{byUUID: map[string]DeviceFields{}, byKey: map[string]string{}}
}

func (m *memStore) UpsertByKey(key string, d DeviceFields) (DeviceFields, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, ok := m.byKey[key]; ok {
		ex := m.byUUID[id]
		if d.Name != "" {
			ex.Name = d.Name
		}
		if d.Backend != "" {
			ex.Backend = d.Backend
		}
		if d.MAC != "" {
			ex.MAC = d.MAC
		}
		ex.UpdatedAt = time.Now()
		m.byUUID[id] = ex
		return ex, false
	}
	if d.UUID == "" {
		d.UUID = uuid.NewString()
	}
	d.Key = key
	d.UpdatedAt = time.Now()
	m.byUUID[d.UUID] = d
	m.byKey[key] = d.UUID
	return d, true
}
func (m *memStore) FindByUUID(id string) (DeviceFields, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.byUUID[id]
	return d, ok
}
func (m *memStore) UpdateStatus(id, st string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.byUUID[id]
	if !ok {
		return errors.New("not found")
	}
	d.Status = st
	d.UpdatedAt = time.Now()
	m.byUUID[id] = d
	return nil
}

/* ───── Контроллер ───── */

type Controller struct {
	store        Store
	sharedSecret string
	builder      ConfigBuilder
}

func NewController(sharedSecret string) *Controller {
	return &Controller{store: NewMemStore(), sharedSecret: sharedSecret}
}
func NewControllerWithStore(sharedSecret string, store Store) *Controller {
	if store == nil {
		store = NewMemStore()
	}
	return &Controller{store: store, sharedSecret: sharedSecret}
}
func NewControllerWithStoreAndBuilder(sharedSecret string, store Store, builder ConfigBuilder) *Controller {
	if store == nil {
		store = NewMemStore()
	}
	return &Controller{store: store, sharedSecret: sharedSecret, builder: builder}
}

func (c *Controller) setOWHeader(w http.ResponseWriter) {
	w.Header().Set("X-Openwisp-Controller", "true")
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

/* ───── Endpoints ───── */

// POST /controller/register(/)
func (c *Controller) handleRegister(w http.ResponseWriter, r *http.Request) {
	c.setOWHeader(w)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "cannot parse form", http.StatusBadRequest)
		return
	}
	secret := r.Form.Get("secret")
	if secret == "" || secret != c.sharedSecret {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	name := r.Form.Get("name")
	backend := r.Form.Get("backend")
	mac := r.Form.Get("mac_address")
	key := r.Form.Get("key")
	if key == "" {
		sum := sha256.Sum256([]byte(mac + "+" + secret))
		key = hex.EncodeToString(sum[:8])
	}
	dev, isNew := c.store.UpsertByKey(key, DeviceFields{Name: name, Backend: backend, MAC: mac})
	w.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(w,
		fmt.Sprintf("uuid: %s\nkey: %s\nhostname: %s\nis-new: %d\n",
			dev.UUID, key, dev.Name, btoi(isNew)))
}

// GET /controller/checksum/{uuid}(/)?key=...
func (c *Controller) handleChecksum(w http.ResponseWriter, r *http.Request) {
	c.setOWHeader(w)
	id := mux.Vars(r)["uuid"]
	key := r.URL.Query().Get("key")
	dev, ok := c.store.FindByUUID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if key == "" || key != dev.Key {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	files, err := c.buildFiles(dev)
	if err != nil {
		http.Error(w, "config build error", http.StatusInternalServerError)
		return
	}
	tgz := mustTarGz(files)
	sum := sha256.Sum256(tgz)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, hex.EncodeToString(sum[:])+"\n")
}

// GET /controller/download-config/{uuid}(/)?key=...
func (c *Controller) handleDownloadConfig(w http.ResponseWriter, r *http.Request) {
	c.setOWHeader(w)
	id := mux.Vars(r)["uuid"]
	key := r.URL.Query().Get("key")
	dev, ok := c.store.FindByUUID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if key == "" || key != dev.Key {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	files, err := c.buildFiles(dev)
	if err != nil {
		http.Error(w, "config build error", http.StatusInternalServerError)
		return
	}
	tgz := mustTarGz(files)
	sum := sha256.Sum256(tgz)
	shaHex := hex.EncodeToString(sum[:])
	etag := `"` + shaHex + `"`
	if inm := r.Header.Get("If-None-Match"); inm != "" && inm == etag {
		w.Header().Set("ETag", etag)
		w.Header().Set("X-Openwisp-Archive-Sha256", shaHex)
		w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("X-Openwisp-Archive-Sha256", shaHex)
	w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
	// максимально совместимый content-type
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=configuration.tar.gz")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(tgz)
}

// POST /controller/report-status/{uuid}(/)
func (c *Controller) handleReportStatus(w http.ResponseWriter, r *http.Request) {
	c.setOWHeader(w)
	id := mux.Vars(r)["uuid"]
	if err := r.ParseForm(); err != nil {
		http.Error(w, "cannot parse form", http.StatusBadRequest)
		return
	}
	key := r.Form.Get("key")
	status := strings.ToLower(r.Form.Get("status"))
	switch status {
	case "running", "applied", "ok", "updated", "error":
	default:
		http.Error(w, "bad status", http.StatusBadRequest)
		return
	}
	dev, ok := c.store.FindByUUID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if key == "" || key != dev.Key {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	_ = c.store.UpdateStatus(id, status)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok\n")
}

/* ───── Fallback build ───── */

func (c *Controller) buildFiles(d DeviceFields) (map[string]string, error) {
	if c.builder != nil {
		return c.builder.BuildConfig(d)
	}
	// минимальный набор
	hostname := safe(d.Name)
	if hostname == "" {
		hostname = d.MAC
	}
	return map[string]string{
		"etc/config/system":                      "config system 'system'\n  option hostname '" + hostname + "'\n  option timezone 'UTC'\n",
		"etc/openwisp/device.meta":               fmt.Sprintf("uuid=%s\nmac=%s\nbackend=%s\n", d.UUID, d.MAC, d.Backend),
		"etc/openwisp/managed_by_openwisp_go.md": "This device is managed by OpenWISP-Go controller.\n",
	}, nil
}

func safe(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "'", ""))
}

func tarGzFromMap(files map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	now := time.Now()

	for name, content := range files {
		h := &tar.Header{
			Name:    strings.TrimLeft(name, "/"),
			Mode:    0644,
			Size:    int64(len(content)),
			ModTime: now,
		}
		if err := tw.WriteHeader(h); err != nil {
			_ = tw.Close()
			_ = gw.Close()
			return nil, err
		}
		if _, err := io.Copy(tw, strings.NewReader(content)); err != nil {
			_ = tw.Close()
			_ = gw.Close()
			return nil, err
		}
	}
	_ = tw.Close()
	_ = gw.Close()
	return buf.Bytes(), nil
}

func mustTarGz(files map[string]string) []byte {
	b, err := tarGzFromMap(files)
	if err != nil {
		return []byte{}
	}
	return b
}

/* ───── Роуты ───── */

func RegisterRoutes(root *mux.Router, sharedSecret string) {
	ctrl := NewController(sharedSecret)
	register(root, ctrl)
}

func RegisterRoutesWithStore(root *mux.Router, sharedSecret string, store Store) {
	ctrl := NewControllerWithStore(sharedSecret, store)
	register(root, ctrl)
}

func RegisterRoutesWithStoreAndBuilder(root *mux.Router, sharedSecret string, store Store, builder ConfigBuilder) {
	ctrl := NewControllerWithStoreAndBuilder(sharedSecret, store, builder)
	register(root, ctrl)
}

func register(root *mux.Router, ctrl *Controller) {
	sub := root.PathPrefix("/controller").Subrouter()

	sub.HandleFunc("/register/", ctrl.handleRegister).Methods(http.MethodPost)
	sub.HandleFunc("/register", ctrl.handleRegister).Methods(http.MethodPost)

	sub.HandleFunc("/checksum/{uuid}/", ctrl.handleChecksum).Methods(http.MethodGet)
	sub.HandleFunc("/checksum/{uuid}", ctrl.handleChecksum).Methods(http.MethodGet)

	sub.HandleFunc("/download-config/{uuid}/", ctrl.handleDownloadConfig).Methods(http.MethodGet)
	sub.HandleFunc("/download-config/{uuid}", ctrl.handleDownloadConfig).Methods(http.MethodGet)

	sub.HandleFunc("/report-status/{uuid}/", ctrl.handleReportStatus).Methods(http.MethodPost)
	sub.HandleFunc("/report-status/{uuid}", ctrl.handleReportStatus).Methods(http.MethodPost)

	// debug для UI/проверки (опционально)
	sub.HandleFunc("/debug-config/{uuid}/", func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["uuid"]
		key := r.URL.Query().Get("key")
		dev, ok := ctrl.store.FindByUUID(id)
		if !ok || dev.Key != key {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		files, err := ctrl.buildFiles(dev)
		if err != nil {
			http.Error(w, "Build failed: "+err.Error(), http.StatusUnprocessableEntity)
			return
		}
		tgz := mustTarGz(files)
		sum := sha256.Sum256(tgz)
		type fileView struct {
			Path    string `json:"path"`
			Size    int    `json:"size"`
			Preview string `json:"preview"`
		}
		out := struct {
			SHA256 string     `json:"sha256"`
			Files  []fileView `json:"files"`
		}{SHA256: hex.EncodeToString(sum[:])}
		for p, c := range files {
			prev := c
			if len(prev) > 300 {
				prev = prev[:300]
			}
			out.Files = append(out.Files, fileView{Path: p, Size: len(c), Preview: prev})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}).Methods(http.MethodGet)
}

// json import
// (вверху файла добавь) "encoding/json"
