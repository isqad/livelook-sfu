package ws

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/isqad/melody"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/isqad/livelook-sfu/internal/core"
)

// AppOptions is options of the application
type WsAppOptions struct {
	Env     core.Environment
	Address string

	websocket *melody.Melody
}

// App is application for Websocket server
type WsApp struct {
	WsAppOptions
}

func New(options WsAppOptions) *WsApp {
	options.websocket = melody.New()
	options.websocket.Config.MaxMessageSize = 200 * 1024 // 200K

	app := &WsApp{
		options,
	}
	return app
}

func (app *WsApp) Start() error {
	quit := make(chan os.Signal, 1)
	done := make(chan struct{}, 1)

	app.initLogger()
	router := app.initRouter()

	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	server := &http.Server{
		Addr:              app.Address,
		Handler:           router,
		ReadHeaderTimeout: 1 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	server.RegisterOnShutdown(func() {
		log.Warn().Msg("received signal to terminate the server")
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

	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server has been closed immediatelly")
	}

	<-done
	log.Info().Msg("server stopped")

	return nil
}

func (app *WsApp) initLogger() {
	cw := zerolog.NewConsoleWriter()
	log.Logger = log.Output(cw)

	level := zerolog.InfoLevel

	if app.Env.IsDevelopment() {
		level = zerolog.DebugLevel
	}

	zerolog.SetGlobalLevel(level)
}

// initRouter is function for construct http router
func (app *WsApp) initRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	app.websocket.HandleConnect(ConnectHandler())
	app.websocket.HandleDisconnect(DisconnectHandler())
	app.websocket.HandleMessage(HandleMessage())
	app.websocket.HandleError(func(s *melody.Session, err error) {
		log.Error().Err(err).Str("service", "ws").Msg("error in websocket session")
	})

	r.Get("/ws", WsHandler(app.websocket))

	return r
}
