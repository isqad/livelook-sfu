package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"text/template"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/isqad/livelook-sfu/internal/api"
	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/service"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/jackc/pgx/v4/stdlib"
)

const (
	developmentEnv = "development"
)

func main() {
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = developmentEnv
	}

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if appEnv == developmentEnv {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	viper.SetConfigName("config." + appEnv)
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("can not read config file")
	}

	quit := make(chan os.Signal, 1)
	done := make(chan bool, 1)

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
		log.Fatal().Err(err).Msg("")
	}
	if err = db.Ping(); err != nil {
		log.Fatal().Err(err).Msg("")
	}

	sessionsStorer := core.NewSessionsRepository(db)

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", viper.GetString("redis.host"), viper.GetString("redis.port")),
		DB:   0,
	})
	redisPubSub := eventbus.RedisPubSub(rdb)

	nc, err := nats.Connect(viper.GetString("nats.addr"), nats.NoEcho())
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	apiApp := api.NewApp(
		api.AppOptions{
			DB:                 db,
			EventsPublisher:    redisPubSub,
			EventsSubscriber:   redisPubSub,
			SessionsRepository: sessionsStorer,
		},
	)

	sfuRouter, err := eventbus.NewRouter(redisPubSub)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	sfuConfig := config.NewConfig()
	sessionManager, err := service.NewSessionsManager(sfuConfig, sfuRouter, redisPubSub, sessionsStorer, nc)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	<-sfuRouter.Start()

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(api.LoggerMiddleware(&log.Logger))
	r.Use(middleware.Recoverer)

	// Mount API
	r.Mount("/", apiApp.Router())

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("app").ParseFiles(
			"web/templates/layout.html",
			"web/templates/index.html",
		)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}

		tmpl.ExecuteTemplate(w, "layout.html", nil)
	})

	r.Get("/broadcasts", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("app").ParseFiles(
			"web/templates/layout.html",
			"web/templates/broadcasts/index.html",
		)
		if err != nil {
			log.Fatal().Err(err).Msg("")
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
			log.Fatal().Err(err).Msg("")
		}

		tmpl.ExecuteTemplate(w, "layout.html", struct{ ID string }{broadcastID})
	})

	// Serve static assets
	// serves files from web/static dir
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	staticPrefix := "/static/"
	staticDir := path.Join(cwd, "web", staticPrefix)
	r.Method("GET", staticPrefix+"*", http.StripPrefix(staticPrefix, http.FileServer(http.Dir(staticDir))))
	r.Method("GET", "/favicon.ico", http.FileServer(http.Dir(staticDir)))

	r.Handle("/metrics", promhttp.Handler())

	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	server := &http.Server{
		Addr:              ":" + viper.GetString("app.port"),
		Handler:           r,
		ReadHeaderTimeout: 1 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	server.RegisterOnShutdown(func() {
		log.Warn().Msg("received signal to terminate the server")

		log.Info().Msg("send terminate signal to clients")
		sessionManager.Close()

		log.Info().Msg("stop router")
		<-sfuRouter.Stop()

		log.Info().Msg("close redis link")
		if err := rdb.Close(); err != nil {
			log.Error().Err(err).Msg("")
		}

		log.Info().Msg("close db link")
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("")
		}

		log.Info().Msg("all services are stopped")
		close(done)
	})

	// Shutdown the HTTP server
	go func() {
		<-quit
		log.Warn().Msg("the server is going shutting down")

		// Wait 20 seconds for close http connections
		waitIdleConnCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(waitIdleConnCtx); err != nil {
			log.Fatal().Err(err).Msg("can't gracefully shutdown the server")
		}
	}()

	err = server.ListenAndServeTLS("configs/certs/cert.pem", "configs/certs/key.pem")

	if err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server has been closed immediatelly")
	}

	<-done
	log.Info().Msg("server stopped")
}
