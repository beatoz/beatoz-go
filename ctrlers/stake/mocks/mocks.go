package mocks

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/web3"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/tendermint/tendermint/libs/rand"
)

type AcctHandlerMock struct {
	wallets  []*web3.Wallet
	accounts []*ctrlertypes.Account
}

func NewAccountHandlerMock(walCnt int) *AcctHandlerMock {
	var wals []*web3.Wallet
	for i := 0; i < walCnt; i++ {
		wals = append(wals, web3.NewWallet(nil))
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

func (mock *AcctHandlerMock) ImmutableAcctCtrlerAt(i int64) (ctrlertypes.IAccountHandler, xerrors.XError) {
	return &AcctHandlerMock{}, nil
}

func (mock *AcctHandlerMock) SetAccountCommittable(account *ctrlertypes.Account, b bool) xerrors.XError {
	return nil
}

var _ ctrlertypes.IAccountHandler = (*AcctHandlerMock)(nil)
