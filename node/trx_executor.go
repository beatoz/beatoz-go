package node

import (
	"fmt"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/tendermint/tendermint/libs/log"
)

type TrxExecutor struct {
	*TrxPreparer
	logger log.Logger
}

func NewTrxExecutor(logger log.Logger) *TrxExecutor {
	return &TrxExecutor{
		TrxPreparer: newTrxPreparer(),
		logger:      logger,
	}
}

func (txe *TrxExecutor) ExecuteSync(ctx *ctrlertypes.TrxContext) xerrors.XError {
	var xerr xerrors.XError

	xerr = validateTrx(ctx)
	if xerr != nil {
		return xerr
	}

	return runTrx(ctx)
}

func commonValidation(ctx *ctrlertypes.TrxContext) xerrors.XError {

	//
	// This validation must be performed sequentially
	// after the previous tx was executed.
	// (after the account balance and nonce have been updated by the previous tx execution.)
	//
	tx := ctx.Tx

	remainedBlockGas := ctx.BlockContext.GetBlockGasRemained()
	if remainedBlockGas <= 0 || remainedBlockGas < tx.Gas {
		return xerrors.ErrInvalidGas.Wrapf("blockGasLimit(%v), used(%v), remained(%v), txGasWanted(%v)",
			ctx.BlockContext.GetBlockGasLimit(),
			ctx.BlockContext.GetBlockGasUsed(),
			remainedBlockGas,
			ctx.Tx.Gas,
		)
	}

	feeAmt := new(uint256.Int).Mul(tx.GasPrice, uint256.NewInt(uint64(tx.Gas)))
	if bytes.Compare(ctx.Sender.Address, ctx.Payer.Address) != 0 {
		if xerr := ctx.Payer.CheckBalance(feeAmt); xerr != nil {
			return xerr
		}
		if xerr := ctx.Sender.CheckBalance(tx.Amount); xerr != nil {
			return xerr
		}
	} else {
		needAmt := new(uint256.Int).Add(feeAmt, tx.Amount)
		if xerr := ctx.Sender.CheckBalance(needAmt); xerr != nil {
			return xerr
		}
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
		if xerr := ctx.GovHandler.ValidateTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_TRANSFER, ctrlertypes.TRX_SETDOC:
		if xerr := ctx.AcctHandler.ValidateTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_WITHDRAW:
		if xerr := ctx.SupplyHandler.ValidateTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_STAKING, ctrlertypes.TRX_UNSTAKING:
		if xerr := ctx.VPowerHandler.ValidateTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_CONTRACT:
		if xerr := ctx.EVMHandler.ValidateTrx(ctx); xerr != nil {
			return xerr
		}
	default:
		return xerrors.ErrUnknownTrxType
	}

	return nil
}

func runTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	var xerr xerrors.XError
	defer func() {
		if ctx.Exec || xerr == nil {
			_xerr0 := postRunTrx(ctx)
			if xerr != nil {
				xerr = xerr.Wrap(_xerr0)
			} else {
				xerr = _xerr0
			}
		}
	}()

	switch ctx.Tx.GetType() {
	case ctrlertypes.TRX_CONTRACT:
		if xerr = ctx.EVMHandler.ExecuteTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_PROPOSAL, ctrlertypes.TRX_VOTING:
		if xerr = ctx.GovHandler.ExecuteTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_TRANSFER, ctrlertypes.TRX_SETDOC:
		if ctx.IsHandledByEVM() {
			if xerr = ctx.EVMHandler.ExecuteTrx(ctx); xerr != nil {
				return xerr
			}
		} else if xerr = ctx.AcctHandler.ExecuteTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_WITHDRAW:
		if xerr = ctx.SupplyHandler.ExecuteTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_STAKING, ctrlertypes.TRX_UNSTAKING:
		if xerr = ctx.VPowerHandler.ExecuteTrx(ctx); xerr != nil {
			return xerr
		}
	default:
		return xerrors.ErrUnknownTrxType
	}

	return nil
}

func postRunTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	// In case of EVM Tx, ctx.GasUsed has been already computed by EVMCtrler.
	// In case of EVM Tx, the block gas pool has been already handled by EVMCtrler.
	if !ctx.IsHandledByEVM() {
		ctx.GasUsed = ctx.Tx.Gas
		_ = ctx.BlockContext.UseBlockGas(ctx.Tx.Gas)
	}
	// processing fee = gas * gasPrice
	fee := types.GasToFee(ctx.GasUsed, ctx.Tx.GasPrice)
	if xerr := ctx.Payer.SubBalance(fee); xerr != nil {
		return xerr
	}

	// processing nonce
	ctx.Sender.AddNonce()

	// update sender account
	if xerr := ctx.AcctHandler.SetAccount(ctx.Sender, ctx.Exec); xerr != nil {
		return xerr
	}
	if bytes.Compare(ctx.Sender.Address, ctx.Payer.Address) != 0 {
		if xerr := ctx.AcctHandler.SetAccount(ctx.Payer, ctx.Exec); xerr != nil {
			return xerr
		}
	}
	return nil
}
