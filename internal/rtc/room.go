package rtc

import (
	"errors"
	"sync"

	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/eventbus/rpc"
)

var (
	errNoParticipant = errors.New("participant is not initialized")
)

type Room struct {
	ID           core.UserSessionID
	cfg          config.PeerConfig
	rtcCfg       config.WebRTCConfig
	lock         sync.RWMutex
	participants map[core.UserSessionID]*Participant

	rpcSink eventbus.Publisher
}

func NewRoom(
	userID core.UserSessionID,
	peerConfig config.PeerConfig,
	rtcConfig config.WebRTCConfig,
	rpcSink eventbus.Publisher,
) *Room {
	return &Room{
		ID:           userID,
		cfg:          peerConfig,
		rtcCfg:       rtcConfig,
		participants: make(map[core.UserSessionID]*Participant),
		rpcSink:      rpcSink,
	}
}

func (r *Room) Join(participant *Participant) {
	r.lock.Lock()
	r.participants[participant.ID] = participant
	r.lock.Unlock()
}

func (r *Room) HandleOffer(userID core.UserSessionID, params rpc.SDPParams) error {
	r.lock.RLock()
	participant := r.participants[userID]
	r.lock.RUnlock()

	if participant == nil {
		return errNoParticipant
	}

	return participant.HandleOffer(params)
}

func (r *Room) AddICECandidate(userID core.UserSessionID, params rpc.ICECandidateParams) error {
	r.lock.RLock()
	participant := r.participants[userID]
	r.lock.RUnlock()

	if participant == nil {
		return errNoParticipant
	}

	return participant.AddICECandidate(params)
}

func (r *Room) PublishStream(userID core.UserSessionID) error {
	return nil
}

// StopStream sends messages to participants about stop host's stream
func (r *Room) StopStream(userID core.UserSessionID) error {
	return nil
}

func (r *Room) Close() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	// Send to all paticipants but current the close signal

	participant := r.participants[r.ID]
	if participant == nil {
		return errNoParticipant
	}

	participant.Close()

	return nil
}
