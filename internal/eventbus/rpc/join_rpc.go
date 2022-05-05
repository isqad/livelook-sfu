package rpc

import "encoding/json"

type JoinRpc struct {
	jsonRpcHead
	Params map[string]string `json:"params"`
}

func NewJoinRpc() *JoinRpc {
	rpc := &JoinRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  JoinMethod,
		},
		Params: nil,
	}

	return rpc
}

func (r JoinRpc) GetMethod() Method {
	return r.Method
}

func (r JoinRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
