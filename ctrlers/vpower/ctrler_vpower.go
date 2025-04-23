package vpower

import (
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

func (ctrler *VPowerCtrler) bondPowerChunk(
	dgtee *DelegateeV1,
	vpow *VPower,
	power int64,
	height int64,
	txhash bytes.HexBytes,
	exec bool) xerrors.XError {

	_ = vpow.addPowerWithTxHash(power, height, txhash)
	if xerr := ctrler.vpowsLedger.Set(vpow.Key(), vpow, exec); xerr != nil {
		return xerr
	}

	dgtee.addPower(vpow.From, power)
	dgtee.addDelegator(vpow.From)
	if xerr := ctrler.dgteesLedger.Set(dgtee.Key(), dgtee, exec); xerr != nil {
		return xerr
	}
	return nil
}

func (ctrler *VPowerCtrler) unbondPowerChunk(dgtee *DelegateeV1, vpow *VPower, txhash bytes.HexBytes, exec bool) (*PowerChunk, xerrors.XError) {
	// delete the power chunk with `txhash`
	var pc = vpow.delPowerWithTxHash(txhash)
	if pc == nil {
		return nil, xerrors.ErrNotFoundStake.Wrapf("validator(%v) has no power chunk(txhash:%v) from %v", dgtee.addr, txhash, vpow.From)
	}
	// decrease the power of `dgteeProto` by `pc.Power`
	dgtee.delPower(vpow.From, pc.Power)
	if len(vpow.PowerChunks) == 0 {
		dgtee.delDelegator(vpow.From)
		if xerr := ctrler.vpowsLedger.Del(vpow.Key(), exec); xerr != nil {
			return nil, xerr
		}
	} else if xerr := ctrler.vpowsLedger.Set(vpow.Key(), vpow, exec); xerr != nil {
		return nil, xerr
	}
	if xerr := ctrler.dgteesLedger.Set(dgtee.Key(), dgtee, exec); xerr != nil {
		return nil, xerr
	}
	return pc, nil
}
