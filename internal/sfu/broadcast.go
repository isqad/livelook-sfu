package sfu

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

type BroadcastRequest struct {
	UserID string                    `json:"user_id"`
	Title  string                    `json:"title"`
	Sdp    webrtc.SessionDescription `json:"sdp"`
}

type Broadcast struct {
	ID             string                    `db:"id"`
	UserID         string                    `db:"user_id"`
	Title          string                    `db:"title"`
	Sdp            webrtc.SessionDescription `db:"-"`
	PeerConnection *webrtc.PeerConnection    `db:"-"`

	// all viewers will be fed via this tracks
	localVideoTrack *webrtc.TrackLocalStaticRTP
	localAudioTrack *webrtc.TrackLocalStaticRTP
	mutex           sync.Mutex
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

	broadcast := &Broadcast{
		ID:             id,
		UserID:         userID,
		Title:          title,
		Sdp:            sdp,
		PeerConnection: peerConnection,
	}
	peerConnection.OnTrack(broadcast.onTrack)

	return broadcast, nil
}

func (b *Broadcast) Start(db *sqlx.DB, rdb *redis.Client) error {
	err := b.PeerConnection.SetRemoteDescription(b.Sdp)
	if err != nil {
		return err
	}
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
	if err := rdb.Publish(context.Background(), "messages:"+b.UserID, answerJSONRpc).Err(); err != nil {
		return err
	}

	_, err = db.Exec(
		`INSERT INTO broadcasts (id, user_id, title, created_at) VALUES ($1, $2, $3, NOW())`,
		b.ID, b.UserID, b.Title,
	)
	if err != nil {
		return err
	}

	return nil
}

func (b *Broadcast) Stop(db *sqlx.DB) error {
	_, err := db.Exec(
		`DELETE FROM broadcasts WHERE id = $1`,
		b.ID,
	)
	if err != nil {
		return err
	}

	return b.ClosePeerConnection()
}

func (b *Broadcast) ClosePeerConnection() error {
	return b.PeerConnection.Close()
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
