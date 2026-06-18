package acct

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/rand"
)

type AcctHandlerMock struct {
	wallets     []*web3.Wallet
	accounts    []*ctrlertypes.Account // has no private key
	snapshots   []acctHandlerSnapshot
	snapshotCnt int
	revertCnt   int
}

type acctHandlerSnapshot struct {
	wallets  []*web3.Wallet
	accounts []*ctrlertypes.Account
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

func (mock *AcctHandlerMock) AddWallet(w *web3.Wallet) {
	mock.wallets = append(mock.wallets, w)
}

func (mock *AcctHandlerMock) AddAccount(acct *ctrlertypes.Account) {
	mock.accounts = append(mock.accounts, acct)
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

	// no wallet, only account
	acct := ctrlertypes.NewAccount(addr)
	mock.accounts = append(mock.accounts, acct)
	return acct
}

func (mock *AcctHandlerMock) FindAccount(addr types.Address, _ bool) *ctrlertypes.Account {
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
	if receiver := mock.FindOrNewAccount(addr, exec); receiver == nil {
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
func (mock *AcctHandlerMock) SetAccount(acct *ctrlertypes.Account, _ bool) xerrors.XError {
	if w := mock.FindWallet(acct.Address); w != nil {
		copyAccount(w.GetAccount(), acct)
		return nil
	}
	for i, old := range mock.accounts {
		if acct.Address.Compare(old.Address) == 0 {
			mock.accounts[i] = acct.Clone()
			return nil
		}
	}
	mock.accounts = append(mock.accounts, acct.Clone())
	return nil
}

func (mock *AcctHandlerMock) Snapshot(bool) v1.Snapshot {
	mock.snapshotCnt++
	mock.snapshots = append(mock.snapshots, acctHandlerSnapshot{
		wallets:  cloneWallets(mock.wallets),
		accounts: cloneAccounts(mock.accounts),
	})
	return v1.Snapshot{}
}

func (mock *AcctHandlerMock) RevertToSnapshot(v1.Snapshot, bool) xerrors.XError {
	if len(mock.snapshots) == 0 {
		return xerrors.ErrInvalidSnapshot
	}
	mock.revertCnt++
	snap := mock.snapshots[len(mock.snapshots)-1]
	mock.snapshots = mock.snapshots[:len(mock.snapshots)-1]

	if len(mock.wallets) > len(snap.wallets) {
		mock.wallets = mock.wallets[:len(snap.wallets)]
	}
	for i, snapWallet := range snap.wallets {
		if i < len(mock.wallets) {
			copyAccount(mock.wallets[i].GetAccount(), snapWallet.GetAccount())
		} else {
			mock.wallets = append(mock.wallets, snapWallet.Clone())
		}
	}
	mock.accounts = cloneAccounts(snap.accounts)
	return nil
}

func (mock *AcctHandlerMock) SnapshotCount() int {
	return mock.snapshotCnt
}

func (mock *AcctHandlerMock) RevertCount() int {
	return mock.revertCnt
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
	return nil
}

func (mock *AcctHandlerMock) ExecuteTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	if xerr := ctx.Sender.SubBalance(ctx.Tx.Amount); xerr != nil {
		return xerr
	}
	if xerr := ctx.Receiver.AddBalance(ctx.Tx.Amount); xerr != nil {
		return xerr
	}
	return nil
}

func cloneWallets(src []*web3.Wallet) []*web3.Wallet {
	ret := make([]*web3.Wallet, len(src))
	for i, w := range src {
		ret[i] = w.Clone()
	}
	return ret
}

func cloneAccounts(src []*ctrlertypes.Account) []*ctrlertypes.Account {
	ret := make([]*ctrlertypes.Account, len(src))
	for i, acct := range src {
		ret[i] = acct.Clone()
	}
	return ret
}

func copyAccount(dst, src *ctrlertypes.Account) {
	dst.SetName(src.GetName())
	dst.SetDocURL(src.GetDocURL())
	dst.SetNonce(src.GetNonce())
	dst.SetBalance(src.GetBalance())
	dst.SetCode(src.GetCode())
}

var _ ctrlertypes.IAccountHandler = (*AcctHandlerMock)(nil)
var _ ctrlertypes.ITrxLedgerSnapshotter = (*AcctHandlerMock)(nil)
