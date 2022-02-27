package sfu

import (
	"encoding/json"
	"io"
	"log"
	"time"

	"github.com/pion/webrtc/v3"
)

type SessionState string

const (
	SessionIdle     SessionState = "idle"
	SingleBroadcast SessionState = "broadcast_single"
	MultiBroadcast  SessionState = "broadcast_multi"
	SessionViewer   SessionState = "viewer"
)

type SessionMediaType string

const (
	VideoSession SessionMediaType = "video"
	AudioSession SessionMediaType = "audio"
)

type Session struct {
	ID            int64                      `json:"id,omitempty" db:"id"`
	UserID        string                     `json:"user_id" db:"user_id"`
	Title         string                     `json:"title" db:"title"`
	CreatedAt     time.Time                  `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt     time.Time                  `json:"updated_at,omitempty" db:"updated_at"`
	ImageNode     *int                       `json:"image_node,omitempty" db:"image_node"`
	ImageFilename *string                    `json:"image_filename,omitempty" db:"image_filename"`
	Online        bool                       `json:"online,omitempty" db:"online"`
	State         SessionState               `json:"state,omitempty" db:"state"`
	MediaType     *SessionMediaType          `json:"media_type,omitempty" db:"media_type"`
	ViewersCount  int                        `json:"viewers_count,omitempty" db:"viewers_count"`
	FinishedAt    *time.Time                 `json:"finished_at,omitempty" db:"finished_at"`
	Sdp           *webrtc.SessionDescription `json:"sdp,omitempty" db:"-"`

	PeerConnection *webrtc.PeerConnection `db:"-" json:"-"`
}

// NewSessionFromReader creates session from incoming request
func NewSessionFromReader(userID string, r io.Reader) (*Session, error) {
	s := &Session{}

	err := json.NewDecoder(r).Decode(s)
	if err != nil {
		return nil, err
	}

	s.UserID = userID
	s.State = SessionIdle
	s.Online = true
	s.CreatedAt = time.Now().UTC()
	s.UpdatedAt = time.Now().UTC()

	return s, nil
}

func (s *Session) EstablishPeerConnection() error {
	peerConnection, err := webrtc.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		return err
	}

	if _, err := peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		return err
	}
	if _, err := peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		return err
	}

	err = peerConnection.SetRemoteDescription(*s.Sdp)
	if err != nil {
		return err
	}

	s.PeerConnection = peerConnection

	return nil
}

func (s *Session) CreateWebrtcAnswer() ([]byte, error) {
	answer, err := s.PeerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}
	gatherComplete := webrtc.GatheringCompletePromise(s.PeerConnection)
	err = s.PeerConnection.SetLocalDescription(answer)
	if err != nil {
		return nil, err
	}
	<-gatherComplete
	// TODO: ICE Trickle
	log.Println("ICE candidates gathered!")

	return NewSdpJSONRpc(s.PeerConnection.LocalDescription(), "answer")
}

func (s *Session) Close() error {
	return s.PeerConnection.Close()
}
