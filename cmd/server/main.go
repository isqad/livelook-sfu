package main

import (
	"log"
	"net/http"
	"os"
	"path"
	"text/template"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/isqad/livelook-sfu/internal/admin"
	"github.com/isqad/livelook-sfu/internal/api"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/sfu"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v4/stdlib"
)

func main() {
	flags := []cli.Flag{
		altsrc.NewStringFlag(&cli.StringFlag{Name: "db.host", Required: true}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "db.port", Required: true}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "db.name", Required: true}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "db.user", Required: true}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "db.password", Required: true}),
		&cli.StringFlag{Name: "config"},
	}

	_ = &cli.Command{
		Name:   "sfu",
		Flags:  flags,
		Before: altsrc.InitInputSourceWithContext(flags, altsrc.NewYamlSourceFromFlagFunc("config")),
	}

	dataSrcName := "postgres://postgres:qwerty@localhost:15433/livelook"
	db, err := sqlx.Connect("pgx", dataSrcName)
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})

	broadcastsRepo := sfu.NewBroadcastsRepository(db)
	ebus := eventbus.New(rdb)
	sup := sfu.NewBroadcastsSupervisor(broadcastsRepo, ebus)
	app := api.NewApp(
		api.AppOptions{
			DB:                   db,
			BroadcastsRepository: broadcastsRepo,
			BroadcastsSupervisor: sup,
			EventBus:             ebus,
		},
	)

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Mount admin
	r.Mount("/admin", admin.NewApp(db, "https://localhost:3001/admin").Router())
	// Mount API
	r.Mount("/api/v1", app.Router())

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

	r.Get("/broadcasts", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("app").ParseFiles(
			"web/templates/layout.html",
			"web/templates/broadcasts/index.html",
		)
		if err != nil {
			log.Fatal(err)
		}

		tmpl.ExecuteTemplate(w, "layout.html", nil)
	})

	r.Get("/broadcasts/{id}", func(w http.ResponseWriter, r *http.Request) {
		broadcastID := chi.URLParam(r, "id")

		tmpl, err := template.New("app").ParseFiles(
			"web/templates/layout.html",
			"web/templates/broadcasts/show.html",
		)
		if err != nil {
			log.Fatal(err)
		}

		tmpl.ExecuteTemplate(w, "layout.html", struct{ ID string }{broadcastID})
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

	err = server.ListenAndServeTLS("configs/certs/cert.pem", "configs/certs/key.pem")

	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server has been closed immediatelly: %v\n", err)
	}
}
