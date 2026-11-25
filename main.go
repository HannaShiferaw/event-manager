package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Server holds the store and the page templates.
type Server struct {
	store *Store
	tmpl  *template.Template
}

func newServer(store *Store, tmplDir string) (*Server, error) {
	tmpl, err := template.ParseGlob(tmplDir + "/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{store: store, tmpl: tmpl}, nil
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.listEvents)
	mux.HandleFunc("/events", s.listEvents)
	mux.HandleFunc("/new", s.newEventForm)
	mux.HandleFunc("/create", s.createEvent)
	mux.HandleFunc("/event", s.viewEvent)
	mux.HandleFunc("/edit", s.editEventForm)
	mux.HandleFunc("/update", s.updateEvent)
	mux.HandleFunc("/delete", s.deleteEvent)
	mux.HandleFunc("/register", s.registerAttendee)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	})
	return mux
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "could not show the page", http.StatusInternalServerError)
	}
}

// listEvents shows all events, or only the ones matching the search box.
func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/events" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := "index.html"
	if r.URL.Path == "/events" {
		name = "events.html"
	}
	s.render(w, name, struct {
		Events []Event
		Query  string
	}{Events: s.store.List(r.URL.Query().Get("q")), Query: r.URL.Query().Get("q")})
}

func (s *Server) newEventForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.render(w, "new_event.html", nil)
}

// eventFromForm reads an event from a form, with the same field names
// the templates use.
func eventFromForm(r *http.Request) Event {
	return Event{
		Title:       strings.TrimSpace(r.FormValue("title")),
		Date:        strings.TrimSpace(r.FormValue("date")),
		Description: strings.TrimSpace(r.FormValue("description")),
		Location:    strings.TrimSpace(r.FormValue("location")),
	}
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := s.store.Create(eventFromForm(r)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// viewEvent shows one event and its attendees.
func (s *Server) viewEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "event id is missing", http.StatusBadRequest)
		return
	}
	e, ok := s.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, "attendees.html", struct {
		Event     Event
		Attendees []Attendee
	}{Event: e, Attendees: e.Attendees})
}

func (s *Server) editEventForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "event id is missing", http.StatusBadRequest)
		return
	}
	e, ok := s.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, "edit_event.html", e)
}

func (s *Server) updateEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Error(w, "event id is missing", http.StatusBadRequest)
		return
	}
	e := eventFromForm(r)
	e.ID = id
	if _, err := s.store.Update(e); err != nil {
		if err.Error() == "event not found" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/event?id="+strconv.Itoa(id), http.StatusSeeOther)
}

func (s *Server) deleteEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "event id is missing", http.StatusBadRequest)
		return
	}
	if !s.store.Delete(id) {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// registerAttendee signs one person up for an event.
func (s *Server) registerAttendee(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.FormValue("event_id"))
	if err != nil {
		http.Error(w, "event id is missing", http.StatusBadRequest)
		return
	}
	a := Attendee{
		Name:  strings.TrimSpace(r.FormValue("name")),
		Email: strings.TrimSpace(r.FormValue("email")),
	}
	if err := s.store.AddAttendee(id, a); err != nil {
		if err.Error() == "event not found" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/event?id="+strconv.Itoa(id), http.StatusSeeOther)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	file := os.Getenv("EVENTS_FILE")
	if file == "" {
		file = "events.json"
	}
	store, err := NewStore(file)
	if err != nil {
		log.Fatal(err)
	}
	srv, err := newServer(store, "templates")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("event-manager is running on port " + port)
	log.Fatal(http.ListenAndServe(":"+port, srv.routes()))
}
