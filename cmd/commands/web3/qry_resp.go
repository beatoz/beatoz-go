package web3

import (
	btztypes "github.com/beatoz/beatoz-go/types"
	btzbytes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/tendermint/tendermint/types"
)

type RespQueryStake struct {
	From        btztypes.Address  `json:"owner"`
	To          btztypes.Address  `json:"to"`
	TxHash      btzbytes.HexBytes `json:"txhash"`
	StartHeight int64             `json:"startHeight,string"`
	//RefundHeight int64           `json:"refundHeight,string"`
	Power int64 `json:"power,string"`
}

type RespQueryDelegatee struct {
	Addr                btztypes.Address   `json:"address"`
	PubKey              btzbytes.HexBytes  `json:"pubKey"`
	SelfPower           int64              `json:"selfPower,string"`
	TotalPower          int64              `json:"totalPower,string"`
	SlashedPower        int64              `json:"slashedPower,string"`
	Delegators          []btztypes.Address `json:"delegators"`
	NotSignedBlockCount int64              `json:"notSingedBlockCount,string"`
	// DEPRECATED: only for backward compatibility
	Stakes []*RespQueryStake `json:"stakes,omitempty"`
	// DEPRECATED: only for backward compatibility
	NotSignedHeights interface{} `json:"notSignedBlocks,omitempty"`
}

type RespQueryReward struct {
	Address   types.Address `json:"address,omitempty"`
	Issued    string        `json:"issued,omitempty"`
	Withdrawn string        `json:"withdrawn,omitempty"`
	Slashed   string        `json:"slashed,omitempty"`
	Cumulated string        `json:"cumulated,omitempty"`
	Height    int64         `json:"height,omitempty"`
}
