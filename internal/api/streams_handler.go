package api

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/jmoiron/sqlx"
)

func StreamCreateHandler(
	sessionRepository core.SessionsDBStorer,
	db *sqlx.DB,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := userFromRequest(r)
		if err != nil {
			log.Printf("can't get user ID from request context: %v", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		session, err := sessionRepository.FindByUserID(user.ID)
		if err != nil {
			log.Error().Err(err).Str("service", "web").Msg("")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if session == nil {
			log.Error().Err(err).Str("service", "web").Str("userID", string(user.ID)).Msg("couldn't find session")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		image := core.NewStreamImage(session, viper.GetString("app.upload_root"))
		imageStorer := core.NewStreamImageDbStore(db)
		if err := image.UploadHandle(r, imageStorer); err != nil {
			log.Error().Err(err).Str("service", "web").Msg("can't upload file")
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func StreamDeleteHandler(
	eventsPublisher eventbus.Publisher,
	db *sqlx.DB,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := userFromRequest(r)
		if err != nil {
			log.Error().Err(err).Str("service", "web").Msg("can't get user ID from request context")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		session := &core.Session{}
		if err := json.NewDecoder(r.Body).Decode(session); err != nil {
			log.Error().Err(err).Str("service", "web").Msg("can't parse session")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		session.UserID = user.ID

		streamRepo := core.NewStreamsRepository(db)
		_, err = streamRepo.Stop(session)
		if err != nil {
			log.Error().Err(err).Str("service", "web").Msg("can't stop stream")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		rpc := eventbus.NewStopStreamRpc()
		if err := eventsPublisher.PublishServer(eventbus.ServerMessage{UserID: user.ID, Rpc: rpc}); err != nil {
			log.Error().Err(err).Str("service", "web").Msg("publish server rpc error")
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func StreamListHandler(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		streamRepo := core.NewStreamsRepository(db)

		sessions, err := streamRepo.GetAll(1, 50)
		if err != nil {
			log.Error().Err(err).Str("service", "web").Msg("can't get user ID from request context")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := json.NewEncoder(w).Encode(sessions); err != nil {
			log.Error().Err(err).Str("service", "web").Msg("publish server rpc error")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
