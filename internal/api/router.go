package api

import "net/http"

// Router wires the notes API routes using Go 1.22+ method+pattern routing.
func Router(h *Handlers) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("POST /tenants", h.CreateTenant)
	mux.HandleFunc("POST /tenants/{tenant_id}/notes", h.CreateNote)
	mux.HandleFunc("GET /tenants/{tenant_id}/notes", h.ListNotes)
	return mux
}
