package evm

import (
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethcore "github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
)

var (
	gasFeeCap = uint256.NewInt(0)
	gasTipCap = uint256.NewInt(0)
)

func zeroBlockHashProvider(uint64) common.Hash {
	return common.Hash{}
}

func newBlockHashProvider(currentHeight int64, lookup func(int64) (common.Hash, bool)) vm.GetHashFunc {
	return func(height uint64) common.Hash {
		if height > math.MaxInt64 || int64(height) >= currentHeight || lookup == nil {
			return common.Hash{}
		}

		hash, ok := lookup(int64(height))
		if !ok {
			return common.Hash{}
		}
		return hash
	}
}

func evmBlockContext(coinbase common.Address, gasLimit int64, bn int64, tm int64, getHash vm.GetHashFunc) vm.BlockContext {
	if getHash == nil {
		getHash = zeroBlockHashProvider
	}

	return vm.BlockContext{
		CanTransfer: ethcore.CanTransfer,
		Transfer:    ethcore.Transfer,
		GetHash:     getHash,
		Coinbase:    coinbase,
		GasLimit:    uint64(gasLimit), // issue #44
		BlockNumber: big.NewInt(bn),
		Time:        uint64(tm),
		Difficulty:  big.NewInt(1),
		BaseFee:     big.NewInt(0),
		BlobBaseFee: big.NewInt(0),
		Random:      &common.Hash{}, // newbie of shanghai
	}
}

func evmMessage(_from common.Address, _to *common.Address, nonce, gasLimit int64, gasPrice, amt *uint256.Int, data []byte, isFake bool) *ethcore.Message {
	return &ethcore.Message{
		To:                _to,
		From:              _from,
		Nonce:             uint64(nonce),
		Value:             amt.ToBig(),
		GasLimit:          uint64(gasLimit),
		GasPrice:          gasPrice.ToBig(),
		GasFeeCap:         gasFeeCap.ToBig(),
		GasTipCap:         gasTipCap.ToBig(),
		Data:              data,
		AccessList:        nil,
		SkipAccountChecks: isFake,
		BlobGasFeeCap:     big.NewInt(0),
		BlobHashes:        nil,
	}
}
