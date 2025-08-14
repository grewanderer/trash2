package owagent

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"time"

	"wisp/internal/repo"

	"github.com/gorilla/mux"
)

type Handler struct {
	ds            *repo.DeviceStore
	sharedSecret  string
	consistentKey bool
}

func New(ds *repo.DeviceStore, sharedSecret string, consistentKey bool) *Handler {
	return &Handler{ds: ds, sharedSecret: sharedSecret, consistentKey: consistentKey}
}

// Вспомогательно: обязателен заголовок для агента
func setOWHeader(w http.ResponseWriter) {
	w.Header().Set("X-Openwisp-Controller", "true")
}

// POST /controller/register/
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	setOWHeader(w)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "error: bad form", http.StatusBadRequest)
		return
	}
	in := repo.RegisterInput{
		SharedSecret:   r.FormValue("secret"),
		ExpectedSecret: h.sharedSecret,
		Name:           r.FormValue("name"),
		Model:          r.FormValue("backend"), // ← было Backend:, теперь Model:
		MAC:            r.FormValue("mac_address"),
		KeyOptional:    r.FormValue("key"),
		ConsistentKey:  h.consistentKey,
	}
	res, err := h.ds.Register(r.Context(), in)
	if err != nil {
		switch err {
		case repo.ErrBadSecret:
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintln(w, "error: unrecognized secret")
		default:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "error: %v\n", err)
		}
		return
	}
	// 201 Created + плэйнтекст с полями (как читает агент)
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "uuid: %s\n", res.UUID)
	fmt.Fprintf(w, "key: %s\n", res.Key)
	fmt.Fprintf(w, "hostname: %s\n", res.Name)
	if res.IsNew {
		fmt.Fprintln(w, "is-new: 1")
	} else {
		fmt.Fprintln(w, "is-new: 0")
	}
}

// GET /controller/checksum/{uuid}/?key=...
func (h *Handler) Checksum(w http.ResponseWriter, r *http.Request) {
	setOWHeader(w)
	uuid := mux.Vars(r)["uuid"]
	key := r.URL.Query().Get("key")
	sum, err := h.ds.GetChecksum(r.Context(), uuid, key)
	if err != nil {
		status := http.StatusInternalServerError
		if err == repo.ErrUnauthorized || err == repo.ErrNotFound {
			status = http.StatusNotFound // агент по 404 делает ретраи/выходит
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	// ВАЖНО: тело должно заканчиваться строкой с checksum (tail -n 1)
	io.WriteString(w, sum+"\n")
}

// GET /controller/download-config/{uuid}/?key=...
func (h *Handler) DownloadConfig(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	key := r.URL.Query().Get("key")
	data, sum, err := h.ds.GetConfig(r.Context(), uuid, key)
	if err != nil {
		status := http.StatusInternalServerError
		if err == repo.ErrUnauthorized || err == repo.ErrNotFound {
			status = http.StatusNotFound
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	setOWHeader(w)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.tar.gz"`, uuid, sum[:8]))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// POST /controller/report-status/{uuid}/  body: key=...&status=running|error
func (h *Handler) ReportStatus(w http.ResponseWriter, r *http.Request) {
	setOWHeader(w)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	uuid := mux.Vars(r)["uuid"]
	key := r.FormValue("key")
	status := r.FormValue("status")
	if status == "" {
		status = "running"
	}
	if err := h.ds.ReportStatus(r.Context(), uuid, key, status); err != nil {
		code := http.StatusInternalServerError
		if err == repo.ErrUnauthorized {
			code = http.StatusUnauthorized
		}
		http.Error(w, http.StatusText(code), code)
		return
	}
	// 200 OK без тела — агент проверяет статус-строку и X-Openwisp-Controller
	w.WriteHeader(http.StatusOK)
}

// Вспомогательный генератор минимального tar.gz (hostname -> /etc/config/system)
func MinimalOpenWrtTar(hostname string) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	add := func(name string, data []byte, mode int64) error {
		hdr := &tar.Header{
			Name:    name,
			Mode:    mode,
			Size:    int64(len(data)),
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}

	// Простейший /etc/config/system с hostname
	uci := []byte(fmt.Sprintf("config system\n\toption hostname '%s'\n", hostname))
	if err := add("etc/config/system", uci, 0644); err != nil {
		return nil, err
	}

	// Можно добавить /etc/openwisp/VERSION как метку
	_ = add("etc/openwisp/VERSION", []byte("24.11-go\n"), 0644)

	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes(), nil
}
