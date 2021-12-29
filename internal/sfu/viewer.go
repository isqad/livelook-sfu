package sfu

import (
	"context"
	"log"

	"github.com/go-redis/redis/v8"
	"github.com/pion/webrtc/v3"
)

type ViewerRequest struct {
	BroadcastID string                    `json:"broadcast_id"`
	UserID      string                    `json:"user_id"`
	Sdp         webrtc.SessionDescription `json:"sdp"`
}

type Viewer struct {
	ID             string                 `db:"id"`
	UserID         string                 `db:"user_id"`
	PeerConnection *webrtc.PeerConnection `db:"-"`
}

func NewViewer(id string, userID string, sdp webrtc.SessionDescription) (*Viewer, error) {
	peerConnection, err := webrtc.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		return nil, err
	}
	err = peerConnection.SetRemoteDescription(sdp)
	if err != nil {
		return nil, err
	}

	viewer := &Viewer{
		ID:             id,
		UserID:         userID,
		PeerConnection: peerConnection,
	}
	return viewer, nil
}

func (v *Viewer) Start(rdb *redis.Client) error {
	answer, err := v.PeerConnection.CreateAnswer(nil)
	if err != nil {
		return err
	}
	gatherComplete := webrtc.GatheringCompletePromise(v.PeerConnection)
	err = v.PeerConnection.SetLocalDescription(answer)
	if err != nil {
		return err
	}
	<-gatherComplete
	// TODO: ICE Trickle
	log.Println("ICE candidates gathered!")

	answerJSONRpc, err := NewSdpJSONRpc(v.PeerConnection.LocalDescription(), "answer")
	if err != nil {
		return err
	}

	// Send answer
	// TODO: incapsulate
	if err := rdb.Publish(context.Background(), "messages:"+v.UserID, answerJSONRpc).Err(); err != nil {
		return err
	}
	return nil
}
