package transcode

import "github.com/isqad/livelook-sfu/internal/core"

type Messsage struct {
	UserID core.UserSessionID `json:user_id`
	SDP    []byte             `json:sdp`
}
