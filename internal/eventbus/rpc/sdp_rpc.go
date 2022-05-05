package rpc

import (
	"encoding/json"

	"github.com/pion/webrtc/v3"
)

// SDP RPC
type SDPRpc struct {
	jsonRpcHead
	Params *webrtc.SessionDescription `json:"params"`
}

func NewSDPAnswerRpc(sdp *webrtc.SessionDescription) *SDPRpc {
	return &SDPRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  SDPAnswerMethod,
		},
		Params: sdp,
	}
}

func NewSDPOfferRpc(sdp *webrtc.SessionDescription) *SDPRpc {
	return &SDPRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  SDPOfferMethod,
		},
		Params: sdp,
	}
}

func (r SDPRpc) GetMethod() Method {
	return r.Method
}

func (r SDPRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
