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
	"github.com/pion/webrtc/v3"
)

var (
	errRoomNotInitialized = errors.New("room is not initialized")
)

// SessionsManager управляет всеми сессиями пользователей
type SessionsManager struct {
	cfg       *config.Config
	rtcConfig *config.WebRTCConfig
	router    *eventbus.Router

	lock     sync.RWMutex
	sessions map[core.UserSessionID]*rtc.Room

	rpcSink            eventbus.Publisher
	sessionsRepository core.SessionsDBStorer
}

func NewSessionsManager(
	cfg *config.Config,
	router *eventbus.Router,
	sink eventbus.Publisher,
	sessionsRepository core.SessionsDBStorer,
) (*SessionsManager, error) {
	rtcConf, err := config.NewWebRTCConfig(cfg)
	if err != nil {
		return nil, err
	}

	s := &SessionsManager{
		router:             router,
		cfg:                cfg,
		rtcConfig:          rtcConf,
		rpcSink:            sink,
		sessionsRepository: sessionsRepository,
		sessions:           make(map[core.UserSessionID]*rtc.Room),
	}

	router.OnJoin(s.StartSession)
	router.OnOffer(s.HandleOffer)
	router.OnAddICECandidate(s.AddICECandidate)
	router.OnCloseSession(s.CloseSession)

	return s, nil
}

func (s *SessionsManager) StartSession(userID core.UserSessionID) error {
	log.Println("received message to start session")

	session := &core.Session{
		UserID: userID,
	}
	_, err := s.sessionsRepository.Save(session)
	if err != nil {
		return err
	}

	room, err := s.findOrInitRoom(userID)
	if err != nil {
		return err
	}

	participant, err := rtc.NewParticipant(userID, s.rpcSink, s.cfg.Peer.EnabledCodecs, s.rtcConfig)
	if err != nil {
		return err
	}

	room.Join(participant)

	// Send Join RPC
	msg := eventbus.NewJoinRpc()
	if err := s.rpcSink.PublishClient(userID, msg); err != nil {
		return err
	}

	telemetry.SessionStarted()

	return nil
}

func (s *SessionsManager) HandleOffer(userID core.UserSessionID, sdp *webrtc.SessionDescription) error {
	room, err := s.findRoom(userID)
	if err != nil {
		return err
	}

	return room.HandleOffer(userID, sdp)
}

func (s *SessionsManager) AddICECandidate(userID core.UserSessionID, candidate *webrtc.ICECandidateInit) error {
	room, err := s.findRoom(userID)
	if err != nil {
		return err
	}

	return room.AddICECandidate(userID, candidate)
}

func (s *SessionsManager) CloseSession(userID core.UserSessionID) error {
	room, err := s.findRoom(userID)
	if err != nil {
		return err
	}

	if err := room.Close(); err != nil {
		telemetry.ServiceOperationCounter.WithLabelValues("sessions", "error", "close").Add(1)
		log.Printf("close session error: %v", err)
	}

	if err := s.sessionsRepository.SetOffline(userID); err != nil {
		telemetry.ServiceOperationCounter.WithLabelValues("database", "error", "session_set_offline").Add(1)
		log.Printf("set offline errored: %v", err)
	}

	s.lock.Lock()
	delete(s.sessions, userID)
	s.lock.Unlock()

	telemetry.SessionStopped()

	return nil
}

func (s *SessionsManager) findOrInitRoom(userID core.UserSessionID) (*rtc.Room, error) {
	s.lock.RLock()
	room := s.sessions[userID]
	s.lock.RUnlock()

	if room != nil {
		return room, nil
	}

	room = rtc.NewRoom(userID, s.cfg.Peer, s.rtcConfig, s.rpcSink)

	s.lock.Lock()
	s.sessions[userID] = room
	s.lock.Unlock()

	return room, nil
}

func (s *SessionsManager) findRoom(userID core.UserSessionID) (*rtc.Room, error) {
	s.lock.RLock()
	room := s.sessions[userID]
	s.lock.RUnlock()

	if room != nil {
		return room, nil
	}

	return nil, errRoomNotInitialized
}
