package owagent

import "github.com/gorilla/mux"

func RegisterRoutes(r *mux.Router, h *Handler) {
	sub := r.PathPrefix("/controller").Subrouter()
	sub.StrictSlash(false) // ← критично: никаких 301
	sub.HandleFunc("/register", h.Register).Methods("POST")
	sub.HandleFunc("/register/", h.Register).Methods("POST")

	// checksum — ОБЕ формы, GET (можно и HEAD)
	sub.HandleFunc("/checksum/{uuid}", h.Checksum).Methods("GET", "HEAD")
	sub.HandleFunc("/checksum/{uuid}/", h.Checksum).Methods("GET", "HEAD")

	// download-config — ОБЕ формы, GET/HEAD
	sub.HandleFunc("/download-config/{uuid}", h.DownloadConfig).Methods("GET", "HEAD")
	sub.HandleFunc("/download-config/{uuid}/", h.DownloadConfig).Methods("GET", "HEAD")

	// report status — underscore И дефис, ОБЕ формы пути
	sub.HandleFunc("/report_status/{uuid}", h.ReportStatus).Methods("POST")
	sub.HandleFunc("/report_status/{uuid}/", h.ReportStatus).Methods("POST")
	sub.HandleFunc("/report-status/{uuid}", h.ReportStatus).Methods("POST")
	sub.HandleFunc("/report-status/{uuid}/", h.ReportStatus).Methods("POST")
}
