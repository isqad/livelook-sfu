package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/sfu"
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
	DB                   *sqlx.DB
	BroadcastsRepository *sfu.BroadcastsRepository
	BroadcastsSupervisor *sfu.BroadcastsSupervisor
	EventBus             *eventbus.Eventbus

	router    *chi.Mux
	websocket *melody.Melody
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

	app := &App{
		options,
	}
	return app
}

// Router is function for construct http router
func (app *App) Router() http.Handler {
	app.router.Get("/broadcasts", func(w http.ResponseWriter, r *http.Request) {
		var (
			page    int
			perPage int
			err     error
		)

		if pageParam := r.URL.Query().Get("p"); pageParam != "" {
			page, err = strconv.Atoi(pageParam)
			if err != nil {
				log.Fatal(err)
			}
		}
		if perPageParam := r.URL.Query().Get("limit"); perPageParam != "" {
			page, err = strconv.Atoi(perPageParam)
			if err != nil {
				log.Fatal(err)
			}
		}
		broadcasts, err := app.BroadcastsRepository.GetAll(page, perPage)
		if err != nil {
			log.Fatal(err)
		}

		resp, err := json.Marshal(broadcasts)
		if err != nil {
			log.Fatal(err)
		}

		if _, err := w.Write(resp); err != nil {
			log.Fatal(err)
		}
	})

	app.router.With(
		FirebaseAuthenticator(viper.GetString("firebase_auth_service.addr"), app.authFailedFunc),
	).Route("/", func(r chi.Router) {
		r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
			userID, err := extractUserID(r)
			if err != nil {
				log.Println("can't get user ID from request context")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			userSub, err := app.EventBus.SubscribeUser(userID) // TODO
			if err != nil {
				log.Printf("can't subscribe the user: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			sessKeys := make(map[string]interface{})
			sessKeys["sub"] = userSub

			app.websocket.HandleRequestWithKeys(w, r, sessKeys)
		})

		r.Post("/users", func(w http.ResponseWriter, r *http.Request) {
			user := sfu.NewUser()
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

		r.Post("/broadcasts", func(w http.ResponseWriter, r *http.Request) {
			req := &sfu.BroadcastRequest{}

			err := json.NewDecoder(r.Body).Decode(req)
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if err := app.BroadcastsSupervisor.CreateBroadcast(req); err != nil {
				log.Fatal(err)
			}

			w.WriteHeader(http.StatusOK)
		})

		r.Post("/broadcasts/{id}/viewers", func(w http.ResponseWriter, r *http.Request) {
			broadcastID := chi.URLParam(r, "id")
			req := &sfu.ViewerRequest{}

			err := json.NewDecoder(r.Body).Decode(req)
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if err := app.BroadcastsSupervisor.AddViewer(broadcastID, req); err != nil {
				log.Fatal(err)
			}

			w.WriteHeader(http.StatusOK)
		})

		// API для добавления аваторки пользователя
		r.Post("/profile/images", func(w http.ResponseWriter, request *http.Request) {
			userID, err := extractUserID(request)
			if err != nil {
				log.Println("can't get user ID from request context")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			image := sfu.NewUserProfileImage(userID, viper.GetString("app.upload_root"))
			imageStorer := sfu.NewUserProfileImageDbStorer(app.DB)
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
	})

	app.websocket.HandleConnect(func(s *melody.Session) {
		subscription, err := getUserSub(s)
		if err != nil {
			log.Fatal(err)
		}

		subReady := make(chan struct{})

		go func() {
			ch := subscription.Channel()
			close(subReady)
			for msg := range ch {
				s.Write([]byte(msg.Payload))
			}
		}()

		<-subReady
		msg, err := json.Marshal(ChatRpc{
			JSONRPC: "2.0",
			Method:  "chat",
			Params:  "Hello, world!",
		})
		if err != nil {
			log.Fatal(err)
		}

		err = app.EventBus.Publish("messages:"+subscription.UserID, msg)
		if err != nil {
			log.Fatal(err)
		}
	})
	app.websocket.HandleDisconnect(func(s *melody.Session) {
		subscription, err := getUserSub(s)
		if err != nil {
			log.Fatal(err)
		}
		err = subscription.Close()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("User disconnected")
		// TODO: stop user's broadcast
	})

	return app.router
}

func (app *App) authFailedFunc(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusUnauthorized)
}

func getUserSub(s *melody.Session) (*eventbus.UserSubscription, error) {
	userSub, ok := s.Keys["sub"]
	if !ok {
		return nil, fmt.Errorf("No sub for given session: %+v", s)
	}
	subscription, ok := userSub.(*eventbus.UserSubscription)
	if !ok {
		return nil, fmt.Errorf("Cann't convert userSub: %+v", userSub)
	}
	return subscription, nil
}

// extractUserID извлекает userID из контекста запроса
func extractUserID(r *http.Request) (string, error) {
	userID, ok := r.Context().Value(UserIDContextKey).(string)
	if !ok {
		return "", errors.New("can't get user ID from request context")
	}

	return userID, nil
}
