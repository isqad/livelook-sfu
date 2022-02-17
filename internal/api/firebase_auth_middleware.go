package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	firebase "github.com/isqad/firebase-auth-service/pkg/service"
	"google.golang.org/grpc"
)

type ctxKey string

const (
	// UserIDContextKey is used for extract uid from request context
	UserIDContextKey ctxKey = "userID"
)

// AuthFailFunc is function that is called when authentication failed
type AuthFailFunc func(w http.ResponseWriter, r *http.Request, err error)

// OptFirebaseAuthHandler is optional handler for mocking in tests
type FirebaseAuthHandler func(next http.Handler) http.Handler

var (
	xAuth             = http.CanonicalHeaderKey("X-Auth")
	ErrEmptyAuthToken = errors.New("empty auth token")
)

type FirebaseAuth struct {
	Addr         string
	AuthFailFunc AuthFailFunc
	StubHandler  FirebaseAuthHandler
}

func NewFirebaseAuth() *FirebaseAuth {
	return &FirebaseAuth{}
}

// Middleware is a middleware that verifies token from Firebase Auth
func (m *FirebaseAuth) Middleware() FirebaseAuthHandler {
	if m.StubHandler != nil {
		return m.StubHandler
	}

	return m.defaultMiddleware()
}

func (m *FirebaseAuth) defaultMiddleware() FirebaseAuthHandler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get(xAuth)
			if token == "" {
				m.authFailed(w, r, ErrEmptyAuthToken)
				return
			}

			conn, err := grpc.Dial(m.Addr, []grpc.DialOption{
				grpc.WithInsecure(),
				grpc.WithBlock(),
			}...)
			if err != nil {
				m.authFailed(w, r, err)
				return
			}
			defer conn.Close()

			authClient := firebase.NewAuthClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			t, err := authClient.Verify(ctx, &firebase.Token{Token: token})
			if err != nil {
				m.authFailed(w, r, err)
				return
			}

			ctx = context.WithValue(r.Context(), UserIDContextKey, t.GetUserId())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (m *FirebaseAuth) authFailed(w http.ResponseWriter, r *http.Request, err error) {
	if m.AuthFailFunc != nil {
		m.AuthFailFunc(w, r, err)
	} else {
		w.WriteHeader(http.StatusUnauthorized)
	}
}
