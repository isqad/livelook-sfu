package rtc

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/isqad/livelook-sfu/internal/eventbus/rpc"

	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/telemetry"
	"github.com/pion/rtcp"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v3"
)

const (
	ReliableDataChannel = "_reliable"
)

type Participant struct {
	sync.RWMutex

	ID              core.UserSessionID
	publisher       *PCTransport
	subscriber      *PCTransport
	reliableDC      *webrtc.DataChannel
	publishedTracks map[MediaTrackID]*MediaTrack
	sink            eventbus.Publisher
	rtcConf         *config.WebRTCConfig

	// TODO: extract into TranscoderGateway
	portsAllocator *PortsAllocator
	allocatedPorts map[webrtc.PayloadType]int
	transcoderSDP  *sdp.SessionDescription
}

type ParticipantOptions struct {
	UserID         core.UserSessionID
	RpcSink        eventbus.Publisher
	EnabledCodecs  config.EnabledCodecs
	RtcConf        *config.WebRTCConfig
	PortsAllocator *PortsAllocator
}

func NewParticipant(opts ParticipantOptions) (*Participant, error) {
	var err error

	p := &Participant{
		ID:              opts.UserID,
		sink:            opts.RpcSink,
		rtcConf:         opts.RtcConf,
		publishedTracks: make(map[MediaTrackID]*MediaTrack),
		portsAllocator:  opts.PortsAllocator,
		allocatedPorts:  make(map[webrtc.PayloadType]int),
	}

	if p.transcoderSDP, err = sdp.NewJSEPSessionDescription(false); err != nil {
		return nil, err
	}

	if p.publisher, err = NewPCTransport(TransportParams{
		EnabledCodecs: opts.EnabledCodecs,
		Config:        opts.RtcConf,
		Target:        rpc.Publisher,
	}); err != nil {
		return nil, err
	}

	p.publisher.pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Interface("candidate", candidate).Msg("send ICE candidate")

		if err := p.sendICECandidate(candidate, rpc.Publisher); err != nil {
			log.Error().Err(err).Str("service", "participant").Str("ID", string(p.ID)).Msg("error on send ICE candidate")
		}
	})
	p.publisher.pc.OnConnectionStateChange(p.handlePrimaryStateChange)
	p.publisher.pc.OnDataChannel(p.onDataChannel)
	p.publisher.pc.OnTrack(p.onMediaTrack)

	// allocate udp ports for transcoder
	for codecType, codecs := range enabledCodecParams {
		for _, codecParams := range codecs {
			udpPort, err := p.portsAllocator.Allocate()
			if err != nil {
				return nil, err
			}

			p.allocatedPorts[codecParams.PayloadType] = udpPort

			md := sdp.NewJSEPMediaDescription(codecType.String(), []string{})
			codecName := strings.TrimPrefix(codecParams.MimeType, "audio/")
			codecName = strings.TrimPrefix(codecName, "video/")
			md.WithCodec(
				uint8(codecParams.PayloadType),
				codecName,
				codecParams.ClockRate,
				codecParams.Channels,
				codecParams.SDPFmtpLine,
			)

			md.MediaName.Port = sdp.RangedPort{Value: udpPort}

			p.transcoderSDP.WithMedia(md)
		}
	}

	sd, err := p.transcoderSDP.Marshal()
	if err != nil {
		return nil, err
	}

	rootDir := viper.GetString("app.streams_root_dir")
	if err := os.MkdirAll(rootDir+"/"+string(p.ID), 0755); err != nil {
		return nil, err
	}

	f, err := os.Create(rootDir + "/" + string(p.ID) + "/transcoder.sdp")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := f.Write(sd); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Participant) AddICECandidate(params rpc.ICECandidateParams) error {
	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Str("target", string(params.Target)).Msg("add ICE candidate")

	if params.Target == rpc.Publisher {
		return p.publisher.AddICECandidate(params.ICECandidateInit)
	} else {
		return p.subscriber.AddICECandidate(params.ICECandidateInit)
	}
}

func (p *Participant) sendICECandidate(candidate *webrtc.ICECandidate, target rpc.SignalingTarget) error {
	if candidate == nil {
		return nil
	}

	candidateInit := candidate.ToJSON()
	rpc := rpc.NewICECandidateRpc(candidateInit, target)

	if err := p.sink.PublishClient(p.ID, rpc); err != nil {
		return err
	}

	return nil
}

