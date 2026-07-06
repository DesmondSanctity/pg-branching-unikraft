//go:build integration

// Full CRUD against a live, migrated database. Run with:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/api
//
// Requires the schema from ./migrations to be applied (tenants + notes tables).
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"pg-branching-unikraft/internal/db"
)

func TestCRUD_Integration(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	pool, err := db.Pool(context.Background())
	if err != nil {
		t.Fatalf("db.Pool: %v", err)
	}
	defer pool.Close()
	r := Router(&Handlers{Pool: pool})

	// Create a tenant.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/tenants", strings.NewReader(`{"name":"integration-test"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create tenant = %d: %s", rec.Code, rec.Body.String())
	}
	var tenant Tenant
	if err := json.Unmarshal(rec.Body.Bytes(), &tenant); err != nil {
		t.Fatalf("decode tenant: %v", err)
	}

	// Create a note under that tenant.
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/tenants/"+tenant.ID+"/notes", strings.NewReader(`{"title":"t","body":"b"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create note = %d: %s", rec.Code, rec.Body.String())
	}

	// List notes and confirm the one we created is present.
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tenants/"+tenant.ID+"/notes", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list notes = %d: %s", rec.Code, rec.Body.String())
	}
	var notes []Note
	if err := json.Unmarshal(rec.Body.Bytes(), &notes); err != nil {
		t.Fatalf("decode notes: %v", err)
	}
	if len(notes) != 1 || notes[0].Title != "t" {
		t.Fatalf("got %d notes %+v, want 1 with title t", len(notes), notes)
	}
}
