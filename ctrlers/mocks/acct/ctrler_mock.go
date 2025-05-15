package acct

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/rand"
)

type AcctHandlerMock struct {
	wallets  []*web3.Wallet
	accounts []*ctrlertypes.Account // has no private key
}

func NewAcctHandlerMock(walCnt int) *AcctHandlerMock {
	var wals []*web3.Wallet
	for i := 0; i < walCnt; i++ {
		w := web3.NewWallet(nil)
		wals = append(wals, w)
	}
	return &AcctHandlerMock{wallets: wals}
}

func (mock *AcctHandlerMock) GetAllWallets() []*web3.Wallet {
	return mock.wallets
}

func (mock *AcctHandlerMock) WalletLen() int {
	return len(mock.wallets)
}

func (mock *AcctHandlerMock) RandWallet() *web3.Wallet {
	idx := rand.Intn(len(mock.wallets))
	return mock.wallets[idx]
}

func (mock *AcctHandlerMock) GetWallet(idx int) *web3.Wallet {
	if idx >= len(mock.wallets) {
		return nil
	}
	return mock.wallets[idx]
}

func (mock *AcctHandlerMock) FindWallet(addr types.Address) *web3.Wallet {
	for _, w := range mock.wallets {
		if addr.Compare(w.Address()) == 0 {
			return w
		}
	}
	return nil
}

func (mock *AcctHandlerMock) Iterate(cb func(int, *web3.Wallet) bool) {
	for i, w := range mock.wallets {
		if !cb(i, w) {
			break
		}
	}
}

//
// IAccountHandler interfaces

func (mock *AcctHandlerMock) FindOrNewAccount(addr types.Address, exec bool) *ctrlertypes.Account {
	if acct := mock.FindAccount(addr, exec); acct != nil {
		return acct
	}

	acct := ctrlertypes.NewAccount(addr)
	mock.accounts = append(mock.accounts, acct)
	return acct
}

func (mock *AcctHandlerMock) FindAccount(addr types.Address, exec bool) *ctrlertypes.Account {
	if w := mock.FindWallet(addr); w != nil {
		return w.GetAccount()
	}

	for _, acct := range mock.accounts {
		if addr.Compare(acct.Address) == 0 {
			return acct
		}
	}
	return nil
}
func (mock *AcctHandlerMock) Transfer(from, to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	if sender := mock.FindAccount(from, exec); sender == nil {
		return xerrors.ErrNotFoundAccount
	} else if receiver := mock.FindAccount(to, exec); receiver == nil {
		return xerrors.ErrNotFoundAccount
	} else if xerr := sender.SubBalance(amt); xerr != nil {
		return xerr
	} else if xerr := receiver.AddBalance(amt); xerr != nil {
		return xerr
	}
	return nil
}
func (mock *AcctHandlerMock) Reward(to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	if receiver := mock.FindAccount(to, exec); receiver == nil {
		return xerrors.ErrNotFoundAccount
	} else if xerr := receiver.AddBalance(amt); xerr != nil {
		return xerr
	}
	return nil
}

func (mock *AcctHandlerMock) AddBalance(addr types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	if receiver := mock.FindAccount(addr, exec); receiver == nil {
		return xerrors.ErrNotFoundAccount
	} else if xerr := receiver.AddBalance(amt); xerr != nil {
		return xerr
	}
	return nil
}

func (mock *AcctHandlerMock) SubBalance(addr types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	if receiver := mock.FindAccount(addr, exec); receiver == nil {
		return xerrors.ErrNotFoundAccount
	} else if xerr := receiver.SubBalance(amt); xerr != nil {
		return xerr
	}
	return nil
}

func (mock *AcctHandlerMock) SetBalance(addr types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	receiver := mock.FindOrNewAccount(addr, exec)
	receiver.SetBalance(amt)
	return nil
}

func (mock *AcctHandlerMock) SimuAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
	return &AcctHandlerMock{}, nil
}
func (mock *AcctHandlerMock) SetAccount(acct *ctrlertypes.Account, b bool) xerrors.XError {
	return nil
}

func (mock *AcctHandlerMock) BeginBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (mock *AcctHandlerMock) EndBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (mock *AcctHandlerMock) Commit() ([]byte, int64, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (mock *AcctHandlerMock) ValidateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (mock *AcctHandlerMock) ExecuteTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	_ = ctx.Sender.AddBalance(ctx.Tx.Amount)
	_ = ctx.Receiver.SubBalance(ctx.Tx.Amount)
	return nil
}

var _ ctrlertypes.IAccountHandler = (*AcctHandlerMock)(nil)
