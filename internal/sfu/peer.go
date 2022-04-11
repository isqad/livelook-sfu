package sfu

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

const (
	rtcpPLIInterval            = time.Second * 3
	dtlsRetransmissionInterval = 100 * time.Millisecond
	mtu                        = 1400
)

var (
	errConnectionNotInitialized = errors.New("connection is not initialized")
)

type peer struct {
	userID           core.UserSessionID
	streamingAllowed bool

	connection *webrtc.PeerConnection

	iceCandidatesLock sync.RWMutex
	iceCandidates     []*webrtc.ICECandidateInit // TODO: rename to pendingICECandidates

	videoTransceiver *webrtc.RTPTransceiver
	audioTransceiver *webrtc.RTPTransceiver

	localVideoTrack *webrtc.TrackLocalStaticRTP
	localAudioTrack *webrtc.TrackLocalStaticRTP

	remotePeersLock sync.RWMutex
	remotePeers     []*peer

	lock sync.Mutex

	closeChan        chan struct{}
	closed           chan struct{}
	stopTracks       chan struct{}
	stopped          chan struct{}
	stopParentTracks chan struct{}
}

func (p *peer) addRemotePeer(remotePeer *peer) error {
	err := remotePeer.videoTransceiver.Sender().ReplaceTrack(p.localVideoTrack)
	if err != nil {
		return err
	}
	err = remotePeer.audioTransceiver.Sender().ReplaceTrack(p.localAudioTrack)
	if err != nil {
		return err
	}
	return nil
}

func (p *peer) establishPeerConnection(eventsPublisher eventbus.Publisher) error {
	api, err := buildAPI()
	if err != nil {
		return err
	}

	peerConnection, err := api.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		return err
	}

	peerConnection.OnICECandidate(p.onICECandidate(eventsPublisher))
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
	peerConnection.OnTrack(p.onTrack)

	log.Println("add video transciever")
	p.videoTransceiver, err = peerConnection.AddTransceiverFromKind(
		webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendrecv},
	)
	if err != nil {
		return err
	}

	log.Println("add audio transciever")
	p.audioTransceiver, err = peerConnection.AddTransceiverFromKind(
		webrtc.RTPCodecTypeAudio,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendrecv},
	)
	if err != nil {
		return err
	}

	p.connection = peerConnection

	return nil
}

func buildAPI() (*webrtc.API, error) {
	// me, err := rtc.createMediaEngine([]*config.CodecSpec{})
	// if err != nil {
	// 	return nil, err
	// }

	// se := webrtc.SettingEngine{}
	// // se.DisableMediaEngineCopy(true)
	// se.DisableSRTPReplayProtection(true)
	// se.DisableSRTCPReplayProtection(true)
	// se.SetDTLSRetransmissionInterval(dtlsRetransmissionInterval)
	// se.SetReceiveMTU(mtu)

	// api := webrtc.NewAPI(
	// 	webrtc.WithMediaEngine(me),
	// 	webrtc.WithSettingEngine(se),
	// )
	return nil, nil
}

func (p *peer) setRemoteDescription(sdp webrtc.SessionDescription) error {
	log.Println("setRemoteDescription")
	if p.connection == nil {
		return errConnectionNotInitialized
	}

	if err := p.connection.SetRemoteDescription(sdp); err != nil {
		return err
	}

	p.iceCandidatesLock.Lock()
	defer p.iceCandidatesLock.Unlock()

	if len(p.iceCandidates) == 0 {
		log.Println("setRemoteDescription: no pending ICE candidates, return")
		return nil
	}

	log.Printf("setRemoteDescription: %d pending ICE candidates, add it all to PC", len(p.iceCandidates))
	for _, c := range p.iceCandidates {
		iceCandidate := *c

		if err := p.connection.AddICECandidate(iceCandidate); err != nil {
			return err
		}
	}

	p.clearCandidates()

	return nil
}

func (p *peer) addICECandidate(candidate *webrtc.ICECandidateInit) error {
	p.iceCandidatesLock.Lock()
	defer p.iceCandidatesLock.Unlock()

	if p.connection == nil {
		return errConnectionNotInitialized
	}

	p.iceCandidates = append(p.iceCandidates, candidate)
	log.Printf("addICECandidate: %d pending ICE candidates", len(p.iceCandidates))

	if p.connection.CurrentRemoteDescription() == nil {
		return nil
	}

	for _, c := range p.iceCandidates {
		iceCandidate := *c

		if err := p.connection.AddICECandidate(iceCandidate); err != nil {
			return err
		}
	}

	p.clearCandidates()

	return nil
}

