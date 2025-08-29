package account

import (
	"sync"

	cfg "github.com/beatoz/beatoz-go/cmd/config"
	btztypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/genesis"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

type AcctCtrler struct {
	acctState v1.IStateLedger

	newbiesCheck   map[btztypes.AcctKey]*btztypes.Account
	newbiesDeliver map[btztypes.AcctKey]*btztypes.Account

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewAcctCtrler(config *cfg.Config, logger tmlog.Logger) (*AcctCtrler, error) {
	lg := logger.With("module", "beatoz_AcctCtrler")

	if _state, xerr := v1.NewStateLedger("accounts", config.DBDir(), 10000, func(key v1.LedgerKey) v1.ILedgerItem { return &btztypes.Account{} }, lg); xerr != nil {
		return nil, xerr
	} else {
		return &AcctCtrler{
			acctState:      _state,
			newbiesCheck:   make(map[btztypes.AcctKey]*btztypes.Account),
			newbiesDeliver: make(map[btztypes.AcctKey]*btztypes.Account),
			logger:         lg,
		}, nil
	}
}

func (ctrler *AcctCtrler) InitLedger(req interface{}) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	genAppState, ok := req.(*genesis.GenesisAppState)
	if !ok {
		return xerrors.ErrInitChain.Wrapf("wrong parameter: AcctCtrler::InitLedger requires *genesis.GenesisAppState")
	}

	for _, holder := range genAppState.AssetHolders {
		addr := append(holder.Address, nil...)
		acct := &btztypes.Account{
			Address: addr,
			Balance: holder.Balance,
		}
		if xerr := ctrler.setAccount(acct, true); xerr != nil {
			return xerr
		}
	}
	return nil
}

func (ctrler *AcctCtrler) ValidateTrx(ctx *btztypes.TrxContext) xerrors.XError {
	switch ctx.Tx.GetType() {
	case btztypes.TRX_SETDOC:
		name := ctx.Tx.Payload.(*btztypes.TrxPayloadSetDoc).Name
		url := ctx.Tx.Payload.(*btztypes.TrxPayloadSetDoc).URL
		if len(name) > btztypes.MAX_ACCT_NAME {
			return xerrors.ErrInvalidTrxPayloadParams.Wrapf("too long name. it should be less than %d.", btztypes.MAX_ACCT_NAME)
		}
		if len(url) > btztypes.MAX_ACCT_DOCURL {
			return xerrors.ErrInvalidTrxPayloadParams.Wrapf("too long url. it should be less than %d.", btztypes.MAX_ACCT_DOCURL)
		}
	}

	return nil
}

func (ctrler *AcctCtrler) ExecuteTrx(ctx *btztypes.TrxContext) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	switch ctx.Tx.GetType() {
	case btztypes.TRX_TRANSFER:
		if xerr := ctrler.transfer(ctx.Sender, ctx.Receiver, ctx.Tx.Amount); xerr != nil {
			return xerr
		}
	case btztypes.TRX_SETDOC:
		ctrler.setDoc(ctx.Sender,
			ctx.Tx.Payload.(*btztypes.TrxPayloadSetDoc).Name,
			ctx.Tx.Payload.(*btztypes.TrxPayloadSetDoc).URL)
	}

	_ = ctrler.setAccount(ctx.Sender, ctx.Exec)
	if ctx.Receiver != nil {
		_ = ctrler.setAccount(ctx.Receiver, ctx.Exec)
	}

	return nil
}

func (ctrler *AcctCtrler) Close() xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.acctState != nil {
		if xerr := ctrler.acctState.Close(); xerr != nil {
			ctrler.logger.Error("acctLedger.Close() returns error", "error", xerr.Error())
		}
		ctrler.logger.Debug("close ledgers")
		ctrler.acctState = nil
	}
	return nil
}

func (ctrler *AcctCtrler) FindOrNewAccount(addr types.Address, exec bool) *btztypes.Account {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	// `AcctCtrler` MUST be locked until a new account has been created (issue #32)

	if acct := ctrler.findAccount(addr, exec); acct != nil {
		return acct
	}

	newbies := ctrler.newbiesCheck
	if exec {
		newbies = ctrler.newbiesDeliver
	}
	acctKey := btztypes.ToAcctKey(addr)
	if acct, ok := newbies[acctKey]; ok {
		return acct
	}

	newAcct := btztypes.NewAccountWithName(addr, "")
	newbies[acctKey] = newAcct
	return newAcct
}

