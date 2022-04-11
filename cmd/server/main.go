package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"text/template"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/viper"

	"github.com/isqad/livelook-sfu/internal/api"
	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/service"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v4/stdlib"
)

func main() {
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "development"
	}

	viper.SetConfigName("config." + appEnv)
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("can not read config file: %v", err)
	}

	dataSrcName := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		viper.GetString("db.user"),
		viper.GetString("db.password"),
		viper.GetString("db.host"),
		viper.GetString("db.port"),
		viper.GetString("db.name"),
	)
	db, err := sqlx.Connect("pgx", dataSrcName)
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", viper.GetString("redis.host"), viper.GetString("redis.port")),
		DB:   0,
	})
	redisPubSub := eventbus.RedisPubSub(rdb)

	apiApp := api.NewApp(
		api.AppOptions{
			DB:               db,
			EventsPublisher:  redisPubSub,
			EventsSubscriber: redisPubSub,
		},
	)

	sfuRouter, err := eventbus.NewRouter(redisPubSub)
	if err != nil {
		log.Fatal(err)
	}
	sfuConfig := config.NewConfig()
	_, err = service.NewSessionsManager(sfuConfig, sfuRouter)
	if err != nil {
		log.Fatal(err)
	}

	sfuRouter.Start()

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Mount API
	r.Mount("/", apiApp.Router())

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
		Addr:              ":" + viper.GetString("app.port"),
		Handler:           r,
		ReadHeaderTimeout: 1 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	err = server.ListenAndServeTLS("configs/certs/cert.pem", "configs/certs/key.pem")

	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server has been closed immediatelly: %v\n", err)
	}
}
