package rpc

import (
	"encoding/json"

	"github.com/pion/webrtc/v3"
)

type ICECandidateParams struct {
	webrtc.ICECandidateInit
	Target SignalingTarget `json:"target"`
}

// ICE candidate RPC
type ICECandidateRpc struct {
	jsonRpcHead
	Params ICECandidateParams `json:"params"`
}

func NewICECandidateRpc(candidate webrtc.ICECandidateInit, target SignalingTarget) *ICECandidateRpc {
	return &ICECandidateRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  ICECandidateMethod,
		},
		Params: ICECandidateParams{
			ICECandidateInit: candidate,
			Target:           target,
		},
	}
}

func (r ICECandidateRpc) GetMethod() Method {
	return r.Method
}

func (r ICECandidateRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
