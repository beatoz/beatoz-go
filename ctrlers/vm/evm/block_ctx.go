package evm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethcore "github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
)

var (
	gasFeeCap = uint256.NewInt(0)
	gasTipCap = uint256.NewInt(0)
)

func GetHash(h uint64) common.Hash {
	var blockHash common.Hash

	height := int64(h)
	retBlock, err := tmrpccore.Block(nil, &height)
	if err != nil {
		return blockHash // zero hash
	}
	blockHash.SetBytes(retBlock.BlockID.Hash)
	return blockHash
}

func evmBlockContext(coinbase common.Address, gasLimit int64, bn int64, tm int64) vm.BlockContext {
	return vm.BlockContext{
		CanTransfer: ethcore.CanTransfer,
		Transfer:    ethcore.Transfer,
		GetHash:     GetHash,
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
