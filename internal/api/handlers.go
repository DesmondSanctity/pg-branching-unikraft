package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handlers holds the dependencies for the HTTP handlers.
type Handlers struct {
	Pool *pgxpool.Pool
}

type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Note struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Healthz reports 200 when the database connection is alive. Also used to
// confirm scale-to-zero wake-up.
func (h *Handlers) Healthz(w http.ResponseWriter, r *http.Request) {
	if err := h.Pool.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	var t Tenant
	err := h.Pool.QueryRow(r.Context(),
		`INSERT INTO tenants (name) VALUES ($1) RETURNING id::text, name, created_at`,
		in.Name,
	).Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create tenant: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handlers) CreateNote(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	var in struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Body) == "" {
		writeError(w, http.StatusBadRequest, "title and body are required")
		return
	}

	var n Note
	err := h.Pool.QueryRow(r.Context(),
		`INSERT INTO notes (tenant_id, title, body) VALUES ($1, $2, $3)
		 RETURNING id::text, tenant_id::text, title, body, created_at`,
		tenantID, in.Title, in.Body,
	).Scan(&n.ID, &n.TenantID, &n.Title, &n.Body, &n.CreatedAt)
	if err != nil {
		// Foreign-key violation on a nonexistent tenant surfaces here.
		writeError(w, http.StatusBadRequest, "failed to create note: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (h *Handlers) ListNotes(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	rows, err := h.Pool.Query(r.Context(),
		`SELECT id::text, tenant_id::text, title, body, created_at
		 FROM notes WHERE tenant_id = $1 ORDER BY created_at`,
		tenantID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query notes: "+err.Error())
		return
	}
	defer rows.Close()

	notes := make([]Note, 0)
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.TenantID, &n.Title, &n.Body, &n.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan note: "+err.Error())
			return
		}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "row iteration error: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, notes)
}
