package rtc

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/eventbus/rpc"
	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/webrtc/v3"
)

const (
	rtcpPLIInterval            = time.Second * 3
	dtlsRetransmissionInterval = 100 * time.Millisecond
	mtu                        = 1460             // Equal to UDP MTU, pion's default
	iceDisconnectedTimeout     = 10 * time.Second // compatible for ice-lite with firefox client
	iceFailedTimeout           = 25 * time.Second // pion's default
	iceKeepaliveInterval       = 2 * time.Second  // pion's default
)

type PCTransport struct {
	sync.Mutex

	pc *webrtc.PeerConnection
	me *webrtc.MediaEngine

	// stream allocator for subscriber PC
	streamAllocator   *StreamAllocator
	pendingCandidates []webrtc.ICECandidateInit
}

type TransportParams struct {
	EnabledCodecs []config.CodecSpec
	Config        *config.WebRTCConfig
	Target        rpc.SignalingTarget
}

func NewPCTransport(params TransportParams) (*PCTransport, error) {
	var bwe cc.BandwidthEstimator

	pc, me, err := newPeerConnection(params, func(estimator cc.BandwidthEstimator) {
		bwe = estimator
	})
	if err != nil {
		return nil, err
	}

	t := &PCTransport{
		pc:                pc,
		me:                me,
		pendingCandidates: make([]webrtc.ICECandidateInit, 0),
	}

	if params.Target == rpc.Receiver {
		t.streamAllocator = NewStreamAllocator()
		if bwe != nil {
			t.streamAllocator.SetBandwidthEstimator(bwe)
		}
	}

	t.pc.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
		if state == webrtc.ICEGathererStateComplete {
			log.Debug().Str("service", "pcTransport").Msg("OnICEGatheringStateChange: complete")
		}
	})

	return t, nil
}

func newPeerConnection(
	params TransportParams,
	onBandwidthEstimator func(estimator cc.BandwidthEstimator),
) (*webrtc.PeerConnection, *webrtc.MediaEngine, error) {
	var directionConfig config.DirectionConfig

	if params.Target == rpc.Publisher {
		directionConfig = params.Config.Publisher
	} else {
		directionConfig = params.Config.Subscriber
	}

	log.Debug().Str("service", "pcTransport").Msgf("create new peer connection for %s", params.Target)
	me, ir, err := createMediaEngine(params.EnabledCodecs, directionConfig, params.Target)
	if err != nil {
		log.Error().Err(err).Str("service", "pcTransport").Msg("")
		return nil, nil, err
	}
	log.Debug().Str("service", "pcTransport").Msgf("interceptor registry: %+v", ir)

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
		webrtc.WithInterceptorRegistry(ir),
	)

	pc, err := api.NewPeerConnection(params.Config.Configuration)

	return pc, me, err
}

func (t *PCTransport) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	desc := t.pc.RemoteDescription()
	if desc != nil {
		t.pc.AddICECandidate(candidate)
		return nil
	}

	t.Lock()
	defer t.Unlock()

	t.pendingCandidates = append(t.pendingCandidates, candidate)

	return nil
}

func (t *PCTransport) SetRemoteDescription(sdp webrtc.SessionDescription) error {
	if err := t.pc.SetRemoteDescription(sdp); err != nil {
		return err
	}

	t.Lock()
	defer t.Unlock()

	for _, candidate := range t.pendingCandidates {
		if err := t.pc.AddICECandidate(candidate); err != nil {
			return err
		}
	}

	t.pendingCandidates = make([]webrtc.ICECandidateInit, 0)

	return nil
}

func (t *PCTransport) Close() {
	_ = t.pc.Close()
}
