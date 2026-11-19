package upgrade

import (
	"bytes"
	"fmt"
	"sync"

	cfg "github.com/beatoz/beatoz-go/cmd/config"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ledger"
	"github.com/beatoz/beatoz-go/ledger/common"
	"github.com/beatoz/beatoz-go/types/xerrors"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

type UpgradeCtrler struct {
	upgradeState *ledger.StateLedgerManager

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func defaultNewItem(key common.LedgerKey) common.ILedgerItem {
	if bytes.HasPrefix(key, common.KeyPrefixUpgradePlan) {
		return &UpgradePlan{}
	} else if bytes.HasPrefix(key, common.KeyPrefixUpgradeDone) {
		return &UpgradeDone{}
	}
	panic(fmt.Errorf("invalid key prefix:0x%x", key[0]))
}

func NewUpgradeCtrler(config *cfg.Config, logger tmlog.Logger) (*UpgradeCtrler, xerrors.XError) {
	lg := logger.With("module", "beatoz_UpgradeCtrler")

	state, xerr := ledger.NewStateLedgerManager("upgrade", config.DBDir(), 100, defaultNewItem, lg)
	if xerr != nil {
		return nil, xerr
	}

	return &UpgradeCtrler{
		upgradeState: state,
		logger:       lg,
	}, nil
}

func (ctrler *UpgradeCtrler) InitLedger(_ interface{}) xerrors.XError {
	return nil
}

func (ctrler *UpgradeCtrler) ValidateTrx(_ *ctrlertypes.TrxContext) xerrors.XError {
	return xerrors.ErrUnknownTrxType
}

func (ctrler *UpgradeCtrler) ExecuteTrx(_ *ctrlertypes.TrxContext) xerrors.XError {
	return xerrors.ErrUnknownTrxType
}

func (ctrler *UpgradeCtrler) ReservedUpgrade(exec bool) (*UpgradePlan, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.getReservedUpgrade(exec)
}

func (ctrler *UpgradeCtrler) CompletedUpgrade(name string, exec bool) (*UpgradeDone, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.getCompletedUpgrade(name, exec)
}

func (ctrler *UpgradeCtrler) LastCompletedUpgrade(exec bool) (*UpgradeDone, xerrors.XError) {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.getLastCompletedUpgrade(exec)
}

func (ctrler *UpgradeCtrler) getLastCompletedUpgrade(exec bool) (*UpgradeDone, xerrors.XError) {
	var last *UpgradeDone
	xerr := ctrler.upgradeState.Seek(common.KeyPrefixUpgradeDone, true, func(_ common.LedgerKey, item common.ILedgerItem) xerrors.XError {
		done, _ := item.(*UpgradeDone)
		if done != nil && (last == nil || done.Height() > last.Height()) {
			last = done
		}
		return nil
	}, exec)
	if xerr != nil {
		return nil, xerr
	}
	return last, nil
}

func (ctrler *UpgradeCtrler) ValidateUpgrade(opt *ctrlertypes.UpgradePlanOption, applyHeight, minDelayBlocks int64, exec bool) xerrors.XError {
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	return ctrler.validateUpgrade(opt, applyHeight, minDelayBlocks, exec)
}

func (ctrler *UpgradeCtrler) ReserveUpgrade(opt *ctrlertypes.UpgradePlanOption, txhash []byte, applyHeight, minDelayBlocks int64) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if xerr := ctrler.validateUpgrade(opt, applyHeight, minDelayBlocks, true); xerr != nil {
		return xerr
	}
	plan := NewUpgradePlan(opt.Name, opt.Height, opt.Info, txhash)
	if xerr := ctrler.upgradeState.Set(common.LedgerKeyUpgradePlan(), plan, true); xerr != nil {
		return xerr
	}
	ctrler.logger.Info("upgrade plan is set", "name", plan.Name(), "height", plan.Height())
	return nil
}

func (ctrler *UpgradeCtrler) CancelUpgrade(name string) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	plan, xerr := ctrler.getReservedUpgrade(true)
	if xerr != nil {
		return xerr
	}
	if plan == nil {
		return xerrors.NewOrdinary(fmt.Sprintf("no pending upgrade plan to delete: %s", name))
	}
	if plan.Name() != name {
		return xerrors.NewOrdinary(fmt.Sprintf("pending upgrade plan is not %s: %s", name, plan.Name()))
	}
	if xerr := ctrler.upgradeState.Del(common.LedgerKeyUpgradePlan(), true); xerr != nil {
		return xerr
	}
	ctrler.logger.Info("upgrade plan is deleted", "name", plan.Name(), "height", plan.Height())
	return nil
}

