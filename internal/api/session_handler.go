package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/sfu"
	"github.com/pion/webrtc/v3"
)

type SessionRequest struct {
	Title         string                    `json:"title,omitempty"`
	ImageFilename string                    `json:"image_filename,omitempty"`
	State         sfu.SessionState          `json:"state,omitempty"`
	Sdp           webrtc.SessionDescription `json:"sdp,omitempty"`
}

func SessionUpdateHandler(
	sessionStorage sfu.SessionsDBStorer,
	eventsPublisher eventbus.Publisher,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := extractUserID(r)
		if err != nil {
			log.Printf("can't get user ID from request context: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = r.ParseMultipartForm(32 << 20) // maxMemory 32MB
		if err != nil {
			log.Printf("can't parse multipart form: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		session, err := sfu.NewSessionFromReader(userID, strings.NewReader(r.FormValue("session")))
		if err != nil {
			log.Printf("can't parse session: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if session.Sdp != nil {
			err = initPeerConnection(session, eventsPublisher, w)
			if err != nil {
				log.Printf("can't establish peer connection: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		_, err = sessionStorage.Save(session)
		if err != nil {
			log.Printf("can't save session: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
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
