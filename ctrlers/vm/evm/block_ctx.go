package evm

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"math/big"
)

var (
	gasFeeCap = uint256.NewInt(0)
	gasTipCap = uint256.NewInt(0)
)

// CanTransfer checks whether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas into account to make the transfer valid.
func CanTransfer(db vm.StateDB, addr common.Address, amount *big.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db vm.StateDB, sender, recipient common.Address, amount *big.Int) {
	db.SubBalance(sender, amount)
	db.AddBalance(recipient, amount)
}

func GetHash(h uint64) common.Hash {
	return common.Hash{}
}

func evmBlockContext(coinbase common.Address, bn int64, tm int64, gasLimit uint64) vm.BlockContext {
	return vm.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     GetHash,
		Coinbase:    coinbase,
		BlockNumber: big.NewInt(bn),
		Time:        big.NewInt(tm),
		Difficulty:  big.NewInt(1),
		BaseFee:     big.NewInt(0),
		GasLimit:    gasLimit, // issue #44
	}
}

func evmMessage(_from common.Address, _to *common.Address, nonce, gasLimit uint64, gasPrice, amt *uint256.Int, data []byte, isFake bool) types.Message {
	return types.NewMessage(
		_from,
		_to,
		nonce,
		amt.ToBig(),
		gasLimit,         // gas limit
		gasPrice.ToBig(), // gas price
		gasFeeCap.ToBig(),
		gasTipCap.ToBig(),
		data,
		nil,
		isFake,
	)
}
