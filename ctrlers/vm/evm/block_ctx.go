package evm

import (
	"github.com/ethereum/go-ethereum/common"
	ethcore "github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"math/big"
)

var (
	gasFeeCap = uint256.NewInt(0)
	gasTipCap = uint256.NewInt(0)
)

func GetHash(h uint64) common.Hash {
	return common.Hash{}
}

func evmBlockContext(coinbase common.Address, bn int64, tm int64, gasLimit int64) vm.BlockContext {
	return vm.BlockContext{
		CanTransfer: ethcore.CanTransfer,
		Transfer:    ethcore.Transfer,
		GetHash:     GetHash,
		Coinbase:    coinbase,
		BlockNumber: big.NewInt(bn),
		Time:        big.NewInt(tm),
		Difficulty:  big.NewInt(1),
		BaseFee:     big.NewInt(0),
		GasLimit:    uint64(gasLimit), // issue #44
	}
}

func evmMessage(_from common.Address, _to *common.Address, nonce, gasLimit int64, gasPrice, amt *uint256.Int, data []byte, isFake bool) types.Message {
	return types.NewMessage(
		_from,
		_to,
		uint64(nonce),
		amt.ToBig(),
		uint64(gasLimit), // gas limit
		gasPrice.ToBig(), // gas price
		gasFeeCap.ToBig(),
		gasTipCap.ToBig(),
		data,
		nil,
		isFake,
	)
}
