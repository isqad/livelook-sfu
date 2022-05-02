package rtc

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/pion/webrtc/v3"
)

const (
	rtcpPLIInterval            = time.Second * 3
	dtlsRetransmissionInterval = 100 * time.Millisecond
	mtu                        = 1400
	iceDisconnectedTimeout     = 10 * time.Second // compatible for ice-lite with firefox client
	iceFailedTimeout           = 25 * time.Second // pion's default
	iceKeepaliveInterval       = 2 * time.Second  // pion's default
)

type PCTransport struct {
	pc *webrtc.PeerConnection
	me *webrtc.MediaEngine

	lock              sync.Mutex
	pendingCandidates []webrtc.ICECandidateInit
}

type TransportParams struct {
	EnabledCodecs []config.CodecSpec
	Config        *config.WebRTCConfig
}

func NewPCTransport(params TransportParams) (*PCTransport, error) {
	pc, me, err := newPeerConnection(params)
	if err != nil {
		return nil, err
	}

	t := &PCTransport{
		pc:                pc,
		me:                me,
		pendingCandidates: make([]webrtc.ICECandidateInit, 0),
	}

	t.pc.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
		if state == webrtc.ICEGathererStateComplete {
			log.Debug().Str("service", "pcTransport").Msg("OnICEGatheringStateChange: complete")
		}
	})

	return t, nil
}

func newPeerConnection(params TransportParams) (*webrtc.PeerConnection, *webrtc.MediaEngine, error) {
	log.Debug().Str("service", "pcTransport").Msg("create new peer connection")

	me, err := createMediaEngine(params.EnabledCodecs, params.Config.Publisher)
	if err != nil {
		log.Error().Err(err).Str("service", "pcTransport").Msg("")
		return nil, nil, err
	}

	se := params.Config.SettingEngine
	se.DisableMediaEngineCopy(true)
	se.DisableSRTPReplayProtection(true)
	se.DisableSRTCPReplayProtection(true)
	se.SetDTLSRetransmissionInterval(dtlsRetransmissionInterval)
	se.SetReceiveMTU(mtu)
	se.SetICETimeouts(iceDisconnectedTimeout, iceFailedTimeout, iceKeepaliveInterval)

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(me),
		webrtc.WithSettingEngine(se),
	)

	pc, err := api.NewPeerConnection(params.Config.Configuration)

	return pc, me, err
}

func (t *PCTransport) AddICECandidate(candidate *webrtc.ICECandidateInit) error {
	desc := t.pc.RemoteDescription()
	if desc != nil {
		t.pc.AddICECandidate(*candidate)
		return nil
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	t.pendingCandidates = append(t.pendingCandidates, *candidate)

	return nil
}

func (t *PCTransport) SetRemoteDescription(sdp webrtc.SessionDescription) error {
	if err := t.pc.SetRemoteDescription(sdp); err != nil {
		return err
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	for _, candidate := range t.pendingCandidates {
		t.pc.AddICECandidate(candidate)
	}

	t.pendingCandidates = make([]webrtc.ICECandidateInit, 0)

	return nil
}

func (t *PCTransport) Close() {
	_ = t.pc.Close()
}
