package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path"
	"text/template"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/isqad/livelook-sfu/internal/sfu"
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

	r.Post("/api/v1/broadcasts", func(w http.ResponseWriter, r *http.Request) {
		req := &sfu.BroadcastRequest{}

		err := json.NewDecoder(r.Body).Decode(req)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("%+v\n", req)

		broadcast, err := sfu.NewBroadcast(
			uuid.NewString(),
			req.UserID,
			req.Title,
			req.Sdp,
		)
		if err != nil {
			log.Fatal(err)
		}
		if err := broadcast.Start(db); err != nil {
			log.Fatal(err)
		}

		w.WriteHeader(http.StatusOK)
	})

	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.URL.Query().Get("uuid")
		if uuid == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// FIXME: create new user for every socket connection
		u := sfu.NewUser(uuid)
		u, err := u.Save(db)
		if err != nil {
			log.Fatal(err)
		}

		sessKeys := make(map[string]interface{})
		sessKeys[uuid] = struct{}{}

		m.HandleRequestWithKeys(w, r, sessKeys)
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
