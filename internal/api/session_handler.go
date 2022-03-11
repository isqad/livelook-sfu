package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/jmoiron/sqlx"
)

func SessionCreateHandler(
	eventsPublisher eventbus.Publisher,
	db *sqlx.DB, // replace to UserRepository
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := userFromRequest(db, r)
		if err != nil {
			log.Printf("can't get user ID from request context: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		sessionReq := &core.Session{}
		if err := json.NewDecoder(r.Body).Decode(sessionReq); err != nil {
			log.Printf("can't parse session: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if sessionReq.Sdp == nil {
			log.Println("no sdp in session")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		sessionReq.UserID = user.ID

		msg := eventbus.ServerMessage{
			UserID: user.ID,
			Rpc:    eventbus.NewCreateSessionRpc(sessionReq),
		}

		if err := eventsPublisher.PublishServer(msg); err != nil {
			log.Printf("publish server error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}
