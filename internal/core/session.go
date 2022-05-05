package core

import (
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
	UserID        UserSessionID              `json:"user_id" db:"user_id"`
	Title         string                     `json:"title" db:"title"`
	CreatedAt     time.Time                  `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt     time.Time                  `json:"updated_at,omitempty" db:"updated_at"`
	ImageNode     *int                       `json:"image_node,omitempty" db:"image_node"`
	ImageFilename *string                    `json:"image_filename,omitempty" db:"image_filename"`
	ImageURL      string                     `json:"image_url,omitempty" db:"-"`
	Online        bool                       `json:"is_online,omitempty" db:"is_online"`
	State         SessionState               `json:"state,omitempty" db:"state"`
	MediaType     *SessionMediaType          `json:"media_type,omitempty" db:"media_type"`
	ViewersCount  int                        `json:"viewers_count,omitempty" db:"viewers_count"`
	FinishedAt    *time.Time                 `json:"finished_at,omitempty" db:"finished_at"`
	Sdp           *webrtc.SessionDescription `json:"sdp,omitempty" db:"-"`
}
