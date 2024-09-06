package account

import (
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	atypes "github.com/beatoz/beatoz-go/ctrlers/types"
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
	acctState *v1.Ledger

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewAcctCtrler(config *cfg.Config, logger tmlog.Logger) (*AcctCtrler, error) {

	lg := logger.With("module", "beatoz_AcctCtrler")
	if _state, xerr := v1.NewLedger("accounts", config.DBDir(), 2048, func() v1.ILedgerItem { return &atypes.Account{} }, lg); xerr != nil {
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
		acct := &atypes.Account{
			Address: addr,
			Balance: holder.Balance.Clone(),
		}
		if xerr := ctrler.setAccount(acct, true); xerr != nil {
			return xerr
		}
	}
	return nil
}

func (ctrler *AcctCtrler) ValidateTrx(ctx *atypes.TrxContext) xerrors.XError {
	switch ctx.Tx.GetType() {
	case atypes.TRX_SETDOC:
		name := ctx.Tx.Payload.(*atypes.TrxPayloadSetDoc).Name
		url := ctx.Tx.Payload.(*atypes.TrxPayloadSetDoc).URL
		if len(name) > atypes.MAX_ACCT_NAME {
			return xerrors.ErrInvalidTrxPayloadParams.Wrapf("too long name. it should be less than %d.", atypes.MAX_ACCT_NAME)
		}
		if len(url) > atypes.MAX_ACCT_DOCURL {
			return xerrors.ErrInvalidTrxPayloadParams.Wrapf("too long url. it should be less than %d.", atypes.MAX_ACCT_DOCURL)
		}
	}

	return nil
}

func (ctrler *AcctCtrler) ExecuteTrx(ctx *atypes.TrxContext) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	switch ctx.Tx.GetType() {
	case atypes.TRX_TRANSFER:
		if xerr := ctrler.transfer(ctx.Sender, ctx.Receiver, ctx.Tx.Amount); xerr != nil {
			return xerr
		}
	case atypes.TRX_SETDOC:
		ctrler.setDoc(ctx.Sender,
			ctx.Tx.Payload.(*atypes.TrxPayloadSetDoc).Name,
			ctx.Tx.Payload.(*atypes.TrxPayloadSetDoc).URL)
	}

	_ = ctrler.setAccount(ctx.Sender, ctx.Exec)
	if ctx.Receiver != nil {
		_ = ctrler.setAccount(ctx.Receiver, ctx.Exec)
	}

	return nil
}

func (ctrler *AcctCtrler) BeginBlock(ctx *atypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	// do nothing
	return nil, nil
}

func (ctrler *AcctCtrler) EndBlock(ctx *atypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
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
			acct = atypes.NewAccount(header.GetProposerAddress())
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

	return ctrler.acctState.Commit()
}

func (ctrler *AcctCtrler) Close() xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.acctState != nil {
		if xerr := ctrler.acctState.Close(); xerr != nil {
			ctrler.logger.Error("AcctCtrler", "acctLedger.Close() returns error", xerr.Error())
		}
		ctrler.logger.Debug("AcctCtrler - close ledgers")
		ctrler.acctState = nil
	}
	return nil
}

func (ctrler *AcctCtrler) FindOrNewAccount(addr types.Address, exec bool) *atypes.Account {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	// `AcctCtrler` MUST be locked until new account is set to acctState (issue #32)

	if acct := ctrler.findAccount(addr, exec); acct != nil {
		return acct
	}

	newAcct := atypes.NewAccountWithName(addr, "")
	_ = ctrler.setAccount(newAcct, exec)
	return newAcct
}

func (ctrler *AcctCtrler) FindAccount(addr types.Address, exec bool) *atypes.Account {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.findAccount(addr, exec)
}