func (ctrler *UpgradeCtrler) CompleteUpgrade(plan *UpgradePlan) xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	done := NewUpgradeDone(plan.Name(), plan.Height())
	if xerr := ctrler.upgradeState.Set(common.LedgerKeyUpgradeDone(done.Name()), done, true); xerr != nil {
		return xerr
	}
	if xerr := ctrler.upgradeState.Del(common.LedgerKeyUpgradePlan(), true); xerr != nil {
		return xerr
	}
	ctrler.logger.Info("upgrade is applied", "name", done.Name(), "height", done.Height())
	return nil
}

func (ctrler *UpgradeCtrler) validateUpgrade(opt *ctrlertypes.UpgradePlanOption, applyHeight, minDelayBlocks int64, exec bool) xerrors.XError {
	if opt == nil {
		return xerrors.ErrInvalidTrxPayloadParams.Wrapf("upgrade plan is nil")
	}
	if xerr := opt.ValidateBasic(); xerr != nil {
		return xerr
	}
	if opt.Height <= applyHeight {
		return xerrors.ErrInvalidTrxPayloadParams.Wrapf("upgrade height must be greater than applying height: %d <= %d", opt.Height, applyHeight)
	}
	if opt.Height < applyHeight+minDelayBlocks {
		return xerrors.ErrInvalidTrxPayloadParams.Wrapf("upgrade height must be equal to or greater than %d: %d", applyHeight+minDelayBlocks, opt.Height)
	}
	done, xerr := ctrler.getCompletedUpgrade(opt.Name, exec)
	if xerr != nil {
		return xerr
	}
	if done != nil {
		return xerrors.ErrInvalidTrxPayloadParams.Wrapf("upgrade %s is already done at %d", done.Name(), done.Height())
	}
	return nil
}

func (ctrler *UpgradeCtrler) getReservedUpgrade(exec bool) (*UpgradePlan, xerrors.XError) {
	item, xerr := ctrler.upgradeState.Get(common.LedgerKeyUpgradePlan(), exec)
	if xerr != nil {
		if xerr.Contains(xerrors.ErrNotFoundResult) {
			return nil, nil
		}
		return nil, xerr
	}
	plan, _ := item.(*UpgradePlan)
	return plan, nil
}

func (ctrler *UpgradeCtrler) getCompletedUpgrade(name string, exec bool) (*UpgradeDone, xerrors.XError) {
	item, xerr := ctrler.upgradeState.Get(common.LedgerKeyUpgradeDone(name), exec)
	if xerr != nil {
		if xerr.Contains(xerrors.ErrNotFoundResult) {
			return nil, nil
		}
		return nil, xerr
	}
	done, _ := item.(*UpgradeDone)
	return done, nil
}

func (ctrler *UpgradeCtrler) Close() xerrors.XError {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	if ctrler.upgradeState != nil {
		if xerr := ctrler.upgradeState.Close(); xerr != nil {
			ctrler.logger.Error("fail to close upgradeState", "error", xerr.Error())
		}
		ctrler.upgradeState = nil
	}
	return nil
}

func (ctrler *UpgradeCtrler) UpgradeLedgerVersion(target common.LedgerVersion) xerrors.XError {
	return ctrler.upgradeState.UpgradeLedgerVersion(target)
}

var _ ctrlertypes.ILedgerHandler = (*UpgradeCtrler)(nil)
var _ ctrlertypes.ITrxHandler = (*UpgradeCtrler)(nil)
var _ ctrlertypes.IBlockHandler = (*UpgradeCtrler)(nil)
var _ ctrlertypes.IUpgradeHandler = (*UpgradeCtrler)(nil)
var _ ctrlertypes.ILedgerVersionHandler = (*UpgradeCtrler)(nil)
