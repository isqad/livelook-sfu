package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/isqad/livelook-sfu/internal/sfu"
	"github.com/jmoiron/sqlx"
)

// AppOptions is options of the application
type AppOptions struct {
	DB                   *sqlx.DB
	router               *chi.Mux
	BroadcastsRepository *sfu.BroadcastsRepository
	BroadcastsSupervisor *sfu.BroadcastsSupervisor
}

// App is application for API
type App struct {
	AppOptions
}

// NewApp creates a new API application
func NewApp(options AppOptions) *App {
	options.router = chi.NewRouter()

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

	app.router.With(FirebaseAuthenticator("127.0.0.1:50053", app.authFailedFunc)).Route("/", func(r chi.Router) {
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

	return app.router
}

func (app *App) authFailedFunc(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusUnauthorized)
}
