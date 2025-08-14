package owagent

import "github.com/gorilla/mux"

func RegisterRoutes(r *mux.Router, h *Handler) {
	sub := r.PathPrefix("/controller").Subrouter()
	sub.HandleFunc("/register/", h.Register).Methods("POST")
	sub.HandleFunc("/checksum/{uuid}/", h.Checksum).Methods("GET")
	sub.HandleFunc("/download-config/{uuid}/", h.DownloadConfig).Methods("GET")
	sub.HandleFunc("/report-status/{uuid}/", h.ReportStatus).Methods("POST")
}
