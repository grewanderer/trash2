package admin

import (
	"github.com/gorilla/mux"
	"gorm.io/gorm"

	"wisp/config"
	"wisp/internal/controller"
	"wisp/internal/pki"
	"wisp/internal/repo"
	"wisp/internal/secrets"
)

type Dependencies struct {
	DB      *gorm.DB
	DS      *repo.DeviceStore
	TS      *repo.TemplateStore
	PKI     *pki.Service
	REC     *controller.Reconciler
	SECRETS *secrets.Service
	CFG     *config.Config
}

func Attach(r *mux.Router, d Dependencies) {
	h := &Handler{d: d, t: parseTemplates()}
	sub := r.PathPrefix("/admin").Subrouter()

	// pages
	sub.HandleFunc("", h.redirect("/admin/devices")).Methods("GET")
	sub.HandleFunc("/", h.redirect("/admin/devices")).Methods("GET")
	sub.HandleFunc("/devices", h.DevicesList).Methods("GET")
	sub.HandleFunc("/devices/{uuid}", h.DeviceDetail).Methods("GET")
	sub.HandleFunc("/devices/{uuid}/config/view", h.DeviceConfigView).Methods("GET")
	sub.HandleFunc("/templates", h.TemplatesList).Methods("GET")
	sub.HandleFunc("/templates/new", h.TemplateNew).Methods("GET")
	sub.HandleFunc("/templates/{id:[0-9]+}/edit", h.TemplateEdit).Methods("GET")
	sub.HandleFunc("/pki", h.PKIPage).Methods("GET")
	sub.HandleFunc("/settings/vpn", h.VPNPage).Methods("GET")

	// api (JSON or redirect back)
	sub.HandleFunc("/api/devices/{uuid}/reconcile", h.APIReconcile).Methods("POST")
	sub.HandleFunc("/api/devices/{uuid}/secrets/issue", h.APISecretIssue).Methods("POST")
	sub.HandleFunc("/api/devices/{uuid}/secrets/revoke_all", h.APISecretRevokeAll).Methods("POST")

	sub.HandleFunc("/api/templates", h.APITemplateCreate).Methods("POST")
	sub.HandleFunc("/api/templates/{id:[0-9]+}", h.APITemplateUpdate).Methods("POST")
	sub.HandleFunc("/api/templates/{id:[0-9]+}/delete", h.APITemplateDelete).Methods("POST")

	// static (very small)
	sub.HandleFunc("/static/style.css", serveCSS).Methods("GET")
	sub.HandleFunc("/static/app.js", serveJS).Methods("GET")
}
