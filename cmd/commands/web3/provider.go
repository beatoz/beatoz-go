package web3

type Provider interface {
	Call(req *JSONRpcReq) (*JSONRpcResp, error)
}
