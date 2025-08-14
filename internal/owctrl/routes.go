package owctrl

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// Store — минимальный контракт, который нужен контроллеру
type Store interface {
	Adopt(ctxCtx interface{ Done() <-chan struct{} }, in AdoptRequest) (*DeviceDTO, error)
	GetConfig(ctxCtx interface{ Done() <-chan struct{} }, uuid string) (cfg []byte, version int, checksum string, err error)
	AckConfig(ctxCtx interface{ Done() <-chan struct{} }, uuid string, version int, checksum, status string, appliedAt time.Time) error
}

// RegisterRoutes — in-memory (для случая без БД)
func RegisterRoutes(r *mux.Router, sharedSecret string) {
	s := newInMemoryStore()
	sub := r.PathPrefix("/ow/api/v1").Subrouter()
	sub.Use(sharedSecretAuth(sharedSecret))
	registerHandlers(sub, s)
}

// RegisterRoutesWithStore — с БД через адаптер
func RegisterRoutesWithStore(r *mux.Router, sharedSecret string, store Store) {
	sub := r.PathPrefix("/ow/api/v1").Subrouter()
	sub.Use(sharedSecretAuth(sharedSecret))
	registerHandlers(sub, store)
}

func registerHandlers(r *mux.Router, store Store) {
	h := &Handler{store: store}
	r.HandleFunc("/devices/adopt", h.Adopt).Methods(http.MethodPost)
	r.HandleFunc("/devices/{uuid:[a-fA-F0-9\\-]{36}}/config", h.GetConfig).Methods(http.MethodGet)
	r.HandleFunc("/devices/{uuid:[a-fA-F0-9\\-]{36}}/config/applied", h.AckConfig).Methods(http.MethodPost)
}
