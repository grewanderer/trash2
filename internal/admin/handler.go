package admin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"gorm.io/gorm"

	"wisp/internal/models"
)

type Handler struct {
	d Dependencies
	t pageTemplates // ← вместо *template.Template теперь наборы по страницам
}

func (h *Handler) redirect(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, path, http.StatusFound)
	}
}

func (h *Handler) render(w http.ResponseWriter, page string, data any) {
	t, ok := h.t[page]
	if !ok {
		http.Error(w, "template not found: "+page, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ---------- Pages ----------

func (h *Handler) DevicesList(w http.ResponseWriter, r *http.Request) {
	var rows []models.Device
	q := h.d.DB.Order("updated_at desc").Limit(200)
	if s := strings.TrimSpace(r.URL.Query().Get("q")); s != "" {
		like := "%" + s + "%"
		q = q.Where("uuid ILIKE ? OR name ILIKE ? OR mac ILIKE ?", like, like, like)
	}
	_ = q.Find(&rows).Error
	h.render(w, "devices_list.tmpl", map[string]any{
		"Title": "Devices",
		"Rows":  rows,
		"Query": r.URL.Query().Get("q"),
	})
}

func (h *Handler) DeviceDetail(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	var dev models.Device
	if err := h.d.DB.Where("uuid=?", uuid).First(&dev).Error; err != nil {
		http.NotFound(w, r)
		return
	}
	// list secrets (ids only)
	type SecretRow struct {
		KeyID     string
		Revoked   bool
		CreatedAt time.Time
	}
	var secs []SecretRow
	_ = h.d.DB.Table("device_secrets").
		Select("key_id, revoked_at IS NOT NULL as revoked, created_at").
		Where("device_id=?", dev.ID).Order("created_at desc").Scan(&secs).Error

	// templates list
	var tpls []models.ConfigTemplate
	_ = h.d.DB.Order("priority asc").Find(&tpls).Error

	h.render(w, "device_detail.tmpl", map[string]any{
		"Title":     "Device " + dev.UUID,
		"Dev":       dev,
		"Secrets":   secs,
		"Templates": tpls,
	})
}

func (h *Handler) DeviceConfigView(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	var dev models.Device
	if err := h.d.DB.Where("uuid=?", uuid).First(&dev).Error; err != nil {
		http.NotFound(w, r)
		return
	}
	// list entries from tar.gz
	files := []struct {
		Name string
		Size int64
	}{}
	if len(dev.ConfigArchive) > 0 {
		tr := tar.NewReader(mustGzipReader(bytes.NewReader(dev.ConfigArchive)))
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			if hdr.FileInfo().IsDir() {
				continue
			}
			files = append(files, struct {
				Name string
				Size int64
			}{hdr.Name, hdr.Size})
		}
	}
	h.render(w, "device_config.tmpl", map[string]any{
		"Title":    "Config tarball",
		"Dev":      dev,
		"Files":    files,
		"Checksum": dev.ConfigChecksum,
		"Version":  dev.ConfigVersion,
	})
}

func mustGzipReader(r io.Reader) *gzip.Reader {
	gr, _ := gzip.NewReader(r)
	return gr
}

func (h *Handler) TemplatesList(w http.ResponseWriter, r *http.Request) {
	var tpls []models.ConfigTemplate
	_ = h.d.DB.Order("priority asc, id asc").Find(&tpls).Error
	h.render(w, "templates_list.tmpl", map[string]any{
		"Title": "Templates", "Rows": tpls,
	})
}

func (h *Handler) TemplateNew(w http.ResponseWriter, r *http.Request) {
	h.render(w, "template_edit.tmpl", map[string]any{
		"Title": "Create Template", "IsNew": true,
	})
}

func (h *Handler) TemplateEdit(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	var t models.ConfigTemplate
	if err := h.d.DB.First(&t, id).Error; err != nil {
		http.NotFound(w, r)
		return
	}
	h.render(w, "template_edit.tmpl", map[string]any{
		"Title": "Edit Template", "Tpl": t, "IsNew": false,
	})
}

func (h *Handler) PKIPage(w http.ResponseWriter, r *http.Request) {
	// show existing CA if any
	var ca models.CA
	err := h.d.DB.Order("id asc").First(&ca).Error
	if err == gorm.ErrRecordNotFound {
		h.render(w, "pki.tmpl", map[string]any{"Title": "PKI", "CA": nil})
		return
	}
	h.render(w, "pki.tmpl", map[string]any{"Title": "PKI", "CA": ca})
}

func (h *Handler) VPNPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "vpn.tmpl", map[string]any{
		"Title": "Mgmt VPN",
		"Cfg":   h.d.CFG.OpenWISP.Controller.MgmtVPN,
	})
}

// ---------- API ----------

func (h *Handler) APIReconcile(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	sum, upd, err := h.d.REC.Reconcile(r.Context(), uuid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"checksum": sum, "updated": upd})
}

func (h *Handler) APISecretIssue(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	var dev models.Device
	if err := h.d.DB.Where("uuid=?", uuid).First(&dev).Error; err != nil {
		http.NotFound(w, r)
		return
	}
	keyID, secret, err := h.d.SECRETS.Issue(r.Context(), dev.ID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// показываем секрет один раз
	writeJSON(w, map[string]any{"key_id": keyID, "secret": fmt.Sprintf("%x", secret)})
}

func (h *Handler) APISecretRevokeAll(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	var dev models.Device
	if err := h.d.DB.Where("uuid=?", uuid).First(&dev).Error; err != nil {
		http.NotFound(w, r)
		return
	}
	if err := h.d.SECRETS.Store.RevokeAll(r.Context(), dev.ID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (h *Handler) APITemplateCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", 400)
		return
	}
	prio, _ := strconv.Atoi(r.FormValue("priority"))
	t := models.ConfigTemplate{
		Name:     r.FormValue("name"),
		Priority: prio,
		NetJSON:  []byte(strings.TrimSpace(r.FormValue("netjson"))),
	}
	if !json.Valid(t.NetJSON) {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if err := h.d.DB.Create(&t).Error; err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/templates", http.StatusFound)
}

func (h *Handler) APITemplateUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", 400)
		return
	}
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	var t models.ConfigTemplate
	if err := h.d.DB.First(&t, id).Error; err != nil {
		http.NotFound(w, r)
		return
	}
	prio, _ := strconv.Atoi(r.FormValue("priority"))
	t.Name = r.FormValue("name")
	t.Priority = prio
	nj := strings.TrimSpace(r.FormValue("netjson"))
	if !json.Valid([]byte(nj)) {
		http.Error(w, "invalid JSON", 400)
		return
	}
	t.NetJSON = []byte(nj)
	if err := h.d.DB.Save(&t).Error; err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/templates", http.StatusFound)
}

func (h *Handler) APITemplateDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	_ = h.d.DB.Delete(&models.ConfigTemplate{}, id).Error
	http.Redirect(w, r, "/admin/templates", http.StatusFound)
}

// ---------- utils ----------

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
