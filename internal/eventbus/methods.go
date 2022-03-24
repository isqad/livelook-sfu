package eventbus

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/pion/webrtc/v3"
)

const jsonRpcVersion = "2.0"

type Method string

const (
	ICECandidateMethod  Method = "iceCandidate"
	SDPAnswerMethod     Method = "answer"
	RenegotiationMethod Method = "renogotiation"
	CreateSessionMethod Method = "create_session"
	CloseSessionMethod  Method = "close_session"
	StartStreamMethod   Method = "start_stream"
	StopStreamMethod    Method = "stop_stream"
	AddRemotePeerMethod Method = "add_remote_peer"
)

type Rpc interface {
	GetMethod() Method
	ToJSON() ([]byte, error)
}

type jsonRpcHead struct {
	Version string `json:"jsonrpc"`
	Method  Method `json:"method"`
}

type jsonRpc struct {
	jsonRpcHead
	Params map[string]interface{} `json:"params"`
}

var (
	ErrUnknownRpcType = errors.New("unknown RPC type")
	ErrMalformedRpc   = errors.New("malformed RPC")
)

func RpcFromReader(reader io.Reader) (Rpc, error) {
	rpc := &jsonRpc{}

	err := json.NewDecoder(reader).Decode(rpc)
	if err != nil {
		return nil, err
	}

	params, err := json.Marshal(rpc.Params)
	if err != nil {
		return nil, err
	}

	switch rpc.Method {
	case ICECandidateMethod:
		c := &webrtc.ICECandidateInit{}
		if err := json.Unmarshal(params, c); err != nil {
			return nil, err
		}

		return NewICECandidateRpc(c), nil
	case SDPAnswerMethod:
		sdp := &webrtc.SessionDescription{}
		if err := json.Unmarshal(params, sdp); err != nil {
			return nil, err
		}

		return NewSDPAnswerRpc(sdp), nil
	case CreateSessionMethod:
		s := &core.Session{}
		if err := json.Unmarshal(params, s); err != nil {
			return nil, err
		}

		return NewCreateSessionRpc(s), nil
	case CloseSessionMethod:
		return NewCloseSessionRpc(), nil
	case RenegotiationMethod:
		sdp := &webrtc.SessionDescription{}
		if err := json.Unmarshal(params, sdp); err != nil {
			return nil, err
		}

		return NewRenegotiationRpc(sdp), nil
	case StartStreamMethod:
		return NewStartStreamRpc(), nil
	case StopStreamMethod:
		return NewStopStreamRpc(), nil
	case AddRemotePeerMethod:
		u := make(map[string]string)
		if err := json.Unmarshal(params, &u); err != nil {
			return nil, err
		}

		userID := u["user_id"]

		return NewAddRemotePeerRpc(userID), nil
	default:
		return nil, ErrUnknownRpcType
	}
}

type CreateSessionRpc struct {
	jsonRpcHead
	Params *core.Session `json:"params"`
}

func NewCreateSessionRpc(session *core.Session) *CreateSessionRpc {
	return &CreateSessionRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  CreateSessionMethod,
		},
		Params: session,
	}
}

func (r CreateSessionRpc) GetMethod() Method {
	return r.Method
}

func (r CreateSessionRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

type CloseSessionRpc struct {
	jsonRpcHead
	Params interface{} `json:"params"`
}

func NewCloseSessionRpc() *CloseSessionRpc {
	return &CloseSessionRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  CloseSessionMethod,
		},
		Params: nil,
	}
}

func (r CloseSessionRpc) GetMethod() Method {
	return r.Method
}

func (r CloseSessionRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

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

func (r SDPRpc) GetMethod() Method {
	return r.Method
}

func (r SDPRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

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

type RenegotiationRpc struct {
	SDPRpc
}

func NewRenegotiationRpc(sdp *webrtc.SessionDescription) *RenegotiationRpc {
	return &RenegotiationRpc{
		SDPRpc: SDPRpc{
			jsonRpcHead: jsonRpcHead{
				Version: jsonRpcVersion,
				Method:  ICECandidateMethod,
			},
			Params: sdp,
		},
	}
}

func (r RenegotiationRpc) GetMethod() Method {
	return r.Method
}

func (r RenegotiationRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

type StartStreamRpc struct {
	jsonRpcHead
	Params interface{} `json:"params"`
}

func NewStartStreamRpc() *StartStreamRpc {
	return &StartStreamRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  StartStreamMethod,
		},
		Params: nil,
	}
}

func (r StartStreamRpc) GetMethod() Method {
	return r.Method
}

func (r StartStreamRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

type StopStreamRpc struct {
	jsonRpcHead
	Params interface{} `json:"params"`
}

func NewStopStreamRpc() *StopStreamRpc {
	return &StopStreamRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  StopStreamMethod,
		},
		Params: nil,
	}
}

func (r StopStreamRpc) GetMethod() Method {
	return r.Method
}

func (r StopStreamRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

type AddRemotePeerRpc struct {
	jsonRpcHead
	Params map[string]string `json:"params"`
}

func NewAddRemotePeerRpc(userID string) *AddRemotePeerRpc {
	rpc := &AddRemotePeerRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  AddRemotePeerMethod,
		},
		Params: make(map[string]string),
	}
	rpc.Params["user_id"] = userID

	return rpc
}

func (r AddRemotePeerRpc) GetMethod() Method {
	return r.Method
}

func (r AddRemotePeerRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
