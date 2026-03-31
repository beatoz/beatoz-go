package vpower

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
)

type OP_POWER bool

const (
	ADD_POWER OP_POWER = true
	SUB_POWER OP_POWER = false
)

// VPowerLimiter limits the change of voting power in one block.
// It limits a stakeholder’s stake change to one-third of their current holdings.
// And, it also limits total voting power changes to one-third of the **changed** total.
type VPowerLimiter struct {
	lastTotalPower int64

	estimatedTotalPower int64
	addingPower         int64
	subingPower         int64

	allowRate int32
}

func NewVPowerLimiter() *VPowerLimiter {
	return &VPowerLimiter{}
}

func (limiter *VPowerLimiter) Reset(total int64, allowRate int32) {
	limiter.lastTotalPower = total
	limiter.estimatedTotalPower = total
	limiter.addingPower = 0
	limiter.subingPower = 0
	limiter.allowRate = allowRate
}

func (limiter *VPowerLimiter) CheckLimit(power int64, op OP_POWER) xerrors.XError {
	if xerr := limiter.checkTotalPower(power, op); xerr != nil {
		return xerr
	}

	if op == ADD_POWER {
		limiter.estimatedTotalPower += power
		limiter.addingPower += power
	} else {
		limiter.estimatedTotalPower -= power
		limiter.subingPower += power
	}
	return nil
}

// checkTotalPower checks whether the voting power change is within the allowed rate.
// It calculates the new total voting power after applying the `diff`,
// then verifies that the ratio of remaining power to the new total does not
// exceed `limiter.allowRate`. If the change rate exceeds the allowed rate, it returns an error.
func (limiter *VPowerLimiter) checkTotalPower(diff int64, op OP_POWER) xerrors.XError {
	remainTotal, newTotal := limiter.lastTotalPower-limiter.subingPower, limiter.estimatedTotalPower

	if op == SUB_POWER && remainTotal <= diff {
		return xerrors.ErrOverFlow.Wrapf("remained total power (%v) > diff (%v)", remainTotal, diff)
	}

	if op == ADD_POWER {
		newTotal = newTotal + diff
	} else {
		remainTotal, newTotal = remainTotal-diff, newTotal-diff
	}

	remainRate := int32(remainTotal * 100 / newTotal)

	if (100 - remainRate) > limiter.allowRate {
		return xerrors.ErrUpdatableStakeRatio.Wrapf(
			"remainedTotalPower(%v) / estimatedTotalPower(%v) = remainRate(%v%%) => changeRate(%v) >= allowedRate(%v%%)",
			remainTotal, newTotal, remainRate, (100 - remainRate), limiter.allowRate)
	}
	return nil
}

// DEPRECATED
func (limiter *VPowerLimiter) ChangeRate(power int64, op OP_POWER) (int32, xerrors.XError) {
	var rate int32
	if op == ADD_POWER {
		limiter.estimatedTotalPower += power
		limiter.addingPower += power
		rate = changeRate(limiter.addingPower, limiter.estimatedTotalPower)
	} else if limiter.estimatedTotalPower >= power {
		limiter.estimatedTotalPower -= power
		limiter.subingPower += power
		rate = changeRate(limiter.addingPower, limiter.estimatedTotalPower)
	} else {
		return 0, xerrors.ErrOverFlow.Wrapf("estimatedTotalPower(%v) > subtractedPower(%v)", limiter.estimatedTotalPower, power)
	}
	return rate, nil
}

// DEPRECATED
func changeRate(part, total int64) int32 {
	return int32(part * 100 / total)
}
