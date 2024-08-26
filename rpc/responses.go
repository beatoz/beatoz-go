package rpc

import (
	"encoding/json"
	"github.com/beatoz/beatoz-go/types/bytes"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/proto/tendermint/crypto"
)

type QueryResult struct {
	abcitypes.ResponseQuery
}

func (qr *QueryResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Code      uint32           `json:"code,omitempty"`
		Log       string           `json:"log,omitempty"`
		Info      string           `json:"info,omitempty"`
		Index     int64            `json:"index,omitempty"`
		Key       bytes.HexBytes   `json:"key,omitempty"`
		Value     json.RawMessage  `json:"value,omitempty"`
		ProofOps  *crypto.ProofOps `json:"proof_ops,omitempty"`
		Height    int64            `json:"height,omitempty"`
		Codespace string           `json:"codespace,omitempty"`
	}{
		Code:      qr.Code,
		Log:       qr.Log,
		Info:      qr.Info,
		Index:     qr.Index,
		Key:       qr.Key,
		Value:     qr.Value,
		ProofOps:  qr.ProofOps,
		Height:    qr.Height,
		Codespace: qr.Codespace,
	})
}

func (qr *QueryResult) UnmarshalJSON(bz []byte) error {
	tmpQr := &struct {
		Code      uint32           `json:"code,omitempty"`
		Log       string           `json:"log,omitempty"`
		Info      string           `json:"info,omitempty"`
		Index     int64            `json:"index,omitempty"`
		Key       bytes.HexBytes   `json:"key,omitempty"`
		Value     json.RawMessage  `json:"value,omitempty"`
		ProofOps  *crypto.ProofOps `json:"proof_ops,omitempty"`
		Height    int64            `json:"height,omitempty"`
		Codespace string           `json:"codespace,omitempty"`
	}{}
	if err := json.Unmarshal(bz, tmpQr); err != nil {
		return err
	}

	qr.Code = tmpQr.Code
	qr.Log = tmpQr.Log
	qr.Info = tmpQr.Info
	qr.Index = tmpQr.Index
	qr.Key = tmpQr.Key
	qr.Value = tmpQr.Value
	qr.ProofOps = tmpQr.ProofOps
	qr.Height = tmpQr.Height
	qr.Codespace = tmpQr.Codespace
	return nil
}
