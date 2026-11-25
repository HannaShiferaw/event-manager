package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Attendee is one person who said they are coming to an event.
type Attendee struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Event is one event, with the people who signed up for it.
type Event struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Date        string     `json:"date"` // YYYY-MM-DD
	Description string     `json:"description"`
	Location    string     `json:"location"`
	Attendees   []Attendee `json:"attendees"`
}

// Validate checks an event the way a person would: it needs a title,
// and the date has to be a real date written as YYYY-MM-DD.
func (e Event) Validate() error {
	if strings.TrimSpace(e.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(e.Date) == "" {
		return errors.New("date is required")
	}
	if _, err := time.Parse("2006-01-02", e.Date); err != nil {
		return errors.New("date must be YYYY-MM-DD, for example 2026-12-20")
	}
	return nil
}

// Validate checks an attendee: a name, and an email that looks like one.
func (a Attendee) Validate() error {
	if strings.TrimSpace(a.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(a.Email) == "" || !strings.Contains(a.Email, "@") {
		return errors.New("a valid email is required")
	}
	return nil
}

// Store keeps events in a JSON file, so they are still there after a
// restart. There is no database to install.
type Store struct {
	path   string
	mu     sync.Mutex
	events []Event
	nextID int
}

// NewStore loads the events from path, or starts empty if there is no
// file yet.
func NewStore(path string) (*Store, error) {
	s := &Store{path: path, nextID: 1}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, &s.events); err != nil {
		return nil, fmt.Errorf("saved events are not valid: %w", err)
	}
	for _, e := range s.events {
		if e.ID >= s.nextID {
			s.nextID = e.ID + 1
		}
	}
	return s, nil
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.events, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

// List returns all events, soonest first. If q is given, it only
// returns events whose title or location contains it.
func (s *Store) List(q string) []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Event{}
	q = strings.ToLower(strings.TrimSpace(q))
	for _, e := range s.events {
		if q == "" || strings.Contains(strings.ToLower(e.Title), q) ||
			strings.Contains(strings.ToLower(e.Location), q) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

// Get returns one event, or false if there is no event with that id.
func (s *Store) Get(id int) (Event, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.events {
		if e.ID == id {
			return e, true
		}
	}
	return Event{}, false
}

// Create saves a new event and gives it the next id.
func (s *Store) Create(e Event) (Event, error) {
	if err := e.Validate(); err != nil {
		return Event{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e.ID = s.nextID
	s.nextID++
	e.Attendees = []Attendee{}
	s.events = append(s.events, e)
	if err := s.saveLocked(); err != nil {
		return Event{}, err
	}
	return e, nil
}

// Update replaces the title, date, description and location of an
// event, and keeps its attendees.
func (s *Store) Update(e Event) (Event, error) {
	if err := e.Validate(); err != nil {
		return Event{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, old := range s.events {
		if old.ID == e.ID {
			e.Attendees = old.Attendees
			s.events[i] = e
			if err := s.saveLocked(); err != nil {
				return Event{}, err
			}
			return e, nil
		}
	}
	return Event{}, errors.New("event not found")
}

// Delete removes an event and its attendees.
func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.events {
		if e.ID == id {
			s.events = append(s.events[:i], s.events[i+1:]...)
			_ = s.saveLocked()
			return true
		}
	}
	return false
}

// AddAttendee signs one person up for an event. The same email cannot
// sign up twice for the same event.
func (s *Store) AddAttendee(id int, a Attendee) error {
	if err := a.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.events {
		if e.ID == id {
			for _, old := range e.Attendees {
				if strings.EqualFold(old.Email, a.Email) {
					return errors.New("that email is already signed up for this event")
				}
			}
			s.events[i].Attendees = append(s.events[i].Attendees, a)
			return s.saveLocked()
		}
	}
	return errors.New("event not found")
}
