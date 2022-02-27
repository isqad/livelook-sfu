package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/sfu"
	"github.com/jmoiron/sqlx"
	"github.com/pion/webrtc/v3"
)

type SessionRequest struct {
	Title         string                    `json:"title,omitempty"`
	ImageFilename string                    `json:"image_filename,omitempty"`
	State         sfu.SessionState          `json:"state,omitempty"`
	Sdp           webrtc.SessionDescription `json:"sdp,omitempty"`
}

func SessionCreateHandler(
	sessionStorage sfu.SessionsDBStorer,
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

		session, err := sfu.NewSessionFromReader(user.ID, r.Body)
		if err != nil {
			log.Printf("can't parse session: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if session.Sdp == nil {
			log.Println("no sdp in session")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = initPeerConnection(session, eventsPublisher, w)
		if err != nil {
			log.Printf("can't establish peer connection: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		session, err = sessionStorage.Save(session)
		if err != nil {
			log.Printf("can't save session: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(session); err != nil {
			log.Printf("can't encode session: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func initPeerConnection(session *sfu.Session, eventsPublisher eventbus.Publisher, w http.ResponseWriter) error {
	err := session.EstablishPeerConnection()
	if err != nil {
		return err
	}

	answer, err := session.CreateWebrtcAnswer()
	if err != nil {
		return errInitPeerConnection(session, err)
	}

	if err := eventsPublisher.Publish(session.UserID, answer); err != nil {
		return errInitPeerConnection(session, err)
	}

	return nil
}

func errInitPeerConnection(session *sfu.Session, err error) error {
	return fmt.Errorf("%v, close session pc: %v", err, session.Close())
}
