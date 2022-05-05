package rpc

import "encoding/json"

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
