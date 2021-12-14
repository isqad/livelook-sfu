package main

import (
	"log"
	"net/http"
	"os"
	"path"
	"text/template"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/isqad/melody"

	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v4/stdlib"
)

func main() {
	dataSrcName := "postgres://postgres:qwerty@localhost:15433/livelook"
	db, err := sqlx.Connect("pgx", dataSrcName)
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	m := melody.New()
	m.Config.MaxMessageSize = 1024

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("app").ParseFiles(
			"web/templates/layout.html",
			"web/templates/index.html",
		)
		if err != nil {
			log.Fatal(err)
		}

		tmpl.ExecuteTemplate(w, "layout.html", nil)
	})
	r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("app").ParseFiles(
			"web/templates/layout.html",
			"web/templates/admin.html",
		)
		if err != nil {
			log.Fatal(err)
		}

		tmpl.ExecuteTemplate(w, "layout.html", nil)
	})

	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Some client connected")
		m.HandleRequest(w, r)
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		m.Broadcast(msg)
	})

	// Serve static assets
	// serves files from web/static dir
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	staticPrefix := "/static/"
	staticDir := path.Join(cwd, "web", staticPrefix)
	r.Method("GET", staticPrefix+"*", http.StripPrefix(staticPrefix, http.FileServer(http.Dir(staticDir))))
	r.Method("GET", "/favicon.ico", http.FileServer(http.Dir(staticDir)))

	server := &http.Server{
		Addr:              ":3001",
		Handler:           r,
		ReadHeaderTimeout: 1 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	if err := server.ListenAndServeTLS("configs/certs/cert.pem", "configs/certs/key.pem"); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server has been closed immediatelly: %v\n", err)
	}
}
