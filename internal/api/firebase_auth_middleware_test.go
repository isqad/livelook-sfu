package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestFirebaseAuthMiddleware(t *testing.T) {
	t.Run("default middleware with given AuthFailFunc", func(t *testing.T) {

		r := chi.NewRouter()

		firebaseAuth := NewFirebaseAuth()
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

		firebaseAuth := NewFirebaseAuth()

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

		firebaseAuth := NewFirebaseAuth()
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
