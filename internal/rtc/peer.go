package rtc

import (
	"sync"

	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/core"
)

type Peer struct {
	ID           core.UserSessionID
	cfg          config.PeerConfig
	rtcCfg       *config.WebRTCConfig
	session      *core.Session
	lock         sync.RWMutex
	participants map[core.UserSessionID]*Participant
}

func NewPeer(session *core.Session, peerConfig config.PeerConfig, rtcConfig *config.WebRTCConfig) *Peer {
	return &Peer{
		cfg:          peerConfig,
		rtcCfg:       rtcConfig,
		session:      session,
		participants: make(map[core.UserSessionID]*Participant),
	}
}

func (p *Peer) Join(participant *Participant) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.participants[participant.ID] = participant
}
