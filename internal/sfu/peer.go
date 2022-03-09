package sfu

import (
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

const (
	rtcpPLIInterval = time.Second * 3
)

var (
	errConnectionNotInitialized = errors.New("connection is not initialized")
)

type peer struct {
	session    *core.Session
	connection *webrtc.PeerConnection

	iceCandidates []*webrtc.ICECandidateInit

	localVideoTrack *webrtc.TrackLocalStaticRTP
	localAudioTrack *webrtc.TrackLocalStaticRTP

	remotePeers []*peer
}

func (p *peer) establishPeerConnection(eventsPublisher eventbus.Publisher) error {
	peerConnection, err := webrtc.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		return err
	}

	peerConnection.OnICECandidate(p.onICECandidate(eventsPublisher))

	if _, err := peerConnection.AddTransceiverFromKind(
		webrtc.RTPCodecTypeAudio,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendrecv},
	); err != nil {
		return err
	}
	if _, err := peerConnection.AddTransceiverFromKind(
		webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendrecv},
	); err != nil {
		return err
	}

	peerConnection.OnTrack(p.onTrack)
	peerConnection.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		log.Printf("OnConnectionStateChange: %v", pcs)
	})
	peerConnection.OnICEConnectionStateChange(func(is webrtc.ICEConnectionState) {
		log.Printf("OnICEConnectionStateChange: %v", is)
	})
	peerConnection.OnICEGatheringStateChange(func(is webrtc.ICEGathererState) {
		log.Printf("OnICEGatheringStateChange: %v", is)
	})
	peerConnection.OnNegotiationNeeded(func() {
		log.Println("OnNegotiationNeeded")
	})
	peerConnection.OnSignalingStateChange(func(ss webrtc.SignalingState) {
		log.Printf("OnSignalingStateChange: %v", ss)
	})

	p.connection = peerConnection

	return nil
}

func (p *peer) setRemoteDescription(sdp webrtc.SessionDescription) error {
	if p.connection == nil {
		return errConnectionNotInitialized
	}

	return p.connection.SetRemoteDescription(sdp)
}

func (p *peer) addICECandidate(candidate *webrtc.ICECandidateInit) error {
	if p.connection == nil {
		return errConnectionNotInitialized
	}

	p.iceCandidates = append(p.iceCandidates, candidate)

	if p.connection.CurrentRemoteDescription() == nil {
		return nil
	}

	defer p.clearCandidates()

	for _, c := range p.iceCandidates {
		iceCandidate := *c

		if err := p.connection.AddICECandidate(iceCandidate); err != nil {
			return err
		}
	}

	return nil
}

func (p *peer) clearCandidates() {
	p.iceCandidates = []*webrtc.ICECandidateInit{}
}

func (p *peer) createAnswer() (*eventbus.SDPRpc, error) {
	if p.connection == nil {
		return nil, errConnectionNotInitialized
	}

	answer, err := p.connection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	err = p.connection.SetLocalDescription(answer)
	if err != nil {
		return nil, err
	}

	rpc := eventbus.NewSDPAnswerRpc(p.connection.LocalDescription())

	return rpc, nil
}

func (p *peer) listenAndAccept() {}

func (p *peer) close() error {
	if p.connection == nil {
		return errConnectionNotInitialized
	}

	return p.connection.Close()
}

func (p *peer) onICECandidate(eventsPublisher eventbus.Publisher) func(*webrtc.ICECandidate) {
	return func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			log.Println("No more ICE candidates")
			return
		}

		candidateInit := candidate.ToJSON()
		rpc := eventbus.NewICECandidateRpc(&candidateInit)

		if err := eventsPublisher.PublishClient(p.session.UserID, rpc); err != nil {
			log.Printf("onICECandidate: error %v", err)
			return
		}
	}
}

func (p *peer) onTrack(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	if remoteTrack.Kind() == webrtc.RTPCodecTypeVideo {
		if err := p.createLocalVideoTrackForwarding(remoteTrack); err != nil {
			fmt.Printf("onTrack create localVideoTrack error: %v", err)
			return
		}
	}

	if remoteTrack.Kind() == webrtc.RTPCodecTypeAudio {
		if err := p.createLocalAudioTrackForwarding(remoteTrack); err != nil {
			fmt.Printf("onTrack create localVideoTrack error: %v", err)
			return
		}
	}
}

func (p *peer) createLocalVideoTrackForwarding(remoteTrack *webrtc.TrackRemote) error {
	log.Println("createLocalVideoTrackForwarding")
	// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
	// This can be less wasteful by processing incoming RTCP events, then we would emit a NACK/PLI when a viewer requests it
	go func() {
		ticker := time.NewTicker(rtcpPLIInterval)
		for range ticker.C {
			if err := p.connection.WriteRTCP(
				[]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(remoteTrack.SSRC())}},
			); err != nil {
				fmt.Printf("onTrack send PLI error: %v", err)
				// TODO: return if closed pipe
			}
		}
	}()

	// Create a local track, all our SFU clients will be fed via this track
	localVideoTrack, err := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, "video", "pion")
	if err != nil {
		return err
	}

	p.localVideoTrack = localVideoTrack

	return forwardPacketsToLocalTrack(remoteTrack, localVideoTrack)
}

func (p *peer) createLocalAudioTrackForwarding(remoteTrack *webrtc.TrackRemote) error {
	log.Println("createLocalAudioTrackForwarding")
	// Create a local track, all our SFU clients will be fed via this track
	localAudioTrack, err := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, "audio", "pion")
	if err != nil {
		return err
	}

	p.localAudioTrack = localAudioTrack

	return forwardPacketsToLocalTrack(remoteTrack, localAudioTrack)
}

func forwardPacketsToLocalTrack(remoteTrack *webrtc.TrackRemote, localTrack *webrtc.TrackLocalStaticRTP) error {
	log.Printf("forwardPacketsToLocalTrack: %s", remoteTrack.Kind().String())

	defer func() { log.Printf("forwardPacketsToLocalTrack %s has been closed", remoteTrack.Kind().String()) }()

	rtpBuf := make([]byte, 1400)
	for {
		i, _, err := remoteTrack.Read(rtpBuf)
		if err != nil {
			return err
		}
		if remoteTrack.Kind() == webrtc.RTPCodecTypeAudio {
			log.Printf("read %d bytes from from remoteTrack for %s", i, remoteTrack.Kind().String())
		}

		// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
		if _, err = localTrack.Write(rtpBuf[:i]); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			return err
		}
	}
}
