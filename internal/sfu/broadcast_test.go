package sfu

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pion/webrtc/v3"
)

func TestNewBroadcast(t *testing.T) {
	b, err := NewBroadcast("some-id", "user-id", "Some title", "sdp-here")
	assert.Nil(t, err)

	assert.Equal(t, "some-id", b.ID)
	assert.Equal(t, "user-id", b.UserID)
	assert.Equal(t, "Some title", b.Title)
	assert.Equal(t, "sdp-here", b.Sdp)
	assert.NotNil(t, b.PeerConnection)
	assert.IsType(t, &webrtc.PeerConnection{}, b.PeerConnection)
}
