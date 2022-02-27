package api

import (
	"log"
	"net/http"

	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/melody"
	"github.com/jmoiron/sqlx"
)

func WebsocketsHandler(
	eventsSubscriber eventbus.Subscriber,
	db *sqlx.DB,
	websocket *melody.Melody, // replace to UserRepository
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := userFromRequest(db, r)
		if err != nil {
			log.Printf("can't get user from request context: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		userSub, err := eventsSubscriber.SubscribeUser(user.ID)
		if err != nil {
			log.Printf("can't subscribe the user: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		sessKeys := make(map[string]interface{})
		sessKeys["sub"] = userSub

		websocket.HandleRequestWithKeys(w, r, sessKeys)
	}
}
