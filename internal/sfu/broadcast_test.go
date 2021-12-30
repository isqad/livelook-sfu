package sfu

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pion/webrtc/v3"
)

const (
	minimalTest = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
a=msid-semantic: WMS
m=application 47299 DTLS/SCTP 5000
c=IN IP4 192.168.20.129
a=candidate:1966762134 1 udp 2122260223 192.168.20.129 47299 typ host generation 0
a=candidate:1966762134 1 udp 2122262783 2001:db8::1 47199 typ host generation 0
a=candidate:211962667 1 udp 2122194687 10.0.3.1 40864 typ host generation 0
a=candidate:1002017894 1 tcp 1518280447 192.168.20.129 0 typ host tcptype active generation 0
a=candidate:1109506011 1 tcp 1518214911 10.0.3.1 0 typ host tcptype active generation 0
a=ice-ufrag:1/MvHwjAyVf27aLu
a=ice-pwd:3dBU7cFOBl120v33cynDvN1E
a=ice-options:google-ice
a=fingerprint:sha-256 75:74:5A:A6:A4:E5:52:F4:A7:67:4C:01:C7:EE:91:3F:21:3D:A2:E3:53:7B:6F:30:86:F2:30:AA:65:FB:04:24
a=setup:actpass
a=mid:data
a=sctpmap:5000 webrtc-datachannel 1024
`
)

var (
	minimalOfferSdp = webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  minimalTest,
	}
)

type broadcastDBSaverMock struct {
	saved   bool
	updated bool
}

func (bs *broadcastDBSaverMock) Save(b *Broadcast) error {
	bs.saved = true

	return nil
}

func (bs *broadcastDBSaverMock) SetStopped(b *Broadcast) error {
	bs.updated = true

	return nil
}

type eventBusPublisherMock struct {
	published bool
}

func (p *eventBusPublisherMock) Publish(channel string, message interface{}) error {
	p.published = true

	return nil
}

func TestNewBroadcast(t *testing.T) {
	b, err := NewBroadcast("some-id", "user-id", "Some title", minimalOfferSdp)
	assert.Nil(t, err)

	assert.Equal(t, "some-id", b.ID)
	assert.Equal(t, "user-id", b.UserID)
	assert.Equal(t, "Some title", b.Title)
	assert.NotNil(t, b.PeerConnection)
	assert.IsType(t, &webrtc.PeerConnection{}, b.PeerConnection)
	assert.IsType(t, map[string]*Viewer{}, b.viewers)
	assert.Equal(t, BroadcastInitialState, b.State)
}

func TestStart(t *testing.T) {
	b, err := NewBroadcast("some-id", "user-id", "Some title", minimalOfferSdp)
	assert.Nil(t, err)

	dbSaveMock := &broadcastDBSaverMock{}
	publisher := &eventBusPublisherMock{}
	b.Start(dbSaveMock, publisher)

	assert.True(t, dbSaveMock.saved)
	assert.True(t, publisher.published)
	assert.Equal(t, BroadcastRunningState, b.State)
}

func TestStop(t *testing.T) {
	//var connState webrtc.PeerConnectionState

	b, err := NewBroadcast("some-id", "user-id", "Some title", minimalOfferSdp)
	assert.Nil(t, err)

	//b.PeerConnection.OnConnectionStateChange(func(cs webrtc.PeerConnectionState) {
	//	connState = cs
	//})

	dbSaveMock := &broadcastDBSaverMock{}
	publisher := &eventBusPublisherMock{}
	b.Start(dbSaveMock, publisher)
	b.Stop(dbSaveMock)

	assert.True(t, dbSaveMock.updated)
	//assert.Equal(t, webrtc.PeerConnectionStateClosed, connState)
}
