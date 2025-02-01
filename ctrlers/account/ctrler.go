package account

import (
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	btztypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/genesis"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

type AcctCtrler struct {
	acctState v1.IStateLedger[*btztypes.Account]

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewAcctCtrler(config *cfg.Config, logger tmlog.Logger) (*AcctCtrler, error) {
	lg := logger.With("module", "beatoz_AcctCtrler")

	if _state, xerr := v1.NewStateLedger[*btztypes.Account]("accounts", config.DBDir(), 2048, func() *btztypes.Account { return &btztypes.Account{} }, lg); xerr != nil {
		return nil, xerr
	} else {
		return &AcctCtrler{
			acctState: _state,
			logger:    lg,
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
			Balance: holder.Balance.Clone(),
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

func (ctrler *AcctCtrler) BeginBlock(ctx *btztypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	// do nothing
	return nil, nil
}

func (ctrler *AcctCtrler) EndBlock(ctx *btztypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	header := ctx.BlockInfo().Header
	if header.GetProposerAddress() != nil && ctx.SumFee().Sign() > 0 {
		//
		// give fee to block proposer
		// If the validator(proposer) has no balance in genesis and this is first tx fee reward,
		// the validator's account may not exist yet not in ledger.
		acct := ctrler.findAccount(header.GetProposerAddress(), true)
		if acct == nil {
			acct = btztypes.NewAccount(header.GetProposerAddress())
		}
		xerr := acct.AddBalance(ctx.SumFee())
		if xerr != nil {
			return nil, xerr
		}

		return nil, ctrler.setAccount(acct, true)
	}
	return nil, nil
}

func (ctrler *AcctCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	h, v, xerr := ctrler.acctState.Commit()
	return h, v, xerr
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

	// `AcctCtrler` MUST be locked until new account is set to acctState (issue #32)

	if acct := ctrler.findAccount(addr, exec); acct != nil {
		return acct
	}

	newAcct := btztypes.NewAccountWithName(addr, "")
	_ = ctrler.setAccount(newAcct, exec)
	return newAcct
}

func (ctrler *AcctCtrler) FindAccount(addr types.Address, exec bool) *btztypes.Account {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.findAccount(addr, exec)
}

func (ctrler *AcctCtrler) findAccount(addr types.Address, exec bool) *btztypes.Account {
	if acct, xerr := ctrler.acctState.Get(addr, exec); xerr != nil {
		//ctrler.logger.Debug("AcctCtrler - not found account", "address", addr, "error", xerr)
		return nil
	} else {
		return acct
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

func (ctrler *AcctCtrler) SetAccount(acct *btztypes.Account, exec bool) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.setAccount(acct, exec)
}

func (ctrler *AcctCtrler) setAccount(acct *btztypes.Account, exec bool) xerrors.XError {
	return ctrler.acctState.Set(acct, exec)
}

func (ctrler *AcctCtrler) SimuAcctCtrlerAt(height int64) (btztypes.IAccountHandler, xerrors.XError) {
	memLedger, xerr := ctrler.acctState.ImitableLedgerAt(height)
	if xerr != nil {
		return nil, xerr
	}

	return &SimuAcctCtrler{
		simuLedger: memLedger,
		logger:     ctrler.logger.With("module", "SimuAcctCtrler"),
	}, nil
}

var _ btztypes.ILedgerHandler = (*AcctCtrler)(nil)
var _ btztypes.ITrxHandler = (*AcctCtrler)(nil)
var _ btztypes.IBlockHandler = (*AcctCtrler)(nil)
var _ btztypes.IAccountHandler = (*AcctCtrler)(nil)

type SimuAcctCtrler struct {
	simuLedger v1.IImitable
	logger     tmlog.Logger
	mtx        sync.RWMutex
}

func (memCtrler *SimuAcctCtrler) SetAccount(acct *btztypes.Account, exec bool) xerrors.XError {
	return memCtrler.simuLedger.Set(acct)
}

func (memCtrler *SimuAcctCtrler) FindOrNewAccount(addr types.Address, exec bool) *btztypes.Account {
	memCtrler.mtx.Lock()
	defer memCtrler.mtx.Unlock()

	if acct := memCtrler.findAccount(addr); acct != nil {
		return acct
	}

	newAcct := btztypes.NewAccountWithName(addr, "")
	if newAcct != nil {
		_ = memCtrler.SetAccount(newAcct, exec)
	}
	return newAcct
}

func (memCtrler *SimuAcctCtrler) FindAccount(addr types.Address, exec bool) *btztypes.Account {
	memCtrler.mtx.RLock()
	defer memCtrler.mtx.RUnlock()

	return memCtrler.findAccount(addr)
}

func (memCtrler *SimuAcctCtrler) findAccount(addr types.Address) *btztypes.Account {
	if acct, xerr := memCtrler.simuLedger.Get(addr); xerr != nil {
		//memCtrler.logger.Debug("SimuAcctCtrler - not found account", "address", addr, "error", xerr)
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

func (memCtrler *SimuAcctCtrler) SimuAcctCtrlerAt(height int64) (btztypes.IAccountHandler, xerrors.XError) {
	panic("SimuAcctCtrler can not create ImmutableAcctCtrlerAt")
}

var _ btztypes.IAccountHandler = (*SimuAcctCtrler)(nil)
