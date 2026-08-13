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
	if xerr := createTrxCaches(ctx.BlockContext, ctx.Exec); xerr != nil {
		return xerr
	}

	if xerr := validateTrx(ctx); xerr != nil {
		clearTrxCaches(ctx.BlockContext, ctx.Exec)
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
	sender := ctx.AcctHandler.FindAccount(tx.From, ctx.Exec)
	if sender == nil {
		return xerrors.ErrNotFoundAccount.Wrapf("sender address: %v", tx.From)
	}

	payer := sender
	if tx.Payer != nil && bytes.Compare(tx.From, tx.Payer) != 0 {
		payer = ctx.AcctHandler.FindAccount(tx.Payer, ctx.Exec)
		if payer == nil {
			return xerrors.ErrNotFoundAccount.Wrapf("payer address: %v", tx.Payer)
		}
	}

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
	if bytes.Compare(sender.Address, payer.Address) != 0 {
		if xerr := payer.CheckBalance(feeAmt); xerr != nil {
			return xerr
		}
		if xerr := sender.CheckBalance(tx.Amount); xerr != nil {
			return xerr
		}
	} else {
		needAmt := new(uint256.Int).Add(feeAmt, tx.Amount)
		if xerr := sender.CheckBalance(needAmt); xerr != nil {
			return xerr
		}
	}

	if xerr := sender.CheckNonce(tx.Nonce); xerr != nil {
		return xerr.Wrap(fmt.Errorf("ledger: %v, tx:%v, address: %v, txhash: %X", sender.GetNonce(), tx.Nonce, sender.Address, ctx.TxHash))
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
	executeErr := executeTrx(ctx)
	if executeErr != nil {
		clearTrxCaches(ctx.BlockContext, ctx.Exec)
		if !ctx.Exec {
			return executeErr
		}
		if xerr := createTrxCaches(ctx.BlockContext, ctx.Exec); xerr != nil {
			return executeErr.Wrap(xerr)
		}
	} else if xerr := writeTrxCaches(ctx.BlockContext, ctx.Exec); xerr != nil {
		clearTrxCaches(ctx.BlockContext, ctx.Exec)
		return xerr
	}

	postErr := postRunTrx(ctx)
	if postErr == nil {
		postErr = writeTrxCaches(ctx.BlockContext, ctx.Exec)
	}
	clearTrxCaches(ctx.BlockContext, ctx.Exec)

	if executeErr == nil {
		return postErr
	}
	if postErr != nil {
		return executeErr.Wrap(postErr)
	}
	return executeErr
}

func createTrxCaches(bctx *ctrlertypes.BlockContext, exec bool) xerrors.XError {
	if bctx == nil {
		return xerrors.NewOrdinary("block context is nil")
	}
	if bctx.AcctHandler == nil {
		return xerrors.NewOrdinary("account handler is nil")
	}
	if bctx.GovHandler == nil {
		return xerrors.NewOrdinary("governance handler is nil")
	}
	if bctx.SupplyHandler == nil {
		return xerrors.NewOrdinary("supply handler is nil")
	}
	if bctx.VPowerHandler == nil {
		return xerrors.NewOrdinary("voting power handler is nil")
	}

	handlers := []ctrlertypes.ITrxCacheHandler{
		bctx.VPowerHandler,
		bctx.SupplyHandler,
		bctx.GovHandler,
		bctx.AcctHandler,
	}
	for idx, handler := range handlers {
		if xerr := handler.CreateCache(exec); xerr != nil {
			for i := idx - 1; i >= 0; i-- {
				_ = handlers[i].ClearCache(exec)
			}
			return xerr
		}
	}
	return nil
}

func writeTrxCaches(bctx *ctrlertypes.BlockContext, exec bool) xerrors.XError {
	if xerr := bctx.AcctHandler.WriteCache(exec); xerr != nil {
		return xerr
	}
	if xerr := bctx.GovHandler.WriteCache(exec); xerr != nil {
		return xerr
	}
	if xerr := bctx.SupplyHandler.WriteCache(exec); xerr != nil {
		return xerr
	}
	if xerr := bctx.VPowerHandler.WriteCache(exec); xerr != nil {
		return xerr
	}
	return nil
}

func clearTrxCaches(bctx *ctrlertypes.BlockContext, exec bool) {
	_ = bctx.AcctHandler.ClearCache(exec)
	_ = bctx.GovHandler.ClearCache(exec)
	_ = bctx.SupplyHandler.ClearCache(exec)
	_ = bctx.VPowerHandler.ClearCache(exec)
}

func executeTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	switch ctx.Tx.GetType() {
	case ctrlertypes.TRX_CONTRACT:
		if xerr := ctx.EVMHandler.ExecuteTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_PROPOSAL, ctrlertypes.TRX_VOTING:
		if xerr := ctx.GovHandler.ExecuteTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_TRANSFER, ctrlertypes.TRX_SETDOC:
		if ctx.IsHandledByEVM() {
			if xerr := ctx.EVMHandler.ExecuteTrx(ctx); xerr != nil {
				return xerr
			}
		} else if xerr := ctx.AcctHandler.ExecuteTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_WITHDRAW:
		if xerr := ctx.SupplyHandler.ExecuteTrx(ctx); xerr != nil {
			return xerr
		}
	case ctrlertypes.TRX_STAKING, ctrlertypes.TRX_UNSTAKING:
		if xerr := ctx.VPowerHandler.ExecuteTrx(ctx); xerr != nil {
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
	sender := ctx.Sender()
	payer := sender
	if ctx.Tx.Payer != nil && bytes.Compare(sender.Address, ctx.Tx.Payer) != 0 {
		payer = ctx.Payer()
	}
	fee := types.GasToFee(ctx.GasUsed, ctx.Tx.GasPrice)
	if xerr := payer.SubBalance(fee); xerr != nil {
		return xerr
	}

	// processing nonce
	sender.AddNonce()

	// update sender account
	if xerr := ctx.AcctHandler.SetAccount(sender, ctx.Exec); xerr != nil {
		return xerr
	}
	if bytes.Compare(sender.Address, payer.Address) != 0 {
		if xerr := ctx.AcctHandler.SetAccount(payer, ctx.Exec); xerr != nil {
			return xerr
		}
	}
	return nil
}
