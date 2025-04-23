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
func selectValidators(delegatees []*DelegateeV1, maxVals int) []*DelegateeV1 {
	sort.Sort(orderByPowerDelegateeV1(delegatees))
	ret := make([]*DelegateeV1, libs.MinInt(len(delegatees), maxVals))
	copy(ret, delegatees[:len(ret)])
	return ret
}

// validatorUpdates returns the difference between `lastVals` and `newVals`.
// NOTE: The elements in both lastVals and newVals are reordered by `addr` in ascending order.
func validatorUpdates(lastVals, newVals []*DelegateeV1) []abcitypes.ValidatorUpdate {
	valUpdates := make(abcitypes.ValidatorUpdates, 0, len(lastVals)+len(newVals))

	sort.Sort(orderByPubDelegateeV1(lastVals))
	sort.Sort(orderByPubDelegateeV1(newVals))

	i, j := 0, 0
	for i < len(lastVals) && j < len(newVals) {
		ret := bytes.Compare(lastVals[i].PubKey, newVals[j].PubKey)
		if ret < 0 {
			// this validator in `lastVals` will be removed because it is not included in `newVals`
			valUpdates = append(valUpdates, abcitypes.UpdateValidator(lastVals[i].PubKey, 0, "secp256k1"))
			i++
		} else if ret == 0 {
			if lastVals[i].TotalPower != newVals[j].TotalPower {
				// if power is changed, add newer who has updated power
				valUpdates = append(valUpdates, abcitypes.UpdateValidator(newVals[j].PubKey, int64(newVals[j].TotalPower), "secp256k1"))
			} else {
				// if the power is not changed, exclude the validator in updated validators
			}
			i++
			j++
		} else { // ret > 0
			valUpdates = append(valUpdates, abcitypes.UpdateValidator(newVals[j].PubKey, int64(newVals[j].TotalPower), "secp256k1"))
			j++
		}
	}

	for ; i < len(lastVals); i++ {
		// removed
		valUpdates = append(valUpdates, abcitypes.UpdateValidator(lastVals[i].PubKey, 0, "secp256k1"))
	}
	for ; j < len(newVals); j++ {
		// added newer
		valUpdates = append(valUpdates, abcitypes.UpdateValidator(newVals[j].PubKey, int64(newVals[j].TotalPower), "secp256k1"))
	}

	return valUpdates
}
