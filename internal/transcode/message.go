package transcode

import "github.com/isqad/livelook-sfu/internal/core"

// Message transfers data for run a new transcoder
type Message struct {
	// UserID field keep user identificator
	UserID core.UserSessionID `json:"user_id"`
	// SDP field keep session description for connecting with ffmpeg
	SDP    []byte             `json:"sdp"`
}
