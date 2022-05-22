package rpc

import (
	"encoding/json"

	"github.com/pion/webrtc/v3"
)

type SDPParams struct {
	webrtc.SessionDescription
	Target SignalingTarget `json:"target"`
}

// SDP RPC
type SDPRpc struct {
	jsonRpcHead
	Params SDPParams `json:"params"`
}

func NewSDPAnswerRpc(sdp *webrtc.SessionDescription, target SignalingTarget) *SDPRpc {
	return &SDPRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  SDPAnswerMethod,
		},
		Params: SDPParams{
			*sdp,
			target,
		},
	}
}

func NewSDPOfferRpc(sdp *webrtc.SessionDescription, target SignalingTarget) *SDPRpc {
	return &SDPRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  SDPOfferMethod,
		},
		Params: SDPParams{
			*sdp,
			target,
		},
	}
}

func (r SDPRpc) GetMethod() Method {
	return r.Method
}

func (r SDPRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
