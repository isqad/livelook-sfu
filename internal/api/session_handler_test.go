package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/isqad/livelook-sfu/internal/sfu"
	"github.com/pion/webrtc/v3"
	"github.com/stretchr/testify/assert"
)

const (
	minimalTestSdp = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
a=msid-semantic: WMS
m=application 47299 DTLS/SCTP 5000
c=IN IP4 192.168.20.129
a=candidate:1966762134 1 udp 2122260223 192.168.20.129 47299 typ host generation 0
a=candidate:1966762134 1 udp 2122262783 2001:db8::1 47199 typ host generation 0
a=candidate:211962667 1 udp 2122194687 10.0.3.1 40864 typ host generation 0
a=candidate:1002017894 1 tcp 1518280447 192.168.20.129 0 typ host tcptype active generation 0
a=candidate:1109506011 1 tcp 1518214911 10.0.3.1 0 typ host tcptype active generation 0
a=ice-ufrag:1/MvHwjAyVf27aLu
a=ice-pwd:3dBU7cFOBl120v33cynDvN1E
a=ice-options:google-ice
a=fingerprint:sha-256 75:74:5A:A6:A4:E5:52:F4:A7:67:4C:01:C7:EE:91:3F:21:3D:A2:E3:53:7B:6F:30:86:F2:30:AA:65:FB:04:24
a=setup:actpass
a=mid:data
a=sctpmap:5000 webrtc-datachannel 1024
`
)

var (
	minimalOfferSdp = webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  minimalTestSdp,
	}
)

type MockSessionStorage struct {
	CurrentSession *sfu.Session
	MockErr        error
}

func (s *MockSessionStorage) Save(session *sfu.Session) (*sfu.Session, error) {
	session.ID = 42
	s.CurrentSession = session

	return session, s.MockErr
}

type MockEventBus struct {
	PublishedMessage []byte
	MockErr          error
}

func (e *MockEventBus) Publish(userID string, message interface{}) error {
	e.PublishedMessage = message.([]byte)

	return e.MockErr
}

func TestSessionCreateHandler(t *testing.T) {
	userID := "foo-bar-42"
	firebaseAuth := NewFirebaseAuth()
	firebaseAuth.StubHandler = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), UserIDContextKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	t.Run("with sdp request creates new session and send answer to eventbus", func(t *testing.T) {
		r := chi.NewRouter()

		// Dummy FireBase authenticator response
		r.Use(firebaseAuth.Middleware())

		sessionsStorage := &MockSessionStorage{}
		bus := &MockEventBus{}

		r.Post("/", SessionCreateHandler(sessionsStorage, bus, nil))
		ts := httptest.NewServer(r)
		defer ts.Close()

		sessionRequest := &sfu.Session{Sdp: &minimalOfferSdp}
		sessionJson, err := json.Marshal(sessionRequest)
		assert.Nil(t, err)

		req, err := http.NewRequest("POST", ts.URL, strings.NewReader(string(sessionJson)))
		assert.Nil(t, err)

		resp, err := http.DefaultClient.Do(req)
		assert.Nil(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, sessionsStorage.CurrentSession)
		assert.Equal(t, int64(42), sessionsStorage.CurrentSession.ID)
		assert.Equal(t, userID, sessionsStorage.CurrentSession.UserID)
		assert.Equal(t, sfu.SessionIdle, sessionsStorage.CurrentSession.State)
		assert.Equal(t, true, sessionsStorage.CurrentSession.Online)
		assert.Nil(t, sessionsStorage.CurrentSession.MediaType)
		assert.Equal(t, 0, sessionsStorage.CurrentSession.ViewersCount)
		assert.NotNil(t, sessionsStorage.CurrentSession.CreatedAt)
		assert.NotNil(t, sessionsStorage.CurrentSession.UpdatedAt)
		assert.NotNil(t, sessionsStorage.CurrentSession.PeerConnection)
		assert.Greater(t, len(bus.PublishedMessage), 0)
		assert.Equal(t, minimalOfferSdp, *sessionsStorage.CurrentSession.Sdp)
	})

	t.Run("internal server error if Save failed", func(t *testing.T) {
		r := chi.NewRouter()

		// Dummy FireBase authenticator response
		r.Use(firebaseAuth.Middleware())

		sessionsStorage := &MockSessionStorage{
			MockErr: errors.New("Boom!"),
		}
		bus := &MockEventBus{}

		r.Put("/", SessionCreateHandler(sessionsStorage, bus, nil))
		ts := httptest.NewServer(r)
		defer ts.Close()

		sessionRequest := &sfu.Session{Sdp: &minimalOfferSdp}
		sessionJson, err := json.Marshal(sessionRequest)
		assert.Nil(t, err)

		req, err := http.NewRequest("PUT", ts.URL, strings.NewReader(string(sessionJson)))
		assert.Nil(t, err)

		resp, err := http.DefaultClient.Do(req)
		assert.Nil(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("internal server error if Publish sdp has been failed", func(t *testing.T) {
		r := chi.NewRouter()

		// Dummy FireBase authenticator response
		r.Use(firebaseAuth.Middleware())

		sessionsStorage := &MockSessionStorage{}
		bus := &MockEventBus{
			MockErr: errors.New("Bam!"),
		}
		r.Put("/", SessionCreateHandler(sessionsStorage, bus, nil))
		ts := httptest.NewServer(r)
		defer ts.Close()

		sessionRequest := &sfu.Session{Sdp: &minimalOfferSdp}
		sessionJson, err := json.Marshal(sessionRequest)
		assert.Nil(t, err)

		req, err := http.NewRequest("PUT", ts.URL, strings.NewReader(string(sessionJson)))
		assert.Nil(t, err)

		resp, err := http.DefaultClient.Do(req)
		assert.Nil(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}
