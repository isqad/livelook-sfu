package rpc

import (
	"encoding/json"

	"github.com/pion/webrtc/v3"
)

// ICE candidate RPC
type ICECandidateRpc struct {
	jsonRpcHead
	Params *webrtc.ICECandidateInit `json:"params"`
}

func NewICECandidateRpc(candidate *webrtc.ICECandidateInit) *ICECandidateRpc {
	return &ICECandidateRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  ICECandidateMethod,
		},
		Params: candidate,
	}
}

func (r ICECandidateRpc) GetMethod() Method {
	return r.Method
}

func (r ICECandidateRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
