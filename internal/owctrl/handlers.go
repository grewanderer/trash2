package owctrl

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func NewHandler(store Store) *Handler { return &Handler{store: store} }

type Handler struct {
	store Store
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) Adopt(w http.ResponseWriter, r *http.Request) {
	var req AdoptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.UUID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "uuid required"})
		return
	}

	dev, err := h.store.Adopt(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, AdoptResponse{
		DeviceID: dev.ID,
		Next:     "/ow/api/v1/devices/" + dev.UUID + "/config",
	})
}

func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	prev := r.URL.Query().Get("checksum")

	cfg, ver, sum, err := h.store.GetConfig(r.Context(), uuid)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	// ETag-like 304 поведение
	if prev != "" && prev == sum {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, ConfigResponse{
		NetJSON:  json.RawMessage(cfg),
		Version:  ver,
		Checksum: sum,
	})
}

func (h *Handler) AckConfig(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	var req AckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.store.AckConfig(r.Context(), uuid, req.Version, req.Checksum, req.Status, time.Time(req.AppliedAt)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// Вспомогательный хелпер (если нужно) — расчёт checksum на стороне сервера
func checksum(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}