func (ctrler *AcctCtrler) FindAccount(addr types.Address, exec bool) *btztypes.Account {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.findAccount(addr, exec)
}

func (ctrler *AcctCtrler) findAccount(addr types.Address, exec bool) *btztypes.Account {
	if acct, xerr := ctrler.acctState.Get(v1.LedgerKeyAccount(addr), exec); xerr != nil {
		return nil
	} else {
		return acct.(*btztypes.Account)
	}
}

func (ctrler *AcctCtrler) Transfer(from, to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	acct0 := ctrler.findAccount(from, exec)
	if acct0 == nil {
		return xerrors.ErrNotFoundAccount.Wrapf("Transfer - address: %v", from)
	}
	acct1 := ctrler.findAccount(to, exec)
	if acct1 == nil {
		acct1 = btztypes.NewAccountWithName(to, "")
	}
	xerr := ctrler.transfer(acct0, acct1, amt)
	if xerr != nil {
		return xerr
	}

	if xerr := ctrler.setAccount(acct0, exec); xerr != nil {
		return xerr
	}
	if xerr := ctrler.setAccount(acct1, exec); xerr != nil {
		return xerr
	}
	return nil
}

func (ctrler *AcctCtrler) transfer(from, to *btztypes.Account, amt *uint256.Int) xerrors.XError {
	if err := from.SubBalance(amt); err != nil {
		return err
	}
	if err := to.AddBalance(amt); err != nil {
		_ = from.AddBalance(amt) // refund
		return err
	}
	return nil
}

func (ctrler *AcctCtrler) SetCode(addr types.Address, code []byte, exec bool) xerrors.XError {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	acct0 := ctrler.findAccount(addr, exec)
	if acct0 == nil {
		return xerrors.ErrNotFoundAccount.Wrapf("SetCode - address: %v", addr)
	}

	acct0.SetCode(code)

	if xerr := ctrler.setAccount(acct0, exec); xerr != nil {
		return xerr
	}
	return nil
}

func (ctrler *AcctCtrler) SetDoc(addr types.Address, name, url string, exec bool) xerrors.XError {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	acct0 := ctrler.findAccount(addr, exec)
	if acct0 == nil {
		return xerrors.ErrNotFoundAccount.Wrapf("SetDoc - address: %v", addr)
	}

	ctrler.setDoc(acct0, name, url)

	if xerr := ctrler.setAccount(acct0, exec); xerr != nil {
		return xerr
	}
	return nil
}

func (ctrler *AcctCtrler) setDoc(acct *btztypes.Account, name, url string) {
	acct.SetName(name)
	acct.SetDocURL(url)
}

// DEPRECATED: Add `AddBlance` and replace it.
func (ctrler *AcctCtrler) Reward(to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	acct := ctrler.findAccount(to, exec)
	if acct == nil {
		return xerrors.ErrNotFoundAccount.Wrapf("Reward - address: %v", to)
	}

	if xerr := acct.AddBalance(amt); xerr != nil {
		return xerr
	}
	if xerr := ctrler.setAccount(acct, exec); xerr != nil {
		return xerr
	}

	return nil
}
func (ctrler *AcctCtrler) AddBalance(addr types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	acct := ctrler.findAccount(addr, exec)
	if acct == nil {
		acct = btztypes.NewAccount(addr)
	}

	if xerr := acct.AddBalance(amt); xerr != nil {
		return xerr
	}
	if xerr := ctrler.setAccount(acct, exec); xerr != nil {
		return xerr
	}
	return nil
}

func (ctrler *AcctCtrler) SubBalance(addr types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	acct := ctrler.findAccount(addr, exec)
	if acct == nil {
		return xerrors.ErrNotFoundAccount.Wrapf("SubBalance - address: %v", addr)
	}

	if xerr := acct.SubBalance(amt); xerr != nil {
		return xerr
	}
	if xerr := ctrler.setAccount(acct, exec); xerr != nil {
		return xerr
	}
	return nil
}

func (ctrler *AcctCtrler) SetBalance(addr types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	acct := ctrler.findAccount(addr, exec)
	if acct == nil {
		acct = btztypes.NewAccount(addr)
	}
	acct.SetBalance(amt)

	if xerr := ctrler.setAccount(acct, exec); xerr != nil {
		return xerr
	}
	return nil
}

