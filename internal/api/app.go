package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/melody"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"
)

type ChatRpc struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  string `json:"params"`
}

// AppOptions is options of the application
type AppOptions struct {
	DB                 *sqlx.DB
	EventsPublisher    eventbus.Publisher
	EventsSubscriber   eventbus.Subscriber
	SessionsRepository core.SessionsDBStorer

	router         *chi.Mux
	websocket      *melody.Melody
	authMiddleware AuthHandler
	userRepository core.UserStorer
	cookieStore    *sessions.CookieStore
	rootURL        string
}

// App is application for API
type App struct {
	AppOptions
}

// NewApp creates a new API application
func NewApp(options AppOptions) *App {
	options.router = chi.NewRouter()
	options.websocket = melody.New()
	options.websocket.Config.MaxMessageSize = 200 * 1024 // 200K

	userRepo := core.NewUserRepository(options.DB)
	cookieStore := sessions.NewCookieStore([]byte(viper.GetString("app.secret_key")))

	options.cookieStore = cookieStore
	options.userRepository = userRepo

	firebaseAuth := NewFirebaseAuth(userRepo)
	firebaseAuth.Addr = viper.GetString("firebase_auth_service.addr")
	firebaseAuth.AuthFailFunc = authFailedFunc
	firebaseAuth.cookieStore = cookieStore

	options.authMiddleware = firebaseAuth.Middleware()
	options.rootURL = fmt.Sprintf("https://%s:%s", viper.GetString("app.hostname"), viper.GetString("app.port"))

	app := &App{
		options,
	}
	return app
}

// Router is function for construct http router
func (app *App) Router() http.Handler {
	app.router.Get("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("app").ParseFiles(
			"web/templates/admin/layout.html",
			"web/templates/admin/login/index.html",
		)
		if err != nil {
			log.Error().Err(err).Str("service", "web").Msg("")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		tmpl.ExecuteTemplate(w, "layout.html", nil)
	})

	app.router.Post("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		email := r.FormValue("email")
		password := r.FormValue("password")

		user, err := app.userRepository.AuthAdminUser(email, password)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			tmpl, err := template.New("app").ParseFiles(
				"web/templates/admin/layout.html",
				"web/templates/admin/login/index.html",
			)
			if err != nil {
				log.Error().Err(err).Str("service", "web").Msg("")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			tmpl.ExecuteTemplate(w, "layout.html", nil)
			return
		}

		session, _ := app.cookieStore.Get(r, core.AdminSessionNameKey)
		session.Values["id"] = string(user.ID)
		if err := session.Save(r, w); err != nil {
			log.Error().Err(err).Str("service", "web").Msg("")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Successfull, redirect to home page
		w.Header().Set("Location", app.rootURL+"/admin")
		w.WriteHeader(http.StatusFound)
	})

	app.router.Delete("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", app.rootURL+"/login")
		w.WriteHeader(http.StatusFound)
	})
	app.router.With(app.authMiddleware).Route("/admin", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			tmpl, err := template.New("app").ParseFiles(
				"web/templates/admin/layout.html",
				"web/templates/admin/root/index.html",
			)
			if err != nil {
				log.Error().Err(err).Str("service", "web").Msg("")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			tmpl.ExecuteTemplate(w, "layout.html", nil)
		})
	})

	// TODO: protect it
	app.router.Get("/api/v1/streams", StreamListHandler(app.DB))

	app.router.With(app.authMiddleware).Route("/api/v1", func(r chi.Router) {
		r.Get("/ws", WebsocketsHandler(app.EventsSubscriber, app.websocket))
		r.Put("/stream", StreamUpdateHandler(app.SessionsRepository, app.DB))

		// r.Get("/streams", StreamListHandler(app.DB))

		r.Post("/users", func(w http.ResponseWriter, r *http.Request) {
			user := core.NewUser()
			err := json.NewDecoder(r.Body).Decode(user)
			if err != nil {
				log.Error().Err(err).Str("service", "web").Msg("")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := user.Save(app.DB); err != nil {
				log.Error().Err(err).Str("service", "web").Msg("")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if err := json.NewEncoder(w).Encode(user); err != nil {
				log.Error().Err(err).Str("service", "web").Msg("")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		})

		// API для добавления аваторки пользователя
		r.Post("/profile/images", func(w http.ResponseWriter, request *http.Request) {
			user, err := userFromRequest(request)
			if err != nil {
				log.Error().Err(err).Str("service", "web").Msg("can't get user ID from request context")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			image := core.NewUserProfileImage(user.ID, viper.GetString("app.upload_root"))
			imageStorer := core.NewUserProfileImageDbStorer(app.DB)
			if err := image.UploadHandle(request, imageStorer); err != nil {
				log.Error().Err(err).Str("service", "web").Msg("can't upload file")
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}

			w.WriteHeader(http.StatusOK)
		})

		// API для получения списка всех диалогов пользователя
		// GET /api/v1/dialogs
		//
		// TODO:
		// - пагинация
		r.Get("/dialogs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotImplemented)
		})
		// API для получения сообщений диалога
		// GET /api/v1/dialogs/:id
		// TODO: пагинация
		r.Get("/dialogs/{id}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotImplemented)
		})
		// API для создания диалога
		// POST /api/v1/dialogs
		r.Post("/dialogs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotImplemented)
		})
		// API для отправки сообщения
		// POST /api/v1/dialogs/{id}/messages
		r.Post("/dialogs/{id}/messages", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotImplemented)
		})

		// API для получения информации о пользователе
		// GET /api/v1/current_user
		r.Get("/current_user", func(w http.ResponseWriter, request *http.Request) {
			user, err := userFromRequest(request)
			if err != nil {
				log.Error().Err(err).Str("service", "web").Msg("can't get user ID from request context")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if err := json.NewEncoder(w).Encode(user); err != nil {
				log.Error().Err(err).Msg("")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		})
	})

	app.websocket.HandleConnect(ConnectHandler(app.EventsPublisher))
	app.websocket.HandleDisconnect(DisconnectHandler(app.EventsPublisher))
	app.websocket.HandleMessage(HandleMessage(app.EventsPublisher))
	app.websocket.HandleError(func(s *melody.Session, err error) {
		log.Error().Err(err).Str("service", "web").Msg("error in websocket session")
	})

	return app.router
}

func authFailedFunc(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusUnauthorized)
}
