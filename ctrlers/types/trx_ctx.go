package types

import (
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/types"
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

	txctx := &TrxContext{
		Tx:        tx,
		TxHash:    types.Tx(txbz).Hash(),
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
	// begin of code from commonValidation0
	{
		if tx.GasPrice.Sign() < 0 || tx.GasPrice.Cmp(txctx.GovHandler.GasPrice()) != 0 {
			return nil, xerrors.ErrInvalidGasPrice
		}

		feeAmt := new(uint256.Int).Mul(tx.GasPrice, uint256.NewInt(tx.Gas))
		if feeAmt.Cmp(txctx.GovHandler.MinTrxFee()) < 0 {
			return nil, xerrors.ErrInvalidGas.Wrapf("too small gas(fee)")
		}

		_, pubKeyBytes, xerr := VerifyTrxRLP(tx, txctx.ChainID)
		if xerr != nil {
			return nil, xerr
		}
		txctx.SenderPubKey = pubKeyBytes
	}
	// end of code from commonValidation0
	//

	txctx.Sender = txctx.AcctHandler.FindAccount(tx.From, txctx.Exec)
	if txctx.Sender == nil {
		return nil, xerrors.ErrNotFoundAccount.Wrapf("sender address: %v", tx.From)
	}
	// RG-91:  Also find the account object with the destination address 0x0.
	txctx.Receiver = txctx.AcctHandler.FindOrNewAccount(tx.To, txctx.Exec)
	if txctx.Receiver == nil {
		return nil, xerrors.ErrNotFoundAccount.Wrapf("receiver address: %v", tx.To)
	}
	return txctx, nil
}
