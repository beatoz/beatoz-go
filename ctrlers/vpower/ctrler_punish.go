package vpower

import (
	"bytes"
	"encoding/binary"

	"github.com/beatoz/beatoz-go/ledger/common"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

type frozenVPowerAtHeight struct {
	height int64
	frozen *FrozenVPower
}

func (ctrler *VPowerCtrler) findFrozenVPowers(targetAddr types.Address, exec bool) ([]frozenVPowerAtHeight, xerrors.XError) {
	heightOffset := len(common.KeyPrefixFrozenVPower)
	fromOffset := heightOffset + 8
	wantKeyLen := fromOffset + len(targetAddr)
	var frozenHeights []int64

	// Seek holds the ledger read lock, so collect heights before reading the execution state.
	xerr := ctrler.vpowerState.Seek(
		common.KeyPrefixFrozenVPower,
		true,
		func(key common.LedgerKey, _ common.ILedgerItem) xerrors.XError {
			if len(key) != wantKeyLen {
				return xerrors.NewOrdinary("invalid frozen vpower key length").Wrapf(
					"expected:%d, actual:%d, key:%x", wantKeyLen, len(key), key,
				)
			}
			if !bytes.Equal(key[fromOffset:], targetAddr) {
				return nil
			}

			frozenHeights = append(frozenHeights, int64(binary.BigEndian.Uint64(key[heightOffset:fromOffset])))
			return nil
		},
		exec,
	)
	if xerr != nil {
		if xerr.Contains(xerrors.ErrNotFoundResult) {
			return nil, nil
		}
		return nil, xerr
	}

	frozenPowers := make([]frozenVPowerAtHeight, 0, len(frozenHeights))
	for _, frozenHeight := range frozenHeights {
		frozen, xerr := ctrler.readFrozenVPower(frozenHeight, targetAddr, exec)
		if xerr != nil {
			return nil, xerr
		}
		if frozen == nil {
			return nil, xerrors.NewOrdinary("invalid frozen vpower item type")
		}
		frozenPowers = append(frozenPowers, frozenVPowerAtHeight{
			height: frozenHeight,
			frozen: frozen,
		})
	}
	return frozenPowers, nil
}

func slashFrozenVPower(frozen *FrozenVPower, slashRate int32) int64 {
	slashed := slashPowerChunks(frozen.PowerChunks, slashRate)
	frozen.RefundPower -= slashed
	return slashed
}

// slashFrozenVPowers slashes and persists the loaded frozen power for targetAddr.
// targetAddr is the refund recipient, not the original delegatee.
func (ctrler *VPowerCtrler) slashFrozenVPowers(
	targetAddr types.Address,
	frozenPowers []frozenVPowerAtHeight,
	slashRate int32,
	exec bool,
) (int64, xerrors.XError) {
	slashed := int64(0)
	for _, frozenPower := range frozenPowers {
		slashed += slashFrozenVPower(frozenPower.frozen, slashRate)
		if xerr := ctrler.vpowerState.Set(
			common.LedgerKeyFrozenVPower(frozenPower.height, targetAddr), frozenPower.frozen, exec,
		); xerr != nil {
			return 0, xerr
		}
	}
	return slashed, nil
}

// doSlash loads and slashes frozen power before force-unbonding active power.
// It tombstones the target after successful processing. Follow-up evidence is
// ignored only when the target is tombstoned and has no frozen power left.
// It returns the total slashed power, which is zero for ignored evidence and may
// also be zero after integer rounding.
// On error, callers must ignore slashed.
func (ctrler *VPowerCtrler) doSlash(
	targetAddr types.Address,
	slashRate int32,
	refundHeight int64,
) (slashed int64, xerr xerrors.XError) {
	tombstoned, xerr := ctrler.isTombstoned(targetAddr, true)
	if xerr != nil {
		return 0, xerr
	}

	frozenPowers, xerr := ctrler.findFrozenVPowers(targetAddr, true)
	if xerr != nil {
		return 0, xerr
	}
	if tombstoned && len(frozenPowers) == 0 {
		return 0, nil
	}

	// Frozen power may coexist with an active Delegatee after partial self-unstaking
	// (including a self-power-rate drop), or remain alone after force-unbond caused by
	// zero self-power, missed blocks, or earlier evidence. Process it before active power
	// so power newly frozen by this evidence is not slashed twice.
	frozenSlashed, xerr := ctrler.slashFrozenVPowers(targetAddr, frozenPowers, slashRate, true)
	if xerr != nil {
		return 0, xerr
	}

	dgtee, xerr := ctrler.readDelegatee(targetAddr, true)
	if xerr != nil && !xerr.Contains(xerrors.ErrNotFoundResult) {
		return 0, xerr
	}

	activeSlashed := int64(0)
	if dgtee != nil {
		// Slash only the validator's active self-delegated power.
		selfPowerBefore := dgtee.SelfPower
		if xerr := ctrler.forceUnbondDelegatee(dgtee, refundHeight, true, slashRate); xerr != nil {
			return 0, xerr
		}
		activeSlashed = selfPowerBefore - dgtee.SelfPower
	}

	if !tombstoned {
		if xerr := ctrler.setTombstone(targetAddr, true); xerr != nil {
			return 0, xerr
		}
	}

	return frozenSlashed + activeSlashed, nil
}
