package main

import (
	"html/template"
	"net/http"
)

func main() {
	tmpl := template.Must(template.ParseGlob("templates/*.html"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl.ExecuteTemplate(w, "index.html", nil)
	})

	http.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		tmpl.ExecuteTemplate(w, "new_event.html", nil)
	})

	http.ListenAndServe(":8080", nil)
}