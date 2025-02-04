package types

import (
	"github.com/beatoz/beatoz-go/libs"
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
	MinGas    uint64
	MaxGas    uint64
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

	GovHandler   IGovHandler
	AcctHandler  IAccountHandler
	StakeHandler IStakeHandler
	ChainID      string

	Callback func(*TrxContext, xerrors.XError)
}

type ITrxHandler interface {
	ValidateTrx(*TrxContext) xerrors.XError
	ExecuteTrx(*TrxContext) xerrors.XError
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
	// The following can be performed in parallel.
	{
		if txctx.MinGas == 0 {
			txctx.MinGas = txctx.GovHandler.MinTrxGas()
		}
		if txctx.MaxGas == 0 {
			txctx.MaxGas = txctx.GovHandler.MaxTrxGas()
		}
		if tx.Gas < txctx.MinGas {
			return nil, xerrors.ErrInvalidGas.Wrapf("too small gas")
		} else if tx.Gas > txctx.MaxGas {
			return nil, xerrors.ErrInvalidGas.Wrapf("too much gas")
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
	return txctx, nil
}

func AdjustMaxGasPerTrx(txcnt int, cap uint64) uint64 {
	txcnt = libs.MaxInt(txcnt, 1)
	return cap / uint64(txcnt)
}