func (ctrler *AcctCtrler) SetAccount(acct *btztypes.Account, exec bool) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.setAccount(acct, exec)
}

func (ctrler *AcctCtrler) setAccount(acct *btztypes.Account, exec bool) xerrors.XError {
	return ctrler.acctState.Set(v1.LedgerKeyAccount(acct.Address), acct, exec)
}

func (ctrler *AcctCtrler) SimuAcctCtrlerAt(height int64) (btztypes.IAccountHandler, xerrors.XError) {
	memLedger, xerr := ctrler.acctState.ImitableLedgerAt(height)
	if xerr != nil {
		return nil, xerr
	}

	return &SimuAcctCtrler{
		simuLedger: memLedger,
		newbies:    make(map[btztypes.AcctKey]*btztypes.Account),
		logger:     ctrler.logger.With("module", "SimuAcctCtrler"),
	}, nil
}

var _ btztypes.ILedgerHandler = (*AcctCtrler)(nil)
var _ btztypes.ITrxHandler = (*AcctCtrler)(nil)
var _ btztypes.IBlockHandler = (*AcctCtrler)(nil)
var _ btztypes.IAccountHandler = (*AcctCtrler)(nil)

type SimuAcctCtrler struct {
	simuLedger v1.IImitable
	newbies    map[btztypes.AcctKey]*btztypes.Account
	logger     tmlog.Logger
	mtx        sync.RWMutex
}

func (memCtrler *SimuAcctCtrler) SetAccount(acct *btztypes.Account, exec bool) xerrors.XError {
	return memCtrler.simuLedger.Set(v1.LedgerKeyAccount(acct.Address), acct)
}

func (memCtrler *SimuAcctCtrler) FindOrNewAccount(addr types.Address, exec bool) *btztypes.Account {
	memCtrler.mtx.Lock()
	defer memCtrler.mtx.Unlock()

	if acct := memCtrler.findAccount(addr); acct != nil {
		return acct
	}
	acctKey := btztypes.ToAcctKey(addr)
	if acct, ok := memCtrler.newbies[acctKey]; ok {
		return acct
	}

	newAcct := btztypes.NewAccountWithName(addr, "")
	memCtrler.newbies[acctKey] = newAcct
	return newAcct
}

func (memCtrler *SimuAcctCtrler) FindAccount(addr types.Address, exec bool) *btztypes.Account {
	memCtrler.mtx.RLock()
	defer memCtrler.mtx.RUnlock()

	return memCtrler.findAccount(addr)
}

func (memCtrler *SimuAcctCtrler) findAccount(addr types.Address) *btztypes.Account {
	if acct, xerr := memCtrler.simuLedger.Get(v1.LedgerKeyAccount(addr)); xerr != nil {
		return nil
	} else {
		return acct.(*btztypes.Account)
	}
}

func (memCtrler *SimuAcctCtrler) Transfer(from types.Address, to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	panic("SimuAcctCtrler can not have this method")
}

func (memCtrler *SimuAcctCtrler) Reward(to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	panic("SimuAcctCtrler can not have this method")
}
func (memCtrler *SimuAcctCtrler) AddBalance(addr types.Address, amt *uint256.Int, b bool) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (memCtrler *SimuAcctCtrler) SubBalance(addr types.Address, amt *uint256.Int, b bool) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (memCtrler *SimuAcctCtrler) SetBalance(addr types.Address, amt *uint256.Int, b bool) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (memCtrler *SimuAcctCtrler) SimuAcctCtrlerAt(height int64) (btztypes.IAccountHandler, xerrors.XError) {
	panic("SimuAcctCtrler can not create ImmutableAcctCtrlerAt")
}

func (memCtrler *SimuAcctCtrler) ValidateTrx(context *btztypes.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (memCtrler *SimuAcctCtrler) ExecuteTrx(context *btztypes.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (memCtrler *SimuAcctCtrler) BeginBlock(context *btztypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (memCtrler *SimuAcctCtrler) EndBlock(context *btztypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (memCtrler *SimuAcctCtrler) Commit() ([]byte, int64, xerrors.XError) {
	memCtrler.newbies = make(map[btztypes.AcctKey]*btztypes.Account)
	return nil, 0, nil
}

var _ btztypes.IAccountHandler = (*SimuAcctCtrler)(nil)
