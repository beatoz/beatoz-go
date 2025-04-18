package vpower

import (
	"fmt"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"sort"
	"sync"
)

// VPowerLimiter limits the amount of stake changes in one block.
type VPowerLimiter struct {
	individualLimitRatio int64
	updatableLimitRatio  int64
	maxValidatorCnt      int64

	powerObjs      []*powerObj
	baseTotalPower int64

	updatedPower int64
	handledAddrs map[[20]byte]int

	mtx sync.RWMutex
}

func NewVPowerLimiter(vals DelegateeArray, maxValCnt, indiLimitRatio, upLimitRatio int64) *VPowerLimiter {
	ret := &VPowerLimiter{}
	ret.Reset(vals, maxValCnt, indiLimitRatio, upLimitRatio)
	return ret
}

func (limiter *VPowerLimiter) Reset(vals DelegateeArray, maxValCnt, indiLimitRatio, upLimitRatio int64) {
	limiter.mtx.Lock()
	defer limiter.mtx.Unlock()

	limiter.reset(vals, maxValCnt, indiLimitRatio, upLimitRatio)
}

func (limiter *VPowerLimiter) reset(vals DelegateeArray, maxValCnt, indiLimitRatio, upLimitRatio int64) {
	_base := int64(0)
	var pobjs []*powerObj
	for i, v := range vals {
		pobjs = append(pobjs, &powerObj{
			Addr:  v.addr,
			Power: v.totalPower,
		})

		if int64(i) < maxValCnt {
			_base += v.totalPower
		}
	}

	limiter.powerObjs = pobjs
	limiter.baseTotalPower = _base
	limiter.individualLimitRatio = indiLimitRatio
	limiter.updatableLimitRatio = upLimitRatio
	limiter.maxValidatorCnt = maxValCnt

	limiter.updatedPower = 0
}

func (limiter *VPowerLimiter) findPowerObj(addr types.Address) (int, *powerObj) {
	for i, o := range limiter.powerObjs {
		if o.Addr.Compare(addr) == 0 {
			return i, o
		}
	}
	return -1, nil
}

// checkIndividualPowerLimit checks if `val`'s changed total power ratio is greater than `individualLimitRatio` of the total power.
// This is to prevent the power of any one validator from exceeding the `individualLimitRatio`% of the total power.
// NOTE: However, when `diffPower` is negative (un-bonding), the power of another validator may exceed `individualLimitRatio`% of the total power.
func (limiter *VPowerLimiter) checkIndividualPowerLimit(val *Delegatee, diffPower int64) xerrors.XError {
	if diffPower <= 0 {
		// not check
		return nil
	}

	individualRatio := (val.totalPower + diffPower) * int64(100) / (limiter.baseTotalPower + diffPower)

	if individualRatio > limiter.individualLimitRatio {
		return xerrors.From(
			fmt.Errorf("VPowerLimiter error: exceeding individual power limit - delegatee(%v), power(%v), diff:%v, base(%v), ratio(%v), limit(%v)",
				val.addr, val.totalPower, diffPower, limiter.baseTotalPower, individualRatio, limiter.individualLimitRatio))
	}

	return nil
}

// checkIndividualPowerLimit checks whether the sum of power change caused by `diffPower`
// exceeds `updatableLimitRatio`% of the total power of the existing validator set.
// todo: Implement again.
func (limiter *VPowerLimiter) checkUpdatablePowerLimit(val *Delegatee, diffPower int64) xerrors.XError {
	ridx, powObj := limiter.findPowerObj(val.addr)
	if powObj == nil {
		// `delg` is new face
		powObj = &powerObj{
			Addr:  val.addr,
			Power: val.totalPower,
		}
		// when the val is new face, the added power should be ignored at this time.
		//ridx = len(limiter.powerObjs)
		//limiter.powerObjs = append(limiter.powerObjs, powObj)
	}

	updatedPower := limiter.updatedPower

	if powObj.Power != val.totalPower {
		return xerrors.From(fmt.Errorf("VPowerLimiter's power(%v) object is not equal to the power(%v) of validator(%v)",
			powObj.Power, val.totalPower, val.addr))
	}

	if ridx >= 0 && ridx < int(limiter.maxValidatorCnt) && diffPower < 0 {
		// the `val` is already validator and un-staking
		var candidate *powerObj
		if len(limiter.powerObjs) > int(limiter.maxValidatorCnt) {
			candidate = limiter.powerObjs[limiter.maxValidatorCnt]
		}

		// `diffPower` is negative
		if candidate != nil && powObj.Power+diffPower < candidate.Power {
			// the `val` will be removed from validator set.
			// so, the power of `val` is included into updatedPower.
			updatedPower += powObj.Power
		} else {
			updatedPower += -1 * diffPower
		}
	}

	if (ridx < 0 || ridx >= int(limiter.maxValidatorCnt)) && diffPower > 0 {
		// the `val` is not validator and staking
		var lastVal *powerObj
		if len(limiter.powerObjs) >= int(limiter.maxValidatorCnt) {
			lastVal = limiter.powerObjs[limiter.maxValidatorCnt-1]
		}

		if lastVal != nil && powObj.Power+diffPower > lastVal.Power {
			// the `lastVal` will be removed from validator set.
			// so, the power of `lastVal` is included into updatedPower.
			updatedPower += lastVal.Power
		}
	}

	_ratio := updatedPower * int64(100) / limiter.baseTotalPower
	if limiter.updatableLimitRatio < _ratio {
		// reject
		return xerrors.From(
			fmt.Errorf("VPowerLimiter error: Exceeding the updatable power limit. updated(%v), base(%v), ratio(%v), limit(%v)",
				updatedPower, limiter.baseTotalPower, _ratio, limiter.updatableLimitRatio))
	}

	powObj.Power += diffPower

	if powObj.Power < 0 {
		return xerrors.From(
			fmt.Errorf("VPowerLimiter error: power(%v) of %v is negative",
				powObj.Power, powObj.Addr))
	}
	limiter.updatedPower = updatedPower
	sort.Sort(orderedPowerObjs(limiter.powerObjs)) // sort by power
	return nil
}

func (limiter *VPowerLimiter) CheckLimit(val *Delegatee, changePower int64) xerrors.XError {
	limiter.mtx.Lock()
	defer limiter.mtx.Unlock()

	if limiter.powerObjs == nil {
		return nil
	}

	if xerr := limiter.checkIndividualPowerLimit(val, changePower); xerr != nil {
		return xerr
	}
	if xerr := limiter.checkUpdatablePowerLimit(val, changePower); xerr != nil {
		return xerr
	}
	return nil
}

type powerObj struct {
	Addr  types.Address
	Power int64
}

// descending order by Power
type orderedPowerObjs []*powerObj

func (objs orderedPowerObjs) Len() int {
	return len(objs)
}

func (objs orderedPowerObjs) Less(i, j int) bool {
	if objs[i].Power != objs[j].Power {
		return objs[i].Power > objs[j].Power
	}
	if bytes.Compare(objs[i].Addr, objs[j].Addr) > 0 {
		return true
	}
	return false
}

func (objs orderedPowerObjs) Swap(i, j int) {
	objs[i], objs[j] = objs[j], objs[i]
}

var _ sort.Interface = (orderedPowerObjs)(nil)
