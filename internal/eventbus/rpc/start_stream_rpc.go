package rpc

import "encoding/json"

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
