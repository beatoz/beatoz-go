package types

type Provider interface {
	Call(req *JSONRpcReq) (*JSONRpcResp, error)
}
