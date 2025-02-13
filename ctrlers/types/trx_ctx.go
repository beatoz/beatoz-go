package types

import (
	"github.com/beatoz/beatoz-go/types"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

type TrxContext struct {
	*BlockContext

	TxHash bytes2.HexBytes
	Tx     *Trx
	TxIdx  int
	Exec   bool

	SenderPubKey []byte
	Sender       *Account
	Receiver     *Account
	GasUsed      uint64
	RetData      []byte
	Events       []abcitypes.Event

	TrxGovHandler   ITrxHandler
	TrxAcctHandler  ITrxHandler
	TrxStakeHandler ITrxHandler
	TrxEVMHandler   ITrxHandler

	Callback func(*TrxContext, xerrors.XError)
}

type ITrxHandler interface {
	ValidateTrx(*TrxContext) xerrors.XError
	ExecuteTrx(*TrxContext) xerrors.XError
}

type NewTrxContextCb func(*TrxContext) xerrors.XError

func NewTrxContext(txbz []byte, bctx *BlockContext, exec bool, cbfns ...NewTrxContextCb) (*TrxContext, xerrors.XError) {
	tx := &Trx{}
	if xerr := tx.Decode(txbz); xerr != nil {
		return nil, xerr
	}
	if xerr := tx.Validate(); xerr != nil {
		return nil, xerr
	}

	txctx := &TrxContext{
		BlockContext: bctx,
		TxIdx:        bctx.GetTxsCnt(),
		Tx:           tx,
		TxHash:       tmtypes.Tx(txbz).Hash(),
		Exec:         exec,
		GasUsed:      0,
	}
	for _, fn := range cbfns {
		if err := fn(txctx); err != nil {
			return nil, err
		}
	}

	//
	// The following can be performed in parallel.
	{
		if tx.Gas < bctx.GovHandler.MinTrxGas() {
			return nil, xerrors.ErrInvalidGas.Wrapf("too small gas. the minimum gas of tx is %v", bctx.GovHandler.MinTrxGas())
		} else if tx.Gas > bctx.GetTrxGasLimit() {
			return nil, xerrors.ErrInvalidGas.Wrapf("too much gas. the gas limit of tx is %v", bctx.TxGasLimit)
		}
		if tx.GasPrice.Cmp(txctx.GovHandler.GasPrice()) != 0 {
			return nil, xerrors.ErrInvalidGasPrice
		}

		_, pubKeyBytes, xerr := VerifyTrxRLP(tx, txctx.ChainID)
		if xerr != nil {
			return nil, xerr
		}
		txctx.SenderPubKey = pubKeyBytes
	}
	//

	txctx.Sender = txctx.AcctHandler.FindAccount(tx.From, txctx.Exec)
	if txctx.Sender == nil {
		return nil, xerrors.ErrNotFoundAccount.Wrapf("sender address: %v", tx.From)
	}
	// RG-91:  Also find the account object with the destination address 0x0.
	toAddr := tx.To
	if toAddr == nil {
		// `toAddr` may be `nil` when the tx type is `TRX_CONTRACT`.
		toAddr = types.ZeroAddress()
	}
	txctx.Receiver = txctx.AcctHandler.FindOrNewAccount(toAddr, txctx.Exec)
	if txctx.Receiver == nil {
		return nil, xerrors.ErrNotFoundAccount.Wrapf("receiver address: %v", tx.To)
	}

	bctx.AddTxsCnt(1)
	return txctx, nil
}

// `UseGas` should not be used in `EVMCtrler`
func (ctx *TrxContext) UseGas(gas uint64) xerrors.XError {
	if xerr := ctx.BlockContext.UseGas(gas); xerr != nil {
		return xerr
	}
	ctx.GasUsed = gas
	return nil
}

func (ctx *TrxContext) FeeUsed() *uint256.Int {
	return GasToFee(ctx.GasUsed, ctx.Tx.GasPrice)
}

func (ctx *TrxContext) FeeWanted() *uint256.Int {
	return GasToFee(ctx.Tx.Gas, ctx.Tx.GasPrice)
}

func (ctx *TrxContext) IsHandledByEVM() bool {
	// the tx whose type is `TRX_CONTRACT`.
	// the tx whose type is `TRX_TRANSFER` and `Receiver` is contract. (for processing fallback)
	return ctx.Tx.GetType() == TRX_CONTRACT || (ctx.Tx.GetType() == TRX_TRANSFER && ctx.Receiver.Code != nil)
}
