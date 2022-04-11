package rtc

import (
	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/core"
)

type Participant struct {
	ID        core.UserSessionID
	publisher *PCTransport
}

func NewParticipant(userID core.UserSessionID, enabledCodecs []config.CodecSpec, rtcConf *config.WebRTCConfig) (*Participant, error) {
	p := &Participant{
		ID: userID,
	}

	t, err := NewPCTransport(TransportParams{
		EnabledCodecs: enabledCodecs,
		Config:        rtcConf,
	})
	if err != nil {
		return nil, err
	}

	p.publisher = t

	return p, nil
}
