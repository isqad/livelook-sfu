package rtc

import (
	"log"
	"sync"
	"time"

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

	return &PCTransport{
		pc:                pc,
		me:                me,
		pendingCandidates: make([]webrtc.ICECandidateInit, 0),
	}, nil
}

func newPeerConnection(params TransportParams) (*webrtc.PeerConnection, *webrtc.MediaEngine, error) {
	log.Println("create new peer connection")

	me, err := createMediaEngine(params.EnabledCodecs, params.Config.Publisher)
	if err != nil {
		return nil, nil, err
	}

	se := params.Config.SettingEngine
	se.DisableMediaEngineCopy(true)
	se.DisableSRTPReplayProtection(true)
	se.DisableSRTCPReplayProtection(true)
	se.SetDTLSRetransmissionInterval(dtlsRetransmissionInterval)
	se.SetReceiveMTU(mtu)
	se.SetICETimeouts(iceDisconnectedTimeout, iceFailedTimeout, iceKeepaliveInterval)

	return nil, me, nil
}
