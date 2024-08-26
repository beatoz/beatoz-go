package types

import (
	"encoding/json"
)

type JSONRpcReq struct {
	Version string          `json:"jsonrpc"`
	Id      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type JSONRpcResp struct {
	Version string          `json:"version"`
	Id      int64           `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

func NewRequest(id int64, method string, args ...interface{}) (*JSONRpcReq, error) {
	params, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	ret := &JSONRpcReq{
		Id:     id,
		Method: method,
		Params: params,
	}
	return ret, nil
}
