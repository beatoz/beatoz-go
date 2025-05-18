package vpower

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

const (
	WHEN_POWER_ADD = true
	WHEN_POWER_SUB = false
)

// VPowerLimiter limits the change of voting power in one block.
// It limits a stakeholder’s stake change to one-third of their current holdings.
// And, it also limits total voting power changes to one-third of the **changed** total.
type VPowerLimiter struct {
	lastTotalPower int64

	newTotalPower int64
	addingPower   int64
	subingPower   int64

	allowRate int32
}

func NewVPowerLimiter() *VPowerLimiter {
	return &VPowerLimiter{}
}

func (limiter *VPowerLimiter) Reset(total int64, allowRate int32) {
	limiter.lastTotalPower = total
	limiter.newTotalPower = total
	limiter.addingPower = 0
	limiter.subingPower = 0
	limiter.allowRate = allowRate
}

func (limiter *VPowerLimiter) CheckLimit(from, to types.Address, power int64, add bool) xerrors.XError {
	if xerr := limiter.checkTotalPower(power, add); xerr != nil {
		return xerr
	}

	if add {
		limiter.newTotalPower += power
		limiter.addingPower += power
	} else {
		limiter.newTotalPower -= power
		limiter.subingPower += power
	}
	return nil
}

// checkTotalPower returns error when the newly added power is greater than the new total power.
// After any changes—whether some amount is added or removed—the new total should be seen as: newly added amount + remaining(previously existing) amount.
// To ensure stability, the existing amount must still represent at least 'X%' of the total after the change.
// Therefore, the newly added amount is only allowed if it stays within '(100 - X)%' of the new total.
func (limiter *VPowerLimiter) checkTotalPower(diff int64, add bool) xerrors.XError {
	var rate int32
	if add {
		rate = changeRate(limiter.addingPower+diff, limiter.newTotalPower+diff)
	} else if limiter.newTotalPower >= diff {
		//
		rate = changeRate(limiter.addingPower, limiter.newTotalPower-diff)
	} else {
		return xerrors.ErrOverFlow.Wrapf("total power(%v) > diff(%v)", limiter.newTotalPower, diff)
	}

	if rate >= limiter.allowRate {
		return xerrors.ErrUpdatableStakeRatio.Wrapf("expected total power(%v) expected change power(%v)", limiter.newTotalPower, limiter.addingPower+diff)
	}
	return nil
}

func changeRate(part, total int64) int32 {
	return int32(part * 100 / total)
}
