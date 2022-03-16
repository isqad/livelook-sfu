package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/jmoiron/sqlx"
)

func StreamCreateHandler(
	eventsPublisher eventbus.Publisher,
	db *sqlx.DB,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := userFromRequest(r)
		if err != nil {
			log.Printf("can't get user ID from request context: %v", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		session := &core.Session{}
		if err := json.NewDecoder(r.Body).Decode(session); err != nil {
			log.Printf("can't parse session: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		session.UserID = user.ID

		streamRepo := core.NewStreamsRepository(db)
		_, err = streamRepo.Start(session)
		if err != nil {
			log.Printf("can't start stream: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		rpc := eventbus.NewStartStreamRpc()
		if err := eventsPublisher.PublishServer(eventbus.ServerMessage{UserID: user.ID, Rpc: rpc}); err != nil {
			log.Printf("publish server rpc error: %v", err)
		}
	}
}

func StreamDeleteHandler(
	eventsPublisher eventbus.Publisher,
	db *sqlx.DB,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := userFromRequest(r)
		if err != nil {
			log.Printf("can't get user ID from request context: %v", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		session := &core.Session{}
		if err := json.NewDecoder(r.Body).Decode(session); err != nil {
			log.Printf("can't parse session: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		session.UserID = user.ID

		streamRepo := core.NewStreamsRepository(db)
		_, err = streamRepo.Stop(session)
		if err != nil {
			log.Printf("can't stop stream: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		rpc := eventbus.NewStopStreamRpc()
		if err := eventsPublisher.PublishServer(eventbus.ServerMessage{UserID: user.ID, Rpc: rpc}); err != nil {
			log.Printf("publish server rpc error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func StreamListHandler(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		streamRepo := core.NewStreamsRepository(db)

		sessions, err := streamRepo.GetAll(1, 50)
		if err != nil {
			log.Printf("can't get user ID from request context: %v", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := json.NewEncoder(w).Encode(sessions); err != nil {
			log.Printf("publish server rpc error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
