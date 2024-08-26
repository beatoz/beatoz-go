package types

type VMCallResult struct {
	UsedGas    uint64 `json:"usedGas,string,omitempty"`
	Err        string `json:"vmErr,string,omitempty"`
	ReturnData []byte `json:"returnData,omitempty"`
}
