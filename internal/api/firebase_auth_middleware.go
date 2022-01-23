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

var xAuth = http.CanonicalHeaderKey("X-Auth")

// FirebaseAuthenticator is a middleware that verifies token from Firebase Auth
func FirebaseAuthenticator(firebaseAuthServiceAddr string, authFailFunc AuthFailFunc) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get(xAuth)
			if token == "" {
				authFailFunc(w, r, errors.New("Emtpy token"))
				return
			}

			conn, err := grpc.Dial(firebaseAuthServiceAddr, []grpc.DialOption{
				grpc.WithInsecure(),
				grpc.WithBlock(),
			}...)
			if err != nil {
				authFailFunc(w, r, err)
				return
			}
			defer conn.Close()

			authClient := firebase.NewAuthClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			t, err := authClient.Verify(ctx, &firebase.Token{Token: token})
			if err != nil {
				authFailFunc(w, r, err)
				return
			}

			ctx = context.WithValue(r.Context(), UserIDContextKey, t.UserId)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