func (ctrler *AcctCtrler) findAccount(addr types.Address, exec bool) *atypes.Account {
	if acct, xerr := ctrler.acctState.GetLedger(exec).Get(addr); xerr != nil {
		//ctrler.logger.Debug("AcctCtrler - not found account", "address", addr, "error", xerr)
		return nil
	} else {
		return acct.(*atypes.Account)
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
		acct1 = atypes.NewAccountWithName(to, "")
	}
	xerr := ctrler.transfer(acct0, acct1, amt)
	if xerr != nil {
		return xerr
	}

	if xerr := ctrler.setAccount(acct0, exec); xerr != nil {
		return xerr
	}
	if xerr := ctrler.setAccount(acct1, exec); xerr != nil {
		// todo: cancel ctrler.setAccount(acct0,exec)
		return xerr
	}
	return nil
}

func (ctrler *AcctCtrler) transfer(from, to *atypes.Account, amt *uint256.Int) xerrors.XError {
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

func (ctrler *AcctCtrler) setDoc(acct *atypes.Account, name, url string) {
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

func (ctrler *AcctCtrler) SetAccount(acct *atypes.Account, exec bool) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.setAccount(acct, exec)
}

func (ctrler *AcctCtrler) setAccount(acct *atypes.Account, exec bool) xerrors.XError {
	return ctrler.acctState.GetLedger(exec).Set(acct)
}

func (ctrler *AcctCtrler) MempoolAcctCtrlerAt(height int64) (atypes.IAccountHandler, xerrors.XError) {
	_ledger := ctrler.acctState.GetLedger(true)
	memLedger, xerr := _ledger.MempoolLedgerAt(height)
	if xerr != nil {
		return nil, xerr
	}

	return &MempoolAcctCtrler{
		mempoolLedger: memLedger,
		logger:        ctrler.logger,
	}, nil
}

var _ atypes.ILedgerHandler = (*AcctCtrler)(nil)
var _ atypes.ITrxHandler = (*AcctCtrler)(nil)
var _ atypes.IBlockHandler = (*AcctCtrler)(nil)
var _ atypes.IAccountHandler = (*AcctCtrler)(nil)

type MempoolAcctCtrler struct {
	mempoolLedger v1.ILedger
	logger        tmlog.Logger
	mtx           sync.RWMutex
}

func (memCtrler *MempoolAcctCtrler) SetAccount(acct *atypes.Account, exec bool) xerrors.XError {
	return memCtrler.mempoolLedger.Set(acct)
}

func (memCtrler *MempoolAcctCtrler) FindOrNewAccount(addr types.Address, exec bool) *atypes.Account {
	memCtrler.mtx.Lock()
	defer memCtrler.mtx.Unlock()

	if acct := memCtrler.findAccount(addr, exec); acct != nil {
		return acct
	}

	newAcct := atypes.NewAccountWithName(addr, "")
	if newAcct != nil {
		_ = memCtrler.SetAccount(newAcct, exec)
	}
	return newAcct
}

func (memCtrler *MempoolAcctCtrler) FindAccount(addr types.Address, exec bool) *atypes.Account {
	memCtrler.mtx.RLock()
	defer memCtrler.mtx.RUnlock()

	return memCtrler.findAccount(addr, exec)
}

func (memCtrler *MempoolAcctCtrler) findAccount(addr types.Address, exec bool) *atypes.Account {
	if acct, xerr := memCtrler.mempoolLedger.Get(addr); xerr != nil {
		//memCtrler.logger.Debug("MempoolAcctCtrler - not found account", "address", addr, "error", xerr)
		return nil
	} else {
		return acct.(*atypes.Account)
	}
}

func (memCtrler *MempoolAcctCtrler) Transfer(from types.Address, to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	panic("MempoolAcctCtrler can not have this method")
}

func (memCtrler *MempoolAcctCtrler) Reward(to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	panic("MempoolAcctCtrler can not have this method")
}

func (memCtrler *MempoolAcctCtrler) MempoolAcctCtrlerAt(height int64) (atypes.IAccountHandler, xerrors.XError) {
	panic("MempoolAcctCtrler can not create ImmutableAcctCtrlerAt")
}

var _ atypes.IAccountHandler = (*MempoolAcctCtrler)(nil)
