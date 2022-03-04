package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
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
	DB               *sqlx.DB
	EventsPublisher  eventbus.Publisher
	EventsSubscriber eventbus.Subscriber

	router         *chi.Mux
	websocket      *melody.Melody
	authMiddleware FirebaseAuthHandler

	sessionsStorage core.SessionsDBStorer
}

// App is application for API
type App struct {
	AppOptions
}

// NewApp creates a new API application
func NewApp(options AppOptions) *App {
	options.router = chi.NewRouter()
	options.websocket = melody.New()
	options.websocket.Config.MaxMessageSize = 1024

	firebaseAuth := NewFirebaseAuth()
	firebaseAuth.Addr = viper.GetString("firebase_auth_service.addr")
	firebaseAuth.AuthFailFunc = authFailedFunc

	options.authMiddleware = firebaseAuth.Middleware()

	options.sessionsStorage = core.NewSessionsRepository(options.DB)

	app := &App{
		options,
	}
	return app
}

// Router is function for construct http router
func (app *App) Router() http.Handler {
	app.router.With(app.authMiddleware).Route("/", func(r chi.Router) {
		r.Get("/ws", WebsocketsHandler(app.EventsSubscriber, app.DB, app.websocket))
		r.Post("/session", SessionCreateHandler(app.EventsPublisher, app.DB))

		r.Post("/users", func(w http.ResponseWriter, r *http.Request) {
			user := core.NewUser()
			err := json.NewDecoder(r.Body).Decode(user)
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := user.Save(app.DB); err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if err := json.NewEncoder(w).Encode(user); err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		})

		// API для добавления аваторки пользователя
		r.Post("/profile/images", func(w http.ResponseWriter, request *http.Request) {
			userID, err := extractUserID(request)
			if err != nil {
				log.Println("can't get user ID from request context")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			image := core.NewUserProfileImage(userID, viper.GetString("app.upload_root"))
			imageStorer := core.NewUserProfileImageDbStorer(app.DB)
			if err := image.UploadHandle(request, imageStorer); err != nil {
				log.Printf("can't upload file: %+v", err)
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

		})
		// API для получения сообщений диалога
		// GET /api/v1/dialogs/:id
		// TODO: пагинация
		r.Get("/dialogs/{id}", func(w http.ResponseWriter, r *http.Request) {

		})
		// API для создания диалога
		// POST /api/v1/dialogs
		r.Post("/dialogs", func(w http.ResponseWriter, r *http.Request) {})
		// API для отправки сообщения
		// POST /api/v1/dialogs/{id}/messages
		r.Post("/dialogs/{id}/messages", func(w http.ResponseWriter, r *http.Request) {})

		// API для получения информации о пользователе
		// GET /api/v1/current_user
		r.Get("/current_user", func(w http.ResponseWriter, request *http.Request) {
			userID, err := extractUserID(request)
			if err != nil {
				log.Printf("can't get user ID from request context: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			user, err := core.FindUserByUID(app.DB, userID)
			if err != nil {
				if err == sql.ErrNoRows {
					log.Println("can't find user")
					w.WriteHeader(http.StatusNotFound)
				} else {
					log.Printf("can't find user: %v", err)
					w.WriteHeader(http.StatusBadRequest)
				}

				return
			}

			if err := json.NewEncoder(w).Encode(user); err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		})
	})

	app.websocket.HandleConnect(ConnectHandler)
	app.websocket.HandleDisconnect(DisconnectHandler(app.EventsPublisher))
	app.websocket.HandleMessage(HandleMessage(app.EventsPublisher))

	return app.router
}

func authFailedFunc(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusUnauthorized)
}

// userFromRequest извлекает User из контекста запроса
func userFromRequest(db *sqlx.DB, r *http.Request) (*core.User, error) {
	uid, err := extractUserID(r)
	if err != nil {
		return nil, err
	}

	user, err := core.FindUserByUID(db, uid)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// extractUserID извлекает userID из контекста запроса
func extractUserID(r *http.Request) (string, error) {
	userID, ok := r.Context().Value(UserIDContextKey).(string)
	if !ok {
		return "", errors.New("can't get user ID from request context")
	}

	return userID, nil
}
