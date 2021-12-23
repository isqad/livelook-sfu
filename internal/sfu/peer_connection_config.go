package sfu

import (
	"github.com/pion/webrtc/v3"
)

var (
	peerConnectionConfig webrtc.Configuration = webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
)
