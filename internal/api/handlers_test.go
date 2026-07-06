package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// These tests exercise the request-validation branches that return before the
// database is touched, so a nil Pool is safe. Full CRUD against a live database
// is covered by the build-tagged integration test (see integration_test.go).

func serve(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := Router(&Handlers{}) // nil Pool: only pre-DB validation paths are hit
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, want, rec.Body.String())
	}
}

func assertErrorContains(t *testing.T, rec *httptest.ResponseRecorder, sub string) {
	t.Helper()
	if !strings.Contains(rec.Body.String(), sub) {
		t.Fatalf("body %q does not contain %q", rec.Body.String(), sub)
	}
}

func TestCreateTenant_InvalidJSON(t *testing.T) {
	assertStatus(t, serve(t, http.MethodPost, "/tenants", "not json"), http.StatusBadRequest)
}

func TestCreateTenant_EmptyName(t *testing.T) {
	rec := serve(t, http.MethodPost, "/tenants", `{"name":"   "}`)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorContains(t, rec, "name is required")
}

func TestCreateNote_InvalidJSON(t *testing.T) {
	assertStatus(t, serve(t, http.MethodPost, "/tenants/abc/notes", "{"), http.StatusBadRequest)
}

func TestCreateNote_MissingFields(t *testing.T) {
	rec := serve(t, http.MethodPost, "/tenants/abc/notes", `{"title":"","body":""}`)
	assertStatus(t, rec, http.StatusBadRequest)
	assertErrorContains(t, rec, "title and body are required")
}

func TestRouter_UnknownPath(t *testing.T) {
	assertStatus(t, serve(t, http.MethodGet, "/does-not-exist", ""), http.StatusNotFound)
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusTeapot, "boom")
	if rec.Code != http.StatusTeapot {
		t.Fatalf("code = %d, want 418", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got["error"] != "boom" {
		t.Fatalf("body = %s, want {\"error\":\"boom\"}", rec.Body.String())
	}
}
