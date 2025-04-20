package types

import (
	"github.com/beatoz/beatoz-go/types"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

type TrxContext struct {
	Height    int64
	BlockTime int64
	TxHash    bytes2.HexBytes
	Tx        *Trx
	TxIdx     int
	Exec      bool

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

	GovParams    IGovParams
	AcctHandler  IAccountHandler
	StakeHandler IStakeHandler
	ChainID      string

	ValidateResult interface{}
	Callback       func(*TrxContext, xerrors.XError)
}

type NewTrxContextCb func(*TrxContext) xerrors.XError

func NewTrxContext(txbz []byte, height, btime int64, exec bool, cbfns ...NewTrxContextCb) (*TrxContext, xerrors.XError) {
	tx := &Trx{}
	if xerr := tx.Decode(txbz); xerr != nil {
		return nil, xerr
	}
	if xerr := tx.Validate(); xerr != nil {
		return nil, xerr
	}

	txctx := &TrxContext{
		Tx:        tx,
		TxHash:    tmtypes.Tx(txbz).Hash(),
		Height:    height,
		BlockTime: btime,
		Exec:      exec,
		GasUsed:   0,
	}
	for _, fn := range cbfns {
		if err := fn(txctx); err != nil {
			return nil, err
		}
	}

	//
	// validation gas and signature.
	if tx.Gas < txctx.GovParams.MinTrxGas() {
		return nil, xerrors.ErrInvalidGas.Wrapf("the tx has too small gas (min: %v)", txctx.GovParams.MinTrxGas())
	} else if tx.Gas > txctx.GovParams.MaxTrxGas() {
		return nil, xerrors.ErrInvalidGas.Wrapf("the tx has too much gas (max: %d)", txctx.GovParams.MaxTrxGas())
	}

	if tx.GasPrice.Cmp(txctx.GovParams.GasPrice()) != 0 {
		return nil, xerrors.ErrInvalidGasPrice
	}

	_, pubKeyBytes, xerr := VerifyTrxRLP(tx, txctx.ChainID)
	if xerr != nil {
		return nil, xerr
	}
	txctx.SenderPubKey = pubKeyBytes
	//
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

	return txctx, nil
}

func (ctx *TrxContext) IsHandledByEVM() bool {
	b := ctx.Tx.GetType() == TRX_CONTRACT || (ctx.Tx.GetType() == TRX_TRANSFER && ctx.Receiver.Code != nil)
	return b
}
