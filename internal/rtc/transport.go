package rtc

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/eventbus/rpc"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/interceptor/pkg/gcc"
	"github.com/pion/interceptor/pkg/twcc"
	"github.com/pion/sdp/v3"
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

	// stream allocator for subscriber PC
	streamAllocator *StreamAllocator

	lock              sync.Mutex
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

func newPeerConnection(params TransportParams, onBandwidthEstimator func(estimator cc.BandwidthEstimator)) (*webrtc.PeerConnection, *webrtc.MediaEngine, error) {
	var directionConfig config.DirectionConfig

	if params.Target == rpc.Publisher {
		directionConfig = params.Config.Publisher
	} else {
		directionConfig = params.Config.Subscriber
	}

	log.Debug().Str("service", "pcTransport").Msgf("create new peer connection for %s", params.Target)
	me, err := createMediaEngine(params.EnabledCodecs, directionConfig)
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

	ir := &interceptor.Registry{}
	if params.Target == rpc.Receiver {
		isSendSideBWE := false
		for _, ext := range directionConfig.RTPHeaderExtension.Video {
			if ext == sdp.TransportCCURI {
				isSendSideBWE = true
				break
			}
		}
		for _, ext := range directionConfig.RTPHeaderExtension.Audio {
			if ext == sdp.TransportCCURI {
				isSendSideBWE = true
				break
			}
		}

		if isSendSideBWE {
			gf, err := cc.NewInterceptor(func() (cc.BandwidthEstimator, error) {
				return gcc.NewSendSideBWE(
					gcc.SendSideBWEInitialBitrate(1*1000*1000),
					gcc.SendSideBWEPacer(gcc.NewNoOpPacer()),
				)
			})
			if err == nil {
				gf.OnNewPeerConnection(func(id string, estimator cc.BandwidthEstimator) {
					if onBandwidthEstimator != nil {
						onBandwidthEstimator(estimator)
					}
				})
				ir.Add(gf)

				tf, err := twcc.NewHeaderExtensionInterceptor()
				if err == nil {
					ir.Add(tf)
				}
			}
		}
	}

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

	t.lock.Lock()
	defer t.lock.Unlock()

	t.pendingCandidates = append(t.pendingCandidates, candidate)

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
