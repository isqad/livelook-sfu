package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/sfu"
	"github.com/isqad/melody"
	"github.com/jmoiron/sqlx"
)

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
		FirebaseAuthenticator("127.0.0.1:50053", app.authFailedFunc),
	).Route("/", func(r chi.Router) {
		r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value(UserIDContextKey).(string)
			if !ok {
				log.Fatal("can't get user ID from request context")
			}

			userSub, err := app.EventBus.SubscribeUser(userID) // TODO
			if err != nil {
				log.Fatal(err)
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
	})

	app.websocket.HandleConnect(func(s *melody.Session) {
		subscription, err := getUserSub(s)
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			ch := subscription.Channel()
			for msg := range ch {
				s.Write([]byte(msg.Payload))
			}
		}()
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
