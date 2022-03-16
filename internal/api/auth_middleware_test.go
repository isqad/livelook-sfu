package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	sqlxDb := sqlx.NewDb(db, "sqlmock")
	defer sqlxDb.Close()

	repo := core.NewUserRepository(sqlxDb)

	t.Run("default middleware with given AuthFailFunc", func(t *testing.T) {

		r := chi.NewRouter()

		firebaseAuth := NewFirebaseAuth(repo)
		firebaseAuth.AuthFailFunc = func(w http.ResponseWriter, r *http.Request, err error) {
			w.WriteHeader(http.StatusBadRequest)
		}

		r.Use(firebaseAuth.Middleware())

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("Hello, world!"))
		})

		ts := httptest.NewServer(r)
		defer ts.Close()

		req, err := http.NewRequest("GET", ts.URL, nil)
		assert.Nil(t, err)

		resp, err := http.DefaultClient.Do(req)
		assert.Nil(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("default middleware without AuthFailFunc", func(t *testing.T) {

		r := chi.NewRouter()

		firebaseAuth := NewFirebaseAuth(repo)

		r.Use(firebaseAuth.Middleware())

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("Hello, world!"))
		})

		ts := httptest.NewServer(r)
		defer ts.Close()

		req, err := http.NewRequest("GET", ts.URL, nil)
		assert.Nil(t, err)

		resp, err := http.DefaultClient.Do(req)
		assert.Nil(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("stub handler", func(t *testing.T) {
		r := chi.NewRouter()

		firebaseAuth := NewFirebaseAuth(repo)
		firebaseAuth.StubHandler = func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			})
		}

		r.Use(firebaseAuth.Middleware())

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("Hello, world!"))
		})

		ts := httptest.NewServer(r)
		defer ts.Close()

		req, err := http.NewRequest("GET", ts.URL, nil)
		assert.Nil(t, err)

		resp, err := http.DefaultClient.Do(req)
		assert.Nil(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusTeapot, resp.StatusCode)
	})
}