func (p *peer) clearCandidates() {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.iceCandidates = []*webrtc.ICECandidateInit{}
}

func (p *peer) createAnswer() (*eventbus.SDPRpc, error) {
	if p.connection == nil {
		return nil, errConnectionNotInitialized
	}

	log.Println("createAnswer")

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

func (p *peer) listenAndAccept() {
	for {
		select {
		case <-p.closeChan:
			log.Printf("closing peer %s...\n", p.userID)

			// p.stopTracks <- struct{}{}
			p.streamingAllowed = false
			p.clearCandidates()
			p.clearRemotePeers()
			if p.connection == nil {
				log.Printf("%v", errConnectionNotInitialized)
			}
			if err := p.connection.Close(); err != nil {
				log.Printf("close peer error %v\n", err)
			}
			//<-p.stopped

			p.closed <- struct{}{}
			return
		case <-p.stopParentTracks:
			log.Printf("accepted stopParentTracks signal for peer %s...", p.userID)
		}
	}
}

func (p *peer) clearRemotePeers() {
	p.remotePeersLock.RLock()
	for _, remotePeer := range p.remotePeers {
		remotePeer.stopParentTracks <- struct{}{}
	}
	p.remotePeersLock.RUnlock()

	p.lock.Lock()
	p.remotePeers = []*peer{}
	p.lock.Unlock()
}

func (p *peer) close() <-chan struct{} {
	p.closeChan <- struct{}{}

	return p.closed
}

func (p *peer) onICECandidate(eventsPublisher eventbus.Publisher) func(*webrtc.ICECandidate) {
	return func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		candidateInit := candidate.ToJSON()
		rpc := eventbus.NewICECandidateRpc(&candidateInit)

		if err := eventsPublisher.PublishClient(p.userID, rpc); err != nil {
			log.Printf("onICECandidate: error %v", err)
			return
		}
	}
}

func (p *peer) onTrack(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	log.Printf("onTrack: %s", remoteTrack.Kind().String())

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
			// TODO: stop the goroutine
			// if !p.streamingAllowed {
			// 	continue
			// }

			if err := p.connection.WriteRTCP(
				[]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(remoteTrack.SSRC())}},
			); err != nil {
				fmt.Printf("onTrack send PLI error: %v\n", err)
				return
			}

			// if err := p.connection.WriteRTCP(
			// 	[]rtcp.Packet{&rtcp.ReceiverEstimatedMaximumBitrate{Bitrate: 1500000, SenderSSRC: uint32(remoteTrack.SSRC())}},
			// ); err != nil {
			// 	fmt.Printf("onTrack send REMB error: %v\n", err)
			// 	return
			// }
		}
	}()

	// Create a local track, all our SFU clients will be fed via this track
	localVideoTrack, err := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, "video", "pion")
	if err != nil {
		return err
	}

	p.lock.Lock()
	p.localVideoTrack = localVideoTrack
	p.lock.Unlock()

	return p.forwardPacketsToLocalTrack(remoteTrack, localVideoTrack)
}

func (p *peer) createLocalAudioTrackForwarding(remoteTrack *webrtc.TrackRemote) error {
	log.Println("createLocalAudioTrackForwarding")
	// Create a local track, all our SFU clients will be fed via this track
	log.Printf("remote audiocodec capability: %+v", remoteTrack.Codec().RTPCodecCapability)
	localAudioTrack, err := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, "audio", "pion")
	if err != nil {
		return err
	}

	p.lock.Lock()
	p.localAudioTrack = localAudioTrack
	p.lock.Unlock()

	return p.forwardPacketsToLocalTrack(remoteTrack, localAudioTrack)
}

func (p *peer) forwardPacketsToLocalTrack(remoteTrack *webrtc.TrackRemote, localTrack *webrtc.TrackLocalStaticRTP) error {
	log.Printf("forwardPacketsToLocalTrack: %s", remoteTrack.Kind().String())

	for {
		// if !p.streamingAllowed {
		// 	time.Sleep(100 * time.Millisecond)
		// 	continue
		// }
		p, _, err := remoteTrack.ReadRTP()
		if err != nil {
			return err
		}

		// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
		if err = localTrack.WriteRTP(p); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			return err
		}
	}
}
