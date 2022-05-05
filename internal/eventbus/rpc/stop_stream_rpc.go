package rpc

import "encoding/json"

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
