package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func testServer(t *testing.T) (*Server, *Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "events.json"))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := newServer(store, "templates")
	if err != nil {
		t.Fatal(err)
	}
	return srv, store
}

func postForm(t *testing.T, srv *Server, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	return rec
}

func TestCreateAndList(t *testing.T) {
	srv, store := testServer(t)
	rec := postForm(t, srv, "/create", url.Values{
		"title": {"Book Club"}, "date": {"2026-12-20"},
		"location": {"Addis Ababa"}, "description": {"Monthly books"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: got %d, want 303", rec.Code)
	}
	events := store.List("")
	if len(events) != 1 || events[0].Title != "Book Club" || events[0].Location != "Addis Ababa" {
		t.Fatalf("list: got %+v", events)
	}
	// Saved to the file, so a fresh store sees it too.
	again, err := NewStore(filepath.Join(filepath.Dir(store.path), "events.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(again.List("")) != 1 {
		t.Fatalf("reload: got %d events, want 1", len(again.List("")))
	}
	// The list page shows the event.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Book Club") {
		t.Fatalf("list page: code %d, body %q", rec.Code, rec.Body.String())
	}
}

func TestCreateValidation(t *testing.T) {
	srv, _ := testServer(t)
	// No title.
	rec := postForm(t, srv, "/create", url.Values{"title": {""}, "date": {"2026-12-20"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing title: got %d, want 400", rec.Code)
	}
	// Bad date.
	rec = postForm(t, srv, "/create", url.Values{"title": {"X"}, "date": {"20-12-2026"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad date: got %d, want 400", rec.Code)
	}
	// Not a real date.
	rec = postForm(t, srv, "/create", url.Values{"title": {"X"}, "date": {"2026-02-30"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("impossible date: got %d, want 400", rec.Code)
	}
}

func TestViewAnd404(t *testing.T) {
	srv, store := testServer(t)
	e, err := store.Create(Event{Title: "Show", Date: "2026-12-01"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/event?id=1", nil)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Show") {
		t.Fatalf("view: code %d, body %q", rec.Code, rec.Body.String())
	}
	_ = e
	req = httptest.NewRequest(http.MethodGet, "/event?id=999", nil)
	rec = httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing event: got %d, want 404", rec.Code)
	}
}

func TestUpdateAndDelete(t *testing.T) {
	srv, store := testServer(t)
	e, _ := store.Create(Event{Title: "Old", Date: "2026-12-01"})
	rec := postForm(t, srv, "/update", url.Values{
		"id": {"1"}, "title": {"New"}, "date": {"2026-12-02"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update: got %d, want 303", rec.Code)
	}
	got, _ := store.Get(e.ID)
	if got.Title != "New" || got.Date != "2026-12-02" {
		t.Fatalf("after update: %+v", got)
	}
	req := httptest.NewRequest(http.MethodGet, "/delete?id=1", nil)
	rec = httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: got %d, want 303", rec.Code)
	}
	if _, ok := store.Get(1); ok {
		t.Fatal("event still there after delete")
	}
	req = httptest.NewRequest(http.MethodGet, "/delete?id=999", nil)
	rec = httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing: got %d, want 404", rec.Code)
	}
}

func TestSearch(t *testing.T) {
	srv, store := testServer(t)
	_, _ = store.Create(Event{Title: "Book Club", Date: "2026-12-20", Location: "Addis"})
	_, _ = store.Create(Event{Title: "Football", Date: "2026-12-21", Location: "Stadium"})
	req := httptest.NewRequest(http.MethodGet, "/?q=book", nil)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Book Club") || strings.Contains(rec.Body.String(), "Football") {
		t.Fatalf("search: body %q", rec.Body.String())
	}
}

func TestRegisterAttendee(t *testing.T) {
	srv, store := testServer(t)
	_, _ = store.Create(Event{Title: "Show", Date: "2026-12-01"})
	rec := postForm(t, srv, "/register", url.Values{
		"event_id": {"1"}, "name": {"Hanna"}, "email": {"hanna@example.com"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("register: got %d, want 303", rec.Code)
	}
	e, _ := store.Get(1)
	if len(e.Attendees) != 1 || e.Attendees[0].Name != "Hanna" {
		t.Fatalf("attendees: %+v", e.Attendees)
	}
	// Same email twice is refused.
	rec = postForm(t, srv, "/register", url.Values{
		"event_id": {"1"}, "name": {"Hanna"}, "email": {"hanna@example.com"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate email: got %d, want 400", rec.Code)
	}
	// Bad email is refused.
	rec = postForm(t, srv, "/register", url.Values{
		"event_id": {"1"}, "name": {"Sam"}, "email": {"not-an-email"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad email: got %d, want 400", rec.Code)
	}
	// No such event.
	rec = postForm(t, srv, "/register", url.Values{
		"event_id": {"999"}, "name": {"Sam"}, "email": {"sam@example.com"},
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("register missing event: got %d, want 404", rec.Code)
	}
}

func TestHealth(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Fatalf("health: code %d body %q", rec.Code, rec.Body.String())
	}
}
