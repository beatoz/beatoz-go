package web3

import (
	types2 "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/holiman/uint256"
)

func NewTrxTransfer(from, to types.Address, nonce, gas uint64, gasPrice, amt *uint256.Int) *types2.Trx {
	return types2.NewTrx(
		uint32(1),
		from, to,
		nonce,
		gas,
		gasPrice,
		amt,
		&types2.TrxPayloadAssetTransfer{})
}

func NewTrxStaking(from, to types.Address, nonce, gas uint64, gasPrice, amt *uint256.Int) *types2.Trx {
	return types2.NewTrx(
		uint32(1),
		from, to,
		nonce,
		gas,
		gasPrice,
		amt,
		&types2.TrxPayloadStaking{})
}

func NewTrxUnstaking(from, to types.Address, nonce, gas uint64, gasPrice *uint256.Int, txhash bytes.HexBytes) *types2.Trx {
	return types2.NewTrx(
		uint32(1),
		from, to,
		nonce,
		gas,
		gasPrice,
		uint256.NewInt(0),
		&types2.TrxPayloadUnstaking{TxHash: txhash})
}

func NewTrxWithdraw(from, to types.Address, nonce, gas uint64, gasPrice, req *uint256.Int) *types2.Trx {
	return types2.NewTrx(
		uint32(1),
		from, to,
		nonce,
		gas,
		gasPrice,
		uint256.NewInt(0),
		&types2.TrxPayloadWithdraw{ReqAmt: req})
}

func NewTrxProposal(from, to types.Address, nonce, gas uint64, gasPrice *uint256.Int, msg string, start, period, applyingHeight int64, optType int32, options ...[]byte) *types2.Trx {
	return types2.NewTrx(
		uint32(1),
		from, to,
		nonce,
		gas,
		gasPrice,
		uint256.NewInt(0),
		&types2.TrxPayloadProposal{
			Message:            msg,
			StartVotingHeight:  start,
			VotingPeriodBlocks: period,
			ApplyingHeight:     applyingHeight,
			OptType:            optType,
			Options:            options,
		})
}

func NewTrxVoting(from, to types.Address, nonce, gas uint64, gasPrice *uint256.Int, txHash bytes.HexBytes, choice int32) *types2.Trx {
	return types2.NewTrx(
		uint32(1),
		from, to,
		nonce,
		gas,
		gasPrice,
		uint256.NewInt(0),
		&types2.TrxPayloadVoting{
			TxHash: txHash,
			Choice: choice,
		})
}

func NewTrxContract(from, to types.Address, nonce, gas uint64, gasPrice, amt *uint256.Int, data bytes.HexBytes) *types2.Trx {
	return types2.NewTrx(
		uint32(1),
		from, to,
		nonce,
		gas,
		gasPrice,
		amt,
		&types2.TrxPayloadContract{
			Data: data,
		})
}

func NewTrxSetDoc(from types.Address, nonce, gas uint64, gasPrice *uint256.Int, name, docUrl string) *types2.Trx {
	return types2.NewTrx(
		uint32(1),
		from, types.ZeroAddress(),
		nonce,
		gas,
		gasPrice,
		uint256.NewInt(0),
		&types2.TrxPayloadSetDoc{name, docUrl},
	)
}
