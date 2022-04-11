package service

import (
	"errors"
	"log"
	"sync"

	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/rtc"
	"github.com/isqad/livelook-sfu/internal/telemetry"
)

// SessionsManager управляет всеми сессиями пользователей
type SessionsManager struct {
	cfg       *config.Config
	rtcConfig *config.WebRTCConfig
	router    *eventbus.Router

	lock     sync.RWMutex
	sessions map[core.UserSessionID]*rtc.Peer

	rpcSink eventbus.Publisher
}

func NewSessionsManager(
	cfg *config.Config,
	router *eventbus.Router,
	sink eventbus.Publisher,
) (*SessionsManager, error) {
	rtcConf, err := config.NewWebRTCConfig(cfg)
	if err != nil {
		return nil, err
	}

	s := &SessionsManager{
		router:    router,
		cfg:       cfg,
		rtcConfig: rtcConf,
		rpcSink:   sink,
		sessions:  make(map[core.UserSessionID]*rtc.Peer),
	}

	router.OnCreateOrUpdateSession(s.StartSession)

	return s, nil
}

func (s *SessionsManager) StartSession(userID core.UserSessionID, session *core.Session) error {
	log.Println("received message to start session")

	if session.Sdp == nil {
		return errors.New("no sdp in session")
	}

	peer, err := s.findOrInitPeer(userID, session)
	if err != nil {
		return err
	}

	participant, err := rtc.NewParticipant(userID, s.rpcSink, s.cfg.Peer.EnabledCodecs, s.rtcConfig)
	if err != nil {
		return err
	}

	peer.Join(participant)

	telemetry.SessionStarted()

	// TODO: публикацию отделить от создания сессии
	// должно быть постоянным только subscriber peer соединение, а publisher pc должен быть активен только по отдельному запросу
	return participant.HandleOffer(*session.Sdp)
}

func (s *SessionsManager) findOrInitPeer(userID core.UserSessionID, session *core.Session) (*rtc.Peer, error) {
	s.lock.RLock()
	peer := s.sessions[userID]
	s.lock.RUnlock()

	if peer != nil {
		return peer, nil
	}

	peer = rtc.NewPeer(session, s.cfg.Peer, s.rtcConfig, s.rpcSink)

	s.lock.Lock()
	s.sessions[userID] = peer
	s.lock.Unlock()

	return peer, nil
}
