package account

import (
	"sync"

	cfg "github.com/beatoz/beatoz-go/cmd/config"
	btztypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/genesis"
	ledger "github.com/beatoz/beatoz-go/ledger"
	"github.com/beatoz/beatoz-go/ledger/common"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

type AcctCtrler struct {
	acctState *ledger.StateLedgerManager

	newbiesCheck   map[btztypes.AcctKey]*btztypes.Account
	newbiesDeliver map[btztypes.AcctKey]*btztypes.Account

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewAcctCtrler(config *cfg.Config, logger tmlog.Logger) (*AcctCtrler, error) {
	lg := logger.With("module", "beatoz_AcctCtrler")

	if _state, xerr := ledger.NewStateLedgerManager("accounts", config.DBDir(), 10000, func(key ledger.LedgerKey) ledger.ILedgerItem { return &btztypes.Account{} }, lg); xerr != nil {
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
		return ctrler.transfer(ctx.Tx.From, ctx.Tx.To, ctx.Tx.Amount, ctx.Exec)
	case btztypes.TRX_SETDOC:
		sender := ctrler.findAccount(ctx.Tx.From, ctx.Exec)
		if sender == nil {
			return xerrors.ErrNotFoundAccount.Wrapf("SetDoc - address: %v", ctx.Tx.From)
		}
		payload := ctx.Tx.Payload.(*btztypes.TrxPayloadSetDoc)
		ctrler.setDoc(sender, payload.Name, payload.URL)
		if xerr := ctrler.setAccount(sender, ctx.Exec); xerr != nil {
			return xerr
		}

		receiver := ctrler.findAccount(ctx.Tx.To, ctx.Exec)
		if receiver == nil {
			receiver = btztypes.NewAccountWithName(ctx.Tx.To, "")
		}
		return ctrler.setAccount(receiver, ctx.Exec)
	}

	return nil
}

func (ctrler *AcctCtrler) CreateCache(exec bool) xerrors.XError {
	return ctrler.acctState.CreateCache(exec)
}

func (ctrler *AcctCtrler) WriteCache(exec bool) xerrors.XError {
	return ctrler.acctState.WriteCache(exec)
}

func (ctrler *AcctCtrler) ClearCache(exec bool) xerrors.XError {
	return ctrler.acctState.ClearCache(exec)
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
	if acct, xerr := ctrler.acctState.Get(common.LedgerKeyAccount(addr), exec); xerr != nil {
		return nil
	} else {
		return acct.(*btztypes.Account)
	}
}

func (ctrler *AcctCtrler) Transfer(from, to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.transfer(from, to, amt, exec)
}

func (ctrler *AcctCtrler) transfer(from, to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	acct0 := ctrler.findAccount(from, exec)
	if acct0 == nil {
		return xerrors.ErrNotFoundAccount.Wrapf("Transfer - address: %v", from)
	}
	if xerr := acct0.SubBalance(amt); xerr != nil {
		return xerr
	}
	if xerr := ctrler.setAccount(acct0, exec); xerr != nil {
		return xerr
	}

	acct1 := ctrler.findAccount(to, exec)
	if acct1 == nil {
		acct1 = btztypes.NewAccountWithName(to, "")
	}
	if xerr := acct1.AddBalance(amt); xerr != nil {
		return xerr
	}
	return ctrler.setAccount(acct1, exec)
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

func (ctrler *AcctCtrler) Refund(to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	acct := ctrler.findAccount(to, exec)
	if acct == nil {
		return xerrors.ErrNotFoundAccount.Wrapf("Refund - address: %v", to)
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
	return ctrler.acctState.Set(common.LedgerKeyAccount(acct.Address), acct, exec)
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

func (ctrler *AcctCtrler) UpgradeLedgerVersion(target common.LedgerVersion) xerrors.XError {
	return ctrler.acctState.UpgradeLedgerVersion(target)
}

var _ btztypes.ILedgerHandler = (*AcctCtrler)(nil)
var _ btztypes.ITrxHandler = (*AcctCtrler)(nil)
var _ btztypes.IBlockHandler = (*AcctCtrler)(nil)
var _ btztypes.IAccountHandler = (*AcctCtrler)(nil)
var _ btztypes.ILedgerVersionHandler = (*AcctCtrler)(nil)

type SimuAcctCtrler struct {
	simuLedger ledger.IImitable
	newbies    map[btztypes.AcctKey]*btztypes.Account
	logger     tmlog.Logger
	mtx        sync.RWMutex
}

func (memCtrler *SimuAcctCtrler) SetAccount(acct *btztypes.Account, exec bool) xerrors.XError {
	return memCtrler.simuLedger.Set(common.LedgerKeyAccount(acct.Address), acct)
}

func (memCtrler *SimuAcctCtrler) CreateCache(bool) xerrors.XError {
	return xerrors.NewOrdinary("simulated account controller does not support transaction cache")
}

func (memCtrler *SimuAcctCtrler) WriteCache(bool) xerrors.XError {
	return xerrors.NewOrdinary("simulated account controller does not support transaction cache")
}

func (memCtrler *SimuAcctCtrler) ClearCache(bool) xerrors.XError {
	return xerrors.NewOrdinary("simulated account controller does not support transaction cache")
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
	if acct, xerr := memCtrler.simuLedger.Get(common.LedgerKeyAccount(addr)); xerr != nil {
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
func (memCtrler *SimuAcctCtrler) Refund(to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
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
