package node

import (
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/tendermint/tendermint/libs/log"
)

type TrxExecutor struct {
	logger log.Logger
}

func NewTrxExecutor(logger log.Logger) *TrxExecutor {
	return &TrxExecutor{
		logger: logger,
	}
}

func (txe *TrxExecutor) ExecuteSync(ctx *ctrlertypes.TrxContext, bctx *ctrlertypes.BlockContext) xerrors.XError {
	xerr := validateTrx(ctx)
	if xerr != nil {
		return xerr
	}
	xerr = runTrx(ctx, bctx)
	if xerr != nil {
		return xerr
	}
	return nil
}

func commonValidation(ctx *ctrlertypes.TrxContext) xerrors.XError {

	//
	// This validation must be performed sequentially
	// after the previous tx was executed.
	// (after the account balance and nonce have been updated by the previous tx execution.)
	//
	tx := ctx.Tx

	feeAmt := new(uint256.Int).Mul(tx.GasPrice, uint256.NewInt(uint64(tx.Gas)))
	needAmt := new(uint256.Int).Add(feeAmt, tx.Amount)
	if xerr := ctx.Sender.CheckBalance(needAmt); xerr != nil {
		return xerr
	}
	if xerr := ctx.Sender.CheckNonce(tx.Nonce); xerr != nil {
		return xerr.Wrap(fmt.Errorf("ledger: %v, tx:%v, address: %v, txhash: %X", ctx.Sender.GetNonce(), tx.Nonce, ctx.Sender.Address, ctx.TxHash))
	}
	return nil
}

func validateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {

	//
	// tx validation
	if xerr := commonValidation(ctx); xerr != nil {
		return xerr
	}

	switch ctx.Tx.GetType() {
	case ctrlertypes.TRX_PROPOSAL, ctrlertypes.TRX_VOTING:
		if xerr := ctx.TrxGovHandler.ValidateTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_TRANSFER, ctrlertypes.TRX_SETDOC:
		if xerr := ctx.TrxAcctHandler.ValidateTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_STAKING, ctrlertypes.TRX_UNSTAKING, ctrlertypes.TRX_WITHDRAW:
		if xerr := ctx.TrxStakeHandler.ValidateTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_CONTRACT:
		if xerr := ctx.TrxEVMHandler.ValidateTrx(ctx); xerr != nil {
			return xerr
		}
	default:
		return xerrors.ErrUnknownTrxType
	}

	return nil
}

func runTrx(ctx *ctrlertypes.TrxContext, bctx *ctrlertypes.BlockContext) xerrors.XError {
	var xerr xerrors.XError

	// consuming gas
	// Note that the gas for txs executed by `EVMCtrler` is handled directly by `EVMCtrler`.
	if bctx != nil && !ctx.IsHandledByEVM() {
		if xerr = bctx.UseBlockGas(ctx.Tx.Gas); xerr != nil {
			return xerr.Wrapf("blockGasLimit(%v), blockGasUsed(%v), txGasWanted(%v)", bctx.GetBlockGasLimit(), bctx.GetBlockGasUsed(), ctx.Tx.Gas)
		}
	}

	defer func() {
		if xerr != nil && bctx != nil && !ctx.IsHandledByEVM() {
			bctx.RefundBlockGas(ctx.Tx.Gas)
		}
	}()

	switch ctx.Tx.GetType() {
	case ctrlertypes.TRX_CONTRACT:
		if xerr = ctx.TrxEVMHandler.ExecuteTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_PROPOSAL, ctrlertypes.TRX_VOTING:
		if xerr = ctx.TrxGovHandler.ExecuteTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_TRANSFER, ctrlertypes.TRX_SETDOC:
		if ctx.IsHandledByEVM() {
			if xerr = ctx.TrxEVMHandler.ExecuteTrx(ctx); xerr != nil {
				return xerr
			}
		} else if xerr = ctx.TrxAcctHandler.ExecuteTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_STAKING, ctrlertypes.TRX_UNSTAKING, ctrlertypes.TRX_WITHDRAW:
		if xerr = ctx.TrxStakeHandler.ExecuteTrx(ctx); xerr != nil {
			return xerr
		}
	default:
		return xerrors.ErrUnknownTrxType
	}

	if xerr = postRunTrx(ctx); xerr != nil {
		return xerr
	}

	return nil
}

func postRunTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	if ctx.Tx.GetType() == ctrlertypes.TRX_CONTRACT ||
		(ctx.Tx.GetType() == ctrlertypes.TRX_TRANSFER && ctx.Receiver.Code != nil) {
		// 1. If the tx type is `TRX_CONTRACT`,
		//    the gas & nonce have already been processed in `EVMCtrler`.
		// 2. If the tx is `TRX_TRANSFER` type and the receiver is a contract,
		//    it is executed by `EVMCtrler` to process the fallback feature.
		//    In this case too, tha gas & nonce have already been also processed in `EVMCtrler`.
		return nil
	}

	// processing fee = gas * gasPrice
	fee := new(uint256.Int).Mul(ctx.Tx.GasPrice, uint256.NewInt(uint64(ctx.Tx.Gas)))
	if xerr := ctx.Sender.SubBalance(fee); xerr != nil {
		return xerr
	}

	// processing nonce
	ctx.Sender.AddNonce()

	// update sender account
	if xerr := ctx.AcctHandler.SetAccount(ctx.Sender, ctx.Exec); xerr != nil {
		return xerr
	}

	// set used gas
	ctx.GasUsed = ctx.Tx.Gas
	return nil
}
