package web3

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/holiman/uint256"
)

func NewTrxTransfer(from, to types.Address, nonce, gas int64, gasPrice, amt *uint256.Int) *ctrlertypes.Trx {
	return ctrlertypes.NewTrx(
		1,
		from, to,
		nonce,
		gas,
		gasPrice,
		amt,
		&ctrlertypes.TrxPayloadAssetTransfer{})
}

func NewTrxStaking(from, to types.Address, nonce, gas int64, gasPrice, amt *uint256.Int) *ctrlertypes.Trx {
	return ctrlertypes.NewTrx(
		1,
		from, to,
		nonce,
		gas,
		gasPrice,
		amt,
		&ctrlertypes.TrxPayloadStaking{})
}

func NewTrxUnstaking(from, to types.Address, nonce, gas int64, gasPrice *uint256.Int, txhash bytes.HexBytes) *ctrlertypes.Trx {
	return ctrlertypes.NewTrx(
		1,
		from, to,
		nonce,
		gas,
		gasPrice,
		uint256.NewInt(0),
		&ctrlertypes.TrxPayloadUnstaking{TxHash: txhash})
}

func NewTrxWithdraw(from, to types.Address, nonce, gas int64, gasPrice, req *uint256.Int) *ctrlertypes.Trx {
	return ctrlertypes.NewTrx(
		1,
		from, to,
		nonce,
		gas,
		gasPrice,
		uint256.NewInt(0),
		&ctrlertypes.TrxPayloadWithdraw{ReqAmt: req})
}

func NewTrxProposal(from, to types.Address, nonce, gas int64, gasPrice *uint256.Int, msg string, start, period, applyingHeight int64, optType int32, options ...[]byte) *ctrlertypes.Trx {
	return ctrlertypes.NewTrx(
		1,
		from, to,
		nonce,
		gas,
		gasPrice,
		uint256.NewInt(0),
		&ctrlertypes.TrxPayloadProposal{
			Message:            msg,
			StartVotingHeight:  start,
			VotingPeriodBlocks: period,
			ApplyingHeight:     applyingHeight,
			OptType:            optType,
			Options:            options,
		})
}

func NewTrxVoting(from, to types.Address, nonce, gas int64, gasPrice *uint256.Int, txHash bytes.HexBytes, choice int32) *ctrlertypes.Trx {
	return ctrlertypes.NewTrx(
		1,
		from, to,
		nonce,
		gas,
		gasPrice,
		uint256.NewInt(0),
		&ctrlertypes.TrxPayloadVoting{
			TxHash: txHash,
			Choice: choice,
		})
}

func NewTrxContract(from, to types.Address, nonce, gas int64, gasPrice, amt *uint256.Int, data bytes.HexBytes) *ctrlertypes.Trx {
	return ctrlertypes.NewTrx(
		1,
		from, to,
		nonce,
		gas,
		gasPrice,
		amt,
		&ctrlertypes.TrxPayloadContract{
			Data: data,
		})
}

func NewTrxSetDoc(from types.Address, nonce, gas int64, gasPrice *uint256.Int, name, docUrl string) *ctrlertypes.Trx {
	return ctrlertypes.NewTrx(
		1,
		from, types.ZeroAddress(),
		nonce,
		gas,
		gasPrice,
		uint256.NewInt(0),
		&ctrlertypes.TrxPayloadSetDoc{name, docUrl},
	)
}
