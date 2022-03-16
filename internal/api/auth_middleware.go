package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	firebase "github.com/isqad/firebase-auth-service/pkg/service"
	"github.com/isqad/livelook-sfu/internal/core"
	"google.golang.org/grpc"
)

type ctxKey string

const (
	// UserContextKey is used for extract uid from request context
	UserContextKey ctxKey = "current_user"
)

// AuthFailFunc is function that is called when authentication failed
type AuthFailFunc func(w http.ResponseWriter, r *http.Request, err error)

// AuthHandler is optional handler for mocking in tests
type AuthHandler func(next http.Handler) http.Handler

var (
	xAuth             = http.CanonicalHeaderKey("X-Auth")
	ErrEmptyAuthToken = errors.New("empty auth token")
)

type FirebaseAuth struct {
	Addr           string
	AuthFailFunc   AuthFailFunc
	StubHandler    AuthHandler
	userRepository core.UserStorer
	cookieStore    *sessions.CookieStore
}

func NewFirebaseAuth(userRepository core.UserStorer) *FirebaseAuth {
	return &FirebaseAuth{
		userRepository: userRepository,
	}
}

// Middleware is a middleware that verifies token from Firebase Auth
func (m *FirebaseAuth) Middleware() AuthHandler {
	if m.StubHandler != nil {
		return m.StubHandler
	}

	return m.defaultMiddleware()
}

func (m *FirebaseAuth) defaultMiddleware() AuthHandler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			adminSession, _ := m.cookieStore.Get(r, core.AdminSessionNameKey)
			adminID, ok := adminSession.Values["id"]
			if ok {
				u, err := m.userRepository.Find(adminID.(string))
				if err == nil && u.IsAdmin {
					ctx := context.WithValue(r.Context(), UserContextKey, u)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

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

			u, err := m.userRepository.FindByUID(t.GetUserId())
			if err != nil {
				m.authFailed(w, r, err)
				return
			}

			ctx = context.WithValue(r.Context(), UserContextKey, u)
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

// userFromRequest извлекает User из контекста запроса
func userFromRequest(r *http.Request) (*core.User, error) {
	user, ok := r.Context().Value(UserContextKey).(*core.User)
	if !ok {
		return nil, errors.New("can't get user from request context")
	}

	return user, nil
}
