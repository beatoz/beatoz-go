package types

import (
	"errors"
	"github.com/beatoz/beatoz-go/types"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

type TrxContext struct {
	*BlockContext

	Tx     *Trx
	TxIdx  int
	TxHash bytes2.HexBytes
	Exec   bool

	SenderPubKey []byte
	Sender       *Account
	Receiver     *Account
	Payer        *Account
	GasUsed      int64
	RetData      []byte
	Events       []abcitypes.Event

	ValidateResult interface{}
	Callback       func(*TrxContext, xerrors.XError)
}

type NewTrxContextCb func(*TrxContext) xerrors.XError

func NewTrxContext(txbz []byte, bctx *BlockContext, exec bool) (*TrxContext, xerrors.XError) {
	tx := &Trx{}
	if xerr := tx.Decode(txbz); xerr != nil {
		return nil, xerr
	}
	if xerr := tx.Validate(); xerr != nil {
		return nil, xerr
	}

	txctx := &TrxContext{
		BlockContext: bctx,
		Tx:           tx,
		TxIdx:        bctx.TxsCnt(),
		TxHash:       tmtypes.Tx(txbz).Hash(),
		Exec:         exec,
		GasUsed:      0,
	}

	//
	// validation gas.
	if tx.Gas < txctx.BlockContext.GovHandler.MinTrxGas() {
		return nil, xerrors.ErrInvalidGas.Wrapf("the tx has too small gas (min: %v)", txctx.GovHandler.MinTrxGas())
	} else if tx.Gas > txctx.BlockContext.GovHandler.MaxTrxGas() {
		return nil, xerrors.ErrInvalidGas.Wrapf("the tx has too much gas (max: %d)", txctx.GovHandler.MaxTrxGas())
	}

	if tx.GasPrice.Cmp(txctx.BlockContext.GovHandler.GasPrice()) != 0 {
		return nil, xerrors.ErrInvalidGasPrice
	}

	//
	// verify signature.
	_, pubKeyBytes, xerr := VerifyTrxRLP(tx, txctx.BlockContext.ChainID())
	if xerr != nil {
		return nil, xerr
	}
	txctx.SenderPubKey = pubKeyBytes

	//
	// verify payer's signature.
	var payerAddr types.Address
	if tx.PayerSig != nil {
		payerAddr, _, xerr = VerifyPayerTrxRLP(tx, txctx.BlockContext.ChainID())
		if xerr != nil {
			return nil, xerr.Wrap(errors.New("payer signature is invalid"))
		}
	}

	//
	//

	txctx.Sender = txctx.BlockContext.AcctHandler.FindAccount(tx.From, txctx.Exec)
	if txctx.Sender == nil {
		return nil, xerrors.ErrNotFoundAccount.Wrapf("sender address: %v", tx.From)
	}

	// RG-91: Find the account object with the destination address 0x0.
	toAddr := txctx.Tx.To
	if toAddr == nil {
		// `toAddr` may be `nil` when the tx type is `TRX_CONTRACT`.
		toAddr = types.ZeroAddress()
	}

	txctx.Receiver = txctx.BlockContext.AcctHandler.FindOrNewAccount(toAddr, txctx.Exec)
	if txctx.Receiver == nil {
		return nil, xerrors.ErrNotFoundAccount.Wrapf("receiver address: %v", toAddr)
	}

	if payerAddr != nil {
		txctx.Payer = txctx.BlockContext.AcctHandler.FindAccount(payerAddr, txctx.Exec)
		if txctx.Payer == nil {
			return nil, xerrors.ErrNotFoundAccount.Wrapf("payer address: %v", payerAddr)
		}
	} else {
		txctx.Payer = txctx.Sender
	}

	return txctx, nil
}

func (ctx *TrxContext) IsHandledByEVM() bool {
	b := ctx.Tx.GetType() == TRX_CONTRACT || (ctx.Tx.GetType() == TRX_TRANSFER && ctx.Receiver.Code != nil)
	return b
}
