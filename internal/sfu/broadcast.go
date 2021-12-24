package sfu

import (
	"context"
	"log"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
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

	// all viewers will be fed via this channel (track)
	localTrackChan chan *webrtc.TrackLocalStaticRTP
}

func NewBroadcast(id string, userID string, title string, sdp webrtc.SessionDescription) (*Broadcast, error) {
	peerConnection, err := webrtc.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		return nil, err
	}
	peerConnection.OnTrack(onTrack)

	broadcast := &Broadcast{
		ID:             id,
		UserID:         userID,
		Title:          title,
		Sdp:            sdp,
		PeerConnection: peerConnection,
		localTrackChan: make(chan *webrtc.TrackLocalStaticRTP),
	}

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
	log.Println("ICE candidates gathered!")

	answerJSONRpc, err := NewSdpJSONRpc(answer, "answer")
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

func onTrack(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {}
