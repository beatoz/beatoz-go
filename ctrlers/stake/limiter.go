package stake

import (
	"fmt"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"sort"
	"sync"
)

// StakeLimiter limits the amount of stake changes in one block.
type StakeLimiter struct {
	individualLimitRatio int32
	updatableLimitRatio  int32
	maxValidatorCnt      int32

	powerObjs      []*powerObj
	baseTotalPower int64

	updatedPower int64
	handledAddrs map[[20]byte]int

	mtx sync.RWMutex
}

func NewStakeLimiter(vals PowerOrderDelegatees, maxValCnt, indiLimitRatio, upLimitRatio int32) *StakeLimiter {
	ret := &StakeLimiter{}
	ret.Reset(vals, maxValCnt, indiLimitRatio, upLimitRatio)
	return ret
}

func (sl *StakeLimiter) Reset(vals PowerOrderDelegatees, maxValCnt, indiLimitRatio, upLimitRatio int32) {
	sl.mtx.Lock()
	defer sl.mtx.Unlock()

	sl.reset(vals, maxValCnt, indiLimitRatio, upLimitRatio)
}

func (sl *StakeLimiter) reset(delgs PowerOrderDelegatees, maxValCnt, indiLimitRatio, upLimitRatio int32) {
	_base := int64(0)
	var pobjs []*powerObj
	for i, d := range delgs {
		pobjs = append(pobjs, &powerObj{
			Addr:  d.Addr,
			Power: d.TotalPower,
		})

		if int32(i) < maxValCnt {
			_base += d.TotalPower
		}
	}

	sl.powerObjs = pobjs
	sl.baseTotalPower = _base
	sl.individualLimitRatio = indiLimitRatio
	sl.updatableLimitRatio = upLimitRatio
	sl.maxValidatorCnt = maxValCnt

	sl.updatedPower = 0
}

func (sl *StakeLimiter) findPowerObj(addr types.Address) (int, *powerObj) {
	for i, o := range sl.powerObjs {
		if o.Addr.Compare(addr) == 0 {
			return i, o
		}
	}
	return -1, nil
}

func (sl *StakeLimiter) checkIndividualPowerLimit(delg *Delegatee, diffPower int64) xerrors.XError {
	if diffPower <= 0 {
		// not check
		return nil
	}

	individualRatio := (delg.TotalPower + diffPower) * int64(100) / (sl.baseTotalPower + diffPower)

	if individualRatio > int64(sl.individualLimitRatio) {
		return xerrors.From(
			fmt.Errorf("StakeLimiter error: exceeding individual power limit - delegatee(%v), power(%v), diff:%v, base(%v), ratio(%v), limit(%v)",
				delg.Addr, delg.TotalPower, diffPower, sl.baseTotalPower, individualRatio, sl.individualLimitRatio))
	}

	return nil
}

func (sl *StakeLimiter) checkUpdatablePowerLimit(delg *Delegatee, diffPower int64) xerrors.XError {
	ridx, powObj := sl.findPowerObj(delg.Addr)
	if powObj == nil {
		// `delg` is new face
		powObj = &powerObj{
			Addr:  delg.Addr,
			Power: delg.TotalPower,
		}
		//ridx = len(sl.powerObjs)
		//sl.powerObjs = append(sl.powerObjs, powObj)
	}

	updatedPower := sl.updatedPower

	if powObj.Power != delg.TotalPower {
		return xerrors.From(fmt.Errorf("StakeLimiter's power(%v) object is not equal to delegatee's power(%v). delegatee: %v",
			powObj.Power, delg.TotalPower, delg.Addr))
	}

	if ridx >= 0 && ridx < int(sl.maxValidatorCnt) && diffPower < 0 {
		// already validator and un-staking

		var candidate *powerObj
		if len(sl.powerObjs) > int(sl.maxValidatorCnt) {
			candidate = sl.powerObjs[sl.maxValidatorCnt]
		}

		// `diffPower` is negative
		if candidate != nil && powObj.Power+diffPower < candidate.Power {
			// `valObj` will be removed from validator set.
			updatedPower += powObj.Power
		} else {
			updatedPower += -1 * diffPower
		}
	}

	if (ridx < 0 || ridx >= int(sl.maxValidatorCnt)) && diffPower > 0 {
		var lastVal *powerObj
		if len(sl.powerObjs) >= int(sl.maxValidatorCnt) {
			lastVal = sl.powerObjs[sl.maxValidatorCnt-1]
		}

		if lastVal != nil && powObj.Power+diffPower > lastVal.Power {
			updatedPower += lastVal.Power
		}
	}

	_ratio := updatedPower * int64(100) / sl.baseTotalPower
	if int64(sl.updatableLimitRatio) < _ratio {
		// reject
		return xerrors.From(
			fmt.Errorf("StakeLimiter error: Exceeding the updatable power limit. updated(%v), base(%v), ratio(%v), limit(%v)",
				updatedPower, sl.baseTotalPower, _ratio, sl.updatableLimitRatio))
	}

	powObj.Power += diffPower

	if powObj.Power < 0 {
		return xerrors.From(
			fmt.Errorf("StakeLimiter error: power(%v) of %v is negative",
				powObj.Power, powObj.Addr))
	}
	sl.updatedPower = updatedPower
	sort.Sort(orderedPowerObj(sl.powerObjs)) // sort by power
	return nil
}

func (sl *StakeLimiter) CheckLimit(delg *Delegatee, changePower int64) xerrors.XError {
	sl.mtx.Lock()
	defer sl.mtx.Unlock()

	if sl.powerObjs == nil {
		return nil
	}

	if xerr := sl.checkIndividualPowerLimit(delg, changePower); xerr != nil {
		return xerr
	}
	if xerr := sl.checkUpdatablePowerLimit(delg, changePower); xerr != nil {
		return xerr
	}
	return nil
}

type powerObj struct {
	Addr  types.Address
	Power int64
}

type orderedPowerObj []*powerObj

func (objs orderedPowerObj) Len() int {
	return len(objs)
}

// descending order by TotalPower
func (objs orderedPowerObj) Less(i, j int) bool {
	if objs[i].Power != objs[j].Power {
		return objs[i].Power > objs[j].Power
	}
	if bytes.Compare(objs[i].Addr, objs[j].Addr) > 0 {
		return true
	}
	return false
}

func (objs orderedPowerObj) Swap(i, j int) {
	objs[i], objs[j] = objs[j], objs[i]
}

var _ sort.Interface = (orderedPowerObj)(nil)
