package sfu

import (
	"errors"
	"io"
	"log"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

type BroadcastRequest struct {
	UserID string                    `json:"user_id"`
	Title  string                    `json:"title"`
	Sdp    webrtc.SessionDescription `json:"sdp"`
}

type BroadcastState string

const (
	BroadcastInitialState BroadcastState = "initial"
	BroadcastRunningState BroadcastState = "running"
	BroadcastStoppedState BroadcastState = "stopped"
	BroadcastErroredState BroadcastState = "errored"
)

type Broadcast struct {
	ID             string                 `db:"id" json:"id"`
	UserID         string                 `db:"user_id" json:"user_id"`
	Title          string                 `db:"title" json:"title"`
	State          BroadcastState         `db:"state" json:"state"`
	Errors         string                 `db:"errors" json:"-"`
	PeerConnection *webrtc.PeerConnection `db:"-" json:"-"`

	// all viewers will be fed via this tracks
	localVideoTrack *webrtc.TrackLocalStaticRTP
	localAudioTrack *webrtc.TrackLocalStaticRTP

	viewers map[string]*Viewer
	mutex   sync.Mutex
}

func NewBroadcast(id string, userID string, title string, sdp webrtc.SessionDescription) (*Broadcast, error) {
	peerConnection, err := webrtc.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		return nil, err
	}

	if _, err := peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}
	if _, err := peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}
	err = peerConnection.SetRemoteDescription(sdp)
	if err != nil {
		return nil, err
	}

	broadcast := &Broadcast{
		ID:             id,
		UserID:         userID,
		Title:          title,
		PeerConnection: peerConnection,
		State:          BroadcastInitialState,
		viewers:        make(map[string]*Viewer),
	}
	peerConnection.OnTrack(broadcast.onTrack)

	return broadcast, nil
}

func (b *Broadcast) Start(broadcastRepository BroadcastsDBStorer, publisher EventBusPublisher) error {
	answer, err := b.PeerConnection.CreateAnswer(nil)
	if err != nil {
		return err
	}
	gatherComplete := webrtc.GatheringCompletePromise(b.PeerConnection)
	err = b.PeerConnection.SetLocalDescription(answer)
	if err != nil {
		return err
	}
	<-gatherComplete
	// TODO: ICE Trickle
	log.Println("ICE candidates gathered!")

	answerJSONRpc, err := NewSdpJSONRpc(b.PeerConnection.LocalDescription(), "answer")
	if err != nil {
		return err
	}
	// Send answer
	if err := publisher.Publish("messages:"+b.UserID, answerJSONRpc); err != nil {
		return err
	}

	b.State = BroadcastRunningState

	return broadcastRepository.Save(b)
}

func (b *Broadcast) Stop(broadcastRepository BroadcastsDBStorer) error {
	err := broadcastRepository.SetStopped(b)
	if err != nil {
		return err
	}

	return b.ClosePeerConnection()
}

func (b *Broadcast) ClosePeerConnection() error {
	return b.PeerConnection.Close()
}

func (b *Broadcast) addViewer(viewer *Viewer) error {
	rtpVideoSender, err := viewer.PeerConnection.AddTrack(b.localVideoTrack)
	if err != nil {
		return err
	}
	rtpAudioSender, err := viewer.PeerConnection.AddTrack(b.localAudioTrack)
	if err != nil {
		return err
	}

	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpVideoSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpAudioSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	b.mutex.Lock()
	b.viewers[viewer.ID] = viewer
	b.mutex.Unlock()

	return nil
}

func (b *Broadcast) onTrack(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	log.Printf("ON TRACK! %+v", remoteTrack.Kind())

	// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
	go func() {
		ticker := time.NewTicker(time.Second * 3)
		for range ticker.C {
			errSend := b.PeerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(remoteTrack.SSRC())}})
			if errSend != nil {
				log.Println(errSend)
			}
		}
	}()

	// Read incoming RTCP packets
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := receiver.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	if remoteTrack.Kind() == webrtc.RTPCodecTypeVideo {
		localVideoTrack, err := webrtc.NewTrackLocalStaticRTP(
			remoteTrack.Codec().RTPCodecCapability,
			remoteTrack.Kind().String(),
			"pion",
		)
		if err != nil {
			log.Println(err)
			return
		}
		b.mutex.Lock()
		b.localVideoTrack = localVideoTrack
		b.mutex.Unlock()

		rtpBuf := make([]byte, 1400)
		for {
			i, _, readErr := remoteTrack.Read(rtpBuf)
			if readErr != nil {
				log.Println(readErr)
				return
			}
			// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
			if _, err = b.localVideoTrack.Write(rtpBuf[:i]); err != nil && errors.Is(err, io.ErrClosedPipe) {
				log.Println(err)
				return
			}
		}
	} else if remoteTrack.Kind() == webrtc.RTPCodecTypeAudio {
		localAudioTrack, err := webrtc.NewTrackLocalStaticRTP(
			remoteTrack.Codec().RTPCodecCapability,
			remoteTrack.Kind().String(),
			"pion",
		)
		if err != nil {
			log.Println(err)
			return
		}
		b.mutex.Lock()
		b.localAudioTrack = localAudioTrack
		b.mutex.Unlock()

		rtpBuf := make([]byte, 1400)
		for {
			i, _, readErr := remoteTrack.Read(rtpBuf)
			if readErr != nil {
				log.Println(readErr)
				return
			}

			// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
			if _, err = b.localAudioTrack.Write(rtpBuf[:i]); err != nil && errors.Is(err, io.ErrClosedPipe) {
				log.Println(err)
				return
			}
		}
	}
}
