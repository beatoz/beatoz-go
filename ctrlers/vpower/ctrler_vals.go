package vpower

import (
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/types/bytes"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"sort"
)

// UpdateValidators is called after executing staking/unstaking txs and before committing the result of the executing.
// `ctrler.allDelegatees` has delegatees committed at previous block.
// It means that UpdateValidators consider the stakes updated at the previous block, not the current block.
func (ctrler *VPowerCtrler) updateValidators(maxVals int) []abcitypes.ValidatorUpdate {

	newValidators := selectValidators(ctrler.allDelegatees, maxVals)
	upVals := validatorUpdates(ctrler.lastValidators, newValidators)

	ctrler.lastValidators = newValidators

	return upVals
}

// selectValidators returns the top maxVals delegatees, sorted in descending order of power.
// NOTE: The order of elements in `delegatees` are rearranged based on their power in descending order.
func selectValidators(delegatees DelegateeArray, maxVals int) DelegateeArray {
	sort.Sort(orderByPowerDelegatees(delegatees))
	ret := make(DelegateeArray, libs.MinInt(len(delegatees), maxVals))
	copy(ret, delegatees[:len(ret)])
	return ret
}

// validatorUpdates returns the difference between `lastVals` and `newVals`.
// NOTE: The elements in both lastVals and newVals are reordered by `addr` in ascending order.
func validatorUpdates(lastVals, newVals DelegateeArray) []abcitypes.ValidatorUpdate {
	valUpdates := make(abcitypes.ValidatorUpdates, 0, len(lastVals)+len(newVals))

	sort.Sort(orderByAddrDelegatees(lastVals))
	sort.Sort(orderByAddrDelegatees(newVals))

	i, j := 0, 0
	for i < len(lastVals) && j < len(newVals) {
		ret := bytes.Compare(lastVals[i].addr, newVals[j].addr)
		if ret < 0 {
			// this validator in `lastVals` will be removed because it is not included in `newVals`
			valUpdates = append(valUpdates, abcitypes.UpdateValidator(lastVals[i].pubKey, 0, "secp256k1"))
			i++
		} else if ret == 0 {
			if lastVals[i].totalPower != newVals[j].totalPower {
				// if power is changed, add newer who has updated power
				valUpdates = append(valUpdates, abcitypes.UpdateValidator(newVals[j].pubKey, int64(newVals[j].totalPower), "secp256k1"))
			} else {
				// if the power is not changed, exclude the validator in updated validators
			}
			i++
			j++
		} else { // ret > 0
			valUpdates = append(valUpdates, abcitypes.UpdateValidator(newVals[j].pubKey, int64(newVals[j].totalPower), "secp256k1"))
			j++
		}
	}

	for ; i < len(lastVals); i++ {
		// removed
		valUpdates = append(valUpdates, abcitypes.UpdateValidator(lastVals[i].pubKey, 0, "secp256k1"))
	}
	for ; j < len(newVals); j++ {
		// added newer
		valUpdates = append(valUpdates, abcitypes.UpdateValidator(newVals[j].pubKey, int64(newVals[j].totalPower), "secp256k1"))
	}

	return valUpdates
}

type orderByPowerDelegatees []*Delegatee

func (dgtees orderByPowerDelegatees) Len() int {
	return len(dgtees)
}

// descending order by TotalPower
func (dgtees orderByPowerDelegatees) Less(i, j int) bool {
	if dgtees[i].totalPower != dgtees[j].totalPower {
		return dgtees[i].totalPower > dgtees[j].totalPower
	}
	if dgtees[i].SelfPower() != dgtees[j].SelfPower() {
		return dgtees[i].SelfPower() > dgtees[j].SelfPower()
	}
	if dgtees[i].sumMaturePower != dgtees[j].sumMaturePower {
		return dgtees[i].sumMaturePower > dgtees[j].sumMaturePower
	}
	if dgtees[i].sumOfBlocks != dgtees[j].sumOfBlocks {
		return dgtees[i].sumOfBlocks > dgtees[j].sumOfBlocks
	}
	if bytes.Compare(dgtees[i].addr, dgtees[j].addr) > 0 {
		return true
	}
	return false
}

func (dgtees orderByPowerDelegatees) Swap(i, j int) {
	dgtees[i], dgtees[j] = dgtees[j], dgtees[i]
}

var _ sort.Interface = (orderByPowerDelegatees)(nil)

type orderByAddrDelegatees []*Delegatee

func (dgtees orderByAddrDelegatees) Len() int {
	return len(dgtees)
}

// ascending order by address
func (dgtees orderByAddrDelegatees) Less(i, j int) bool {
	return bytes.Compare(dgtees[i].addr, dgtees[j].addr) < 0
}

func (dgtees orderByAddrDelegatees) Swap(i, j int) {
	dgtees[i], dgtees[j] = dgtees[j], dgtees[i]
}

var _ sort.Interface = (orderByAddrDelegatees)(nil)