func (p *Participant) HandleOffer(params rpc.SDPParams) error {
	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Interface("params", params).Msg("handle offer")

	if params.Target == rpc.Publisher {
		if err := p.publisher.SetRemoteDescription(params.SessionDescription); err != nil {
			return err
		}

		answer, err := p.publisher.pc.CreateAnswer(nil)
		if err != nil {
			return err
		}

		log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Interface("sdp", answer).Msg("created answer")

		err = p.publisher.pc.SetLocalDescription(answer)
		if err != nil {
			return err
		}

		rpc := rpc.NewSDPAnswerRpc(p.publisher.pc.LocalDescription(), rpc.Publisher)
		if err := p.sink.PublishClient(p.ID, rpc); err != nil {
			return err
		}
	} else {
		log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Msg("handle offer of the receiver")
	}

	return nil
}

func (p *Participant) StartPublish() error {
	return nil
}

func (p *Participant) StopPublish() {

}

func (p *Participant) handlePrimaryStateChange(state webrtc.PeerConnectionState) {
	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Str("state", state.String()).Msg("connection state changed")

	if state == webrtc.PeerConnectionStateConnected {
		telemetry.ServiceOperationCounter.WithLabelValues("ice_connection", "success", "").Add(1)
	} else if state == webrtc.PeerConnectionStateFailed {
		telemetry.ServiceOperationCounter.WithLabelValues("ice_connection", "error", "state_failed").Add(1)
		p.closeSignalConnection()
		p.Close()
	}
}

// func (p *Participant) handleSecondaryStateChange(state webrtc.PeerConnectionState) {
// 	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Str("state", state.String()).Msg("secondary connection state changed")

// 	if state == webrtc.PeerConnectionStateConnected {
// 		telemetry.ServiceOperationCounter.WithLabelValues("ice_connection_receiver", "success", "").Add(1)
// 	} else if state == webrtc.PeerConnectionStateFailed {
// 		telemetry.ServiceOperationCounter.WithLabelValues("ice_connection_receiver", "error", "state_failed").Add(1)
// 		p.closeSignalConnection()
// 	}
// }

func (p *Participant) onDataChannel(dc *webrtc.DataChannel) {
	switch dc.Label() {
	case ReliableDataChannel:
		p.reliableDC = dc
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			// Just ignore
		})
	default:
		log.Error().Str("service", "participant").Str("ID", string(p.ID)).Str("label", dc.Label()).Msg("unsupported datachannel added")
	}
}

// Метод вызывается для каждого трека
func (p *Participant) onMediaTrack(track *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
	payloadType := track.Codec().PayloadType

	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Uint8("codec type", uint8(payloadType)).Msg("on media track")

	// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
	go func() {
		ticker := time.NewTicker(time.Second * 2)
		for range ticker.C {
			if rtcpErr := p.publisher.pc.WriteRTCP(
				[]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}},
			); rtcpErr != nil {
				log.Error().Err(rtcpErr).Str("service", "participant").Str("ID", string(p.ID)).Msg("")
				return
			}
		}
	}()

	id := MediaTrackID(track.ID())
	mt, err := NewMediaTrack(id, payloadType, p.allocatedPorts[payloadType])
	if err != nil {
		log.Error().Err(err).Str("service", "participant").Str("ID", string(p.ID)).Msg("")
		return
	}

	p.Lock()
	p.publishedTracks[id] = mt
	p.Unlock()

	mt.ForwardRTP(track, rtpReceiver)
}

// closes signal connection to notify client to resume/reconnect
func (p *Participant) closeSignalConnection() {
	// Need to send RPC to client
}

func (p *Participant) Close() {
	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Msg("close participant")

	p.Lock()
	defer p.Unlock()

	for _, t := range p.publishedTracks {
		t.Close()
		delete(p.publishedTracks, t.ID)
	}

	for _, port := range p.allocatedPorts {
		p.portsAllocator.Deallocate(port)
	}

	p.publishedTracks = nil
	p.allocatedPorts = nil
	// Close peer connections without blocking participant close. If peer connections are gathering candidates
	// Close will block.
	go func() {
		p.publisher.Close()
		// p.subscriber.Close()
	}()
}
