package web3

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"
)

type TrxResult struct {
	Hash     bytes.HexBytes         `json:"hash"`
	Height   int64                  `json:"height"`
	Index    uint32                 `json:"index"`
	TxResult abci.ResponseDeliverTx `json:"tx_result"`
	Proof    types.TxProof          `json:"proof,omitempty"`
	Tx       *ctrlertypes.Trx       `json:"tx"`
}
