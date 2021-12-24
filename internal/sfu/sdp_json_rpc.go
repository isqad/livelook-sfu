package sfu

import (
	"encoding/json"

	"github.com/pion/webrtc/v3"
)

type sdpJSONRpc struct {
	JSONRPC string                    `json:"jsonrpc"`
	Method  string                    `json:"method"`
	Params  webrtc.SessionDescription `json:"params"`
}

func NewSdpJSONRpc(sdp webrtc.SessionDescription, sdpType string) ([]byte, error) {
	rpc := sdpJSONRpc{
		JSONRPC: "2.0",
		Method:  sdpType,
		Params:  sdp,
	}

	return json.Marshal(rpc)
}
