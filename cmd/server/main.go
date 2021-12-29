package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"text/template"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/isqad/livelook-sfu/internal/admin"
	"github.com/isqad/livelook-sfu/internal/sfu"
	"github.com/isqad/melody"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v4/stdlib"
)

// UserSub is user subscription
type UserSub struct {
	userID string
	pubsub *redis.PubSub
}

func main() {
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
	sup := sfu.NewBroadcastsSupervisor(db, rdb)

	m := melody.New()
	m.Config.MaxMessageSize = 1024

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Mount admin
	r.Mount("/admin", admin.NewApp(db, "https://localhost:3001/admin").Router())

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

	r.Get("/api/v1/broadcasts", func(w http.ResponseWriter, r *http.Request) {
		var (
			page    int
			perPage int
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
		broadcasts, err := broadcastsRepo.GetAll(page, perPage)
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

	r.Post("/api/v1/broadcasts", func(w http.ResponseWriter, r *http.Request) {
		req := &sfu.BroadcastRequest{}

		err := json.NewDecoder(r.Body).Decode(req)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := sup.CreateBroadcast(req); err != nil {
			log.Fatal(err)
		}

		w.WriteHeader(http.StatusOK)
	})

	r.Post("/api/v1/broadcasts/{id}/viewers", func(w http.ResponseWriter, r *http.Request) {
		broadcastID := chi.URLParam(r, "id")
		req := &sfu.ViewerRequest{}

		err := json.NewDecoder(r.Body).Decode(req)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := sup.AddViewer(broadcastID, req); err != nil {
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

		ctx := context.Background()
		// Subscribe user to messages
		pubsub := rdb.Subscribe(ctx, "messages:"+uuid)
		// Wait until subscription is created
		_, err = pubsub.Receive(ctx)
		if err != nil {
			log.Fatal(err)
		}

		userSub := &UserSub{userID: uuid, pubsub: pubsub}

		sessKeys := make(map[string]interface{})
		sessKeys["sub"] = userSub

		m.HandleRequestWithKeys(w, r, sessKeys)
	})
	m.HandleConnect(func(s *melody.Session) {
		subscription, err := getUserSub(s)
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			ch := subscription.pubsub.Channel()
			for msg := range ch {
				s.Write([]byte(msg.Payload))
			}
		}()
	})
	m.HandleDisconnect(func(s *melody.Session) {
		subscription, err := getUserSub(s)
		if err != nil {
			log.Fatal(err)
		}
		err = subscription.pubsub.Close()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("User disconnected")
		// TODO: stop user's broadcast
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

func getUserSub(s *melody.Session) (*UserSub, error) {
	userSub, ok := s.Keys["sub"]
	if !ok {
		return nil, fmt.Errorf("No sub for given session: %+v", s)
	}
	subscription, ok := userSub.(*UserSub)
	if !ok {
		return nil, fmt.Errorf("Cann't convert userSub: %+v", userSub)
	}
	return subscription, nil
}
