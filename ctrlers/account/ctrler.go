package account

import (
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	atypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/genesis"
	"github.com/beatoz/beatoz-go/ledger"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

type AcctCtrler struct {
	acctLedger ledger.IFinalityLedger[*atypes.Account]

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewAcctCtrler(config *cfg.Config, logger tmlog.Logger) (*AcctCtrler, error) {
	if execLedger, err := ledger.NewFinalityLedger[*atypes.Account]("accounts", config.DBDir(), 128, func() *atypes.Account { return &atypes.Account{} }); err != nil {
		return nil, err
	} else {
		return &AcctCtrler{
			acctLedger: execLedger,
			logger:     logger.With("module", "beatoz_AcctCtrler"),
		}, nil
	}
}

func (ctrler *AcctCtrler) ImmutableAcctCtrlerAt(height int64) (atypes.IAccountHandler, xerrors.XError) {
	ledger0, xerr := ctrler.acctLedger.ImmutableLedgerAt(height, 128)
	if xerr != nil {
		return nil, xerr
	}

	return &ImmuAcctCtrler{
		immuLedger: ledger0,
		logger:     ctrler.logger,
	}, nil
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
		if xerr := ctrler.setAccountCommittable(acct, true); xerr != nil {
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

	_ = ctrler.setAccountCommittable(ctx.Sender, ctx.Exec)
	if ctx.Receiver != nil {
		_ = ctrler.setAccountCommittable(ctx.Receiver, ctx.Exec)
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

		return nil, ctrler.setAccountCommittable(acct, true)
	}
	return nil, nil
}

func (ctrler *AcctCtrler) Commit() ([]byte, int64, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.acctLedger.Commit()
}

func (ctrler *AcctCtrler) Close() xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.acctLedger != nil {
		if xerr := ctrler.acctLedger.Close(); xerr != nil {
			ctrler.logger.Error("AcctCtrler", "acctLedger.Close() returns error", xerr.Error())
		}
		ctrler.logger.Debug("AcctCtrler - close ledgers")
		ctrler.acctLedger = nil
	}
	return nil
}

func (ctrler *AcctCtrler) findAccount(addr types.Address, exec bool) *atypes.Account {
	k := ledger.ToLedgerKey(addr)

	fn := ctrler.acctLedger.Get
	if exec {
		fn = ctrler.acctLedger.GetFinality
	}

	if acct, xerr := fn(k); xerr != nil {
		//ctrler.logger.Debug("AcctCtrler - not found account", "address", addr, "error", xerr)
		return nil
	} else {
		return acct
	}
}

func (ctrler *AcctCtrler) FindOrNewAccount(addr types.Address, exec bool) *atypes.Account {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	// `AcctCtrler` MUST be locked until new account is set to acctLedger (issue #32)

	if acct := ctrler.findAccount(addr, exec); acct != nil {
		return acct
	}

	newAcct := atypes.NewAccountWithName(addr, "")
	ctrler.setAccountCommittable(newAcct, exec)
	return newAcct
}

func (ctrler *AcctCtrler) FindAccount(addr types.Address, exec bool) *atypes.Account {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.findAccount(addr, exec)
}

func (ctrler *AcctCtrler) ReadAccount(addr types.Address) *atypes.Account {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.readAccount(addr)
}

func (ctrler *AcctCtrler) readAccount(addr types.Address) *atypes.Account {
	if acct, xerr := ctrler.acctLedger.Read(addr.Array32()); xerr != nil {
		// db error or not found
		return atypes.NewAccount(addr)
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
		acct1 = atypes.NewAccountWithName(to, "")
	}
	xerr := ctrler.transfer(acct0, acct1, amt)
	if xerr != nil {
		return xerr
	}

	if xerr := ctrler.setAccountCommittable(acct0, exec); xerr != nil {
		return xerr
	}
	if xerr := ctrler.setAccountCommittable(acct1, exec); xerr != nil {
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

	if xerr := ctrler.setAccountCommittable(acct0, exec); xerr != nil {
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

	if xerr := ctrler.setAccountCommittable(acct0, exec); xerr != nil {
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

	if acct := ctrler.findAccount(to, exec); acct == nil {
		return xerrors.ErrNotFoundAccount.Wrapf("Reward - address: %v", to)
	} else if xerr := acct.AddBalance(amt); xerr != nil {
		return xerr
	} else if xerr := ctrler.setAccountCommittable(acct, exec); xerr != nil {
		return xerr
	}
	return nil
}

func (ctrler *AcctCtrler) SetAccountCommittable(acct *atypes.Account, exec bool) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.setAccountCommittable(acct, exec)
}

func (ctrler *AcctCtrler) setAccountCommittable(acct *atypes.Account, exec bool) xerrors.XError {
	fn := ctrler.acctLedger.Set
	if exec {
		fn = ctrler.acctLedger.SetFinality
	}
	return fn(acct)
}

var _ atypes.ILedgerHandler = (*AcctCtrler)(nil)
var _ atypes.ITrxHandler = (*AcctCtrler)(nil)
var _ atypes.IBlockHandler = (*AcctCtrler)(nil)
var _ atypes.IAccountHandler = (*AcctCtrler)(nil)

type ImmuAcctCtrler struct {
	immuLedger ledger.ILedger[*atypes.Account]
	logger     tmlog.Logger
}

//func (immuCtrler *ImmuAcctCtrler) SetCode(addr types.Address, code []byte, exec bool) xerrors.XError {
//	acct0 := immuCtrler.FindAccount(addr, exec)
//	if acct0 == nil {
//		return xerrors.ErrNotFoundAccount.Wrapf("address: %v", addr)
//	}
//
//	acct0.SetCode(code)
//
//	if xerr := immuCtrler.SetAccountCommittable(acct0, exec); xerr != nil {
//		return xerr
//	}
//	return nil
//}
//
//func (immuCtrler *ImmuAcctCtrler) SetDoc(addr types.Address, name string, url string, exec bool) xerrors.XError {
//	acct0 := immuCtrler.FindAccount(addr, exec)
//	if acct0 == nil {
//		return xerrors.ErrNotFoundAccount.Wrapf("address: %v", addr)
//	}
//
//	acct0.SetName(url)
//	acct0.SetDocURL(url)
//
//	if xerr := immuCtrler.SetAccountCommittable(acct0, exec); xerr != nil {
//		return xerr
//	}
//	return nil
//}

func (immuCtrler *ImmuAcctCtrler) FindOrNewAccount(addr types.Address, exec bool) *atypes.Account {
	// `AcctCtrler` MUST be locked until new account is set to acctLedger (issue #32)

	if acct := immuCtrler.FindAccount(addr, exec); acct != nil {
		return acct
	}

	newAcct := atypes.NewAccountWithName(addr, "")
	if newAcct != nil {
		_ = immuCtrler.SetAccountCommittable(newAcct, exec)
	}
	return newAcct
}

func (immuCtrler *ImmuAcctCtrler) FindAccount(addr types.Address, exec bool) *atypes.Account {
	k := ledger.ToLedgerKey(addr)

	if acct, xerr := immuCtrler.immuLedger.Get(k); xerr != nil {
		//immuCtrler.logger.Debug("ImmuAcctCtrler - not found account", "address", addr, "error", xerr)
		return nil
	} else {
		return acct
	}
}

func (immuCtrler *ImmuAcctCtrler) Transfer(from types.Address, to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	acct0 := immuCtrler.FindAccount(from, exec)
	if acct0 == nil {
		return xerrors.ErrNotFoundAccount.Wrapf("address: %v", from)
	}
	acct1 := immuCtrler.FindAccount(to, exec)
	if acct1 == nil {
		acct1 = atypes.NewAccountWithName(to, "")
	}

	if xerr := acct0.SubBalance(amt); xerr != nil {
		return xerr
	}
	if xerr := acct1.AddBalance(amt); xerr != nil {
		_ = acct0.AddBalance(amt) // refund
		return xerr
	}

	if xerr := immuCtrler.SetAccountCommittable(acct0, exec); xerr != nil {
		return xerr
	}
	if xerr := immuCtrler.SetAccountCommittable(acct1, exec); xerr != nil {
		return xerr
	}
	return nil
}

func (immuCtrler *ImmuAcctCtrler) Reward(to types.Address, amt *uint256.Int, exec bool) xerrors.XError {
	if acct := immuCtrler.FindAccount(to, exec); acct == nil {
		return xerrors.ErrNotFoundAccount.Wrapf("address: %v", to)
	} else if xerr := acct.AddBalance(amt); xerr != nil {
		return xerr
	} else if xerr := immuCtrler.SetAccountCommittable(acct, exec); xerr != nil {
		return xerr
	}
	return nil
}

func (immuCtrler *ImmuAcctCtrler) ImmutableAcctCtrlerAt(height int64) (atypes.IAccountHandler, xerrors.XError) {
	panic("ImmuAcctCtrler can not create ImmutableAcctCtrlerAt")
}

func (immuCtrler *ImmuAcctCtrler) SetAccountCommittable(acct *atypes.Account, exec bool) xerrors.XError {
	return immuCtrler.immuLedger.Set(acct)
}

var _ atypes.IAccountHandler = (*ImmuAcctCtrler)(nil)
