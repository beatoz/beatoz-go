package evm

import (
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcoretypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"math/big"
)

type vmMessage struct {
	to         *ethcommon.Address
	from       ethcommon.Address
	nonce      uint64
	amount     *big.Int
	gasLimit   uint64
	gasPrice   *big.Int
	gasFeeCap  *big.Int
	gasTipCap  *big.Int
	data       []byte
	accessList ethcoretypes.AccessList
	isFake     bool

	// added for txfee sponsor
	sponsor *ethcommon.Address
}

func (m vmMessage) From() ethcommon.Address             { return m.from }
func (m vmMessage) To() *ethcommon.Address              { return m.to }
func (m vmMessage) GasPrice() *big.Int                  { return m.gasPrice }
func (m vmMessage) GasFeeCap() *big.Int                 { return m.gasFeeCap }
func (m vmMessage) GasTipCap() *big.Int                 { return m.gasTipCap }
func (m vmMessage) Value() *big.Int                     { return m.amount }
func (m vmMessage) Gas() uint64                         { return m.gasLimit }
func (m vmMessage) Nonce() uint64                       { return m.nonce }
func (m vmMessage) Data() []byte                        { return m.data }
func (m vmMessage) AccessList() ethcoretypes.AccessList { return m.accessList }
func (m vmMessage) IsFake() bool                        { return m.isFake }
func (m vmMessage) Sponsor() *ethcommon.Address         { return m.sponsor }

func evmMessage(from ethcommon.Address, to *ethcommon.Address, nonce, gasLimit int64, gasPrice, amt *uint256.Int, data []byte, isFake bool) vmMessage {
	return vmMessage{
		to:         to,
		from:       from,
		nonce:      uint64(nonce),
		amount:     amt.ToBig(),
		gasLimit:   uint64(gasLimit), // gas limit
		gasPrice:   gasPrice.ToBig(),
		gasFeeCap:  gasFeeCap.ToBig(),
		gasTipCap:  gasTipCap.ToBig(),
		data:       data,
		accessList: nil,
		isFake:     isFake,
		sponsor:    nil,
	}
}
