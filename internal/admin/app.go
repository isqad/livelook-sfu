package admin

import (
	"context"
	"html/template"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
)

type ctxKey string

const (
	sessionNameKey        = "_livelook_session"
	userIDCtxKey   ctxKey = "userID"
)

// App is admin application struct
type App struct {
	rootURL      string
	router       *chi.Mux
	db           *sqlx.DB
	sessionStore *sessions.CookieStore
}

// NewApp creates new instance of admin application
func NewApp(db *sqlx.DB, rootURL string) *App {
	r := chi.NewRouter()
	app := &App{
		rootURL:      rootURL,
		db:           db,
		sessionStore: sessions.NewCookieStore([]byte("some-secret-key")),
	}
	app.router = r

	return app
}

// Router return admin router
func (app *App) Router() http.Handler {
	app.router.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("app").ParseFiles(
			"web/templates/admin/layout.html",
			"web/templates/admin/login/index.html",
		)
		if err != nil {
			log.Fatal(err)
		}

		tmpl.ExecuteTemplate(w, "layout.html", nil)
	})
	app.router.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		email := r.FormValue("email")
		password := r.FormValue("password")

		user, err := AuthAdminUser(app.db, email, password)
		if err != nil {
			log.Fatal(err)
		}
		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			tmpl, err := template.New("app").ParseFiles(
				"web/templates/admin/layout.html",
				"web/templates/admin/login/index.html",
			)
			if err != nil {
				log.Fatal(err)
			}

			tmpl.ExecuteTemplate(w, "layout.html", nil)
			return
		}

		session, _ := app.sessionStore.Get(r, sessionNameKey)
		session.Values["id"] = user.ID
		if err := session.Save(r, w); err != nil {
			log.Fatal(err)
		}

		// Successfull, redirect to home page
		w.Header().Set("Location", app.rootURL)
		w.WriteHeader(http.StatusFound)
	})
	app.router.Delete("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", app.rootURL+"/login")
		w.WriteHeader(http.StatusFound)
	})

	app.router.With(app.authenticateOrLogin).Route("/", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			tmpl, err := template.New("app").ParseFiles(
				"web/templates/admin/layout.html",
				"web/templates/admin/root/index.html",
			)
			if err != nil {
				log.Fatal(err)
			}

			tmpl.ExecuteTemplate(w, "layout.html", nil)
		})
	})

	return app.router
}

func (app *App) authenticateOrLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := app.sessionStore.Get(r, sessionNameKey)
		userID, ok := session.Values["id"]
		failFunc := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", app.rootURL+"/login")
			w.WriteHeader(http.StatusTemporaryRedirect)
		}
		if !ok {
			failFunc(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), userIDCtxKey, userID.(string))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
