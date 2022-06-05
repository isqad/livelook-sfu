package service

import (
	"errors"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/eventbus/rpc"
	"github.com/isqad/livelook-sfu/internal/rtc"
	"github.com/isqad/livelook-sfu/internal/telemetry"
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
	router.OnPublishStream(s.PublishStream)
	router.OnStopStream(s.StopStream)
	router.OnSubscribeStream(s.Subscribe)
	router.OnSubscribeStreamCancel(s.Unsubscribe)

	return s, nil
}

func (s *SessionsManager) StartSession(userID core.UserSessionID) error {
	log.Debug().Str("service", "sessionsManager").Str("UserID", string(userID)).Msg("received message to start session")

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

	// RTC-конфиг копируется для каждого participant'а
	rtcConf := *s.rtcConfig
	participant, err := rtc.NewParticipant(userID, s.rpcSink, s.cfg.Peer.EnabledCodecs, &rtcConf)
	if err != nil {
		return err
	}

	room.Join(participant)

	// Send Join RPC
	msg := rpc.NewJoinRpc()
	if err := s.rpcSink.PublishClient(userID, msg); err != nil {
		return err
	}

	telemetry.SessionStarted()

	return nil
}

func (s *SessionsManager) HandleOffer(userID core.UserSessionID, params rpc.SDPParams) error {
	log.Debug().Str("service", "sessionsManager").Str("UserID", string(userID)).Msg("handle offer")

	room, err := s.findRoom(userID)
	if err != nil {
		log.Error().Str("service", "sessionsManager").Str("UserID", string(userID)).Err(err).Msg("room not found")
		return err
	}

	return room.HandleOffer(userID, params)
}

func (s *SessionsManager) AddICECandidate(userID core.UserSessionID, params rpc.ICECandidateParams) error {
	room, err := s.findRoom(userID)
	if err != nil {
		log.Error().Str("service", "sessionsManager").Str("UserID", string(userID)).Err(err).Msg("room not found")
		return err
	}

	return room.AddICECandidate(userID, params)
}

func (s *SessionsManager) CloseSession(userID core.UserSessionID) error {
	log.Debug().Str("service", "sessionsManager").Str("UserID", string(userID)).Msg("close session")

	room, err := s.findRoom(userID)
	if err != nil {
		log.Error().Str("service", "sessionsManager").Str("UserID", string(userID)).Err(err).Msg("room not found")
		return err
	}

	if err := room.Close(); err != nil {
		telemetry.ServiceOperationCounter.WithLabelValues("sessions", "error", "close").Add(1)
		log.Error().Str("service", "sessionsManager").Str("UserID", string(userID)).Err(err).Msg("close session error")
	}

	if err := s.sessionsRepository.SetOffline(userID); err != nil {
		telemetry.ServiceOperationCounter.WithLabelValues("database", "error", "session_set_offline").Add(1)
		log.Error().Str("service", "sessionsManager").Str("UserID", string(userID)).Err(err).Msg("set offline errored")
	}

	s.lock.Lock()
	delete(s.sessions, userID)
	s.lock.Unlock()

	telemetry.SessionStopped()

	return nil
}

func (s *SessionsManager) PublishStream(userID core.UserSessionID) error {
	log.Debug().Str("service", "sessionsManager").Str("UserID", string(userID)).Msg("publish stream")

	room, err := s.findRoom(userID)
	if err != nil {
		log.Error().Str("service", "sessionsManager").Str("UserID", string(userID)).Err(err).Msg("room not found")
		return err
	}

	if err := s.sessionsRepository.StartPublish(userID); err != nil {
		log.Error().Str("service", "sessionsManager").Str("UserID", string(userID)).Err(err).Msg("can't publish")
		return err
	}

	if err := room.PublishStream(userID); err != nil {
		log.Error().Str("service", "sessionsManager").Str("UserID", string(userID)).Err(err).Msg("publishing error")
		return err
	}

	return nil
}

func (s *SessionsManager) StopStream(userID core.UserSessionID) error {
	log.Debug().Str("service", "sessionsManager").Str("UserID", string(userID)).Msg("stop stream")

	room, err := s.findRoom(userID)
	if err != nil {
		log.Error().Str("service", "sessionsManager").Str("UserID", string(userID)).Err(err).Msg("room not found")
		return err
	}

	if err := s.sessionsRepository.StopPublish(userID); err != nil {
		log.Error().Str("service", "sessionsManager").Str("UserID", string(userID)).Err(err).Msg("can't stop stream")
		return err
	}

	if err := room.StopStream(userID); err != nil {
		log.Error().Str("service", "sessionsManager").Str("UserID", string(userID)).Err(err).Msg("stop stream error")
		return err
	}

	return nil
}

func (s *SessionsManager) Subscribe(userID core.UserSessionID, streamerUserID core.UserSessionID) error {
	log.Debug().Str("service", "sessionsManager").Str("UserID", string(userID)).Str("StreamerID", string(streamerUserID)).Msg("creating a subscription to the streaming")

	return nil
}

func (s *SessionsManager) Unsubscribe(userID core.UserSessionID, streamerUserID core.UserSessionID) error {
	log.Debug().Str("service", "sessionsManager").Str("UserID", string(userID)).Str("StreamerID", string(streamerUserID)).Msg("cancel subscription to the streaming")

	return nil
}

// Close sends messages about terminate the server to all active clients
func (s *SessionsManager) Close() error {
	return nil
}

func (s *SessionsManager) findOrInitRoom(userID core.UserSessionID) (*rtc.Room, error) {
	s.lock.RLock()
	room := s.sessions[userID]
	s.lock.RUnlock()

	if room != nil {
		return room, nil
	}

	room = rtc.NewRoom(userID, s.cfg.Peer, *s.rtcConfig, s.rpcSink)

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
