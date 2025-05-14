package vpower

import (
	"encoding/binary"
	types2 "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

func (ctrler *VPowerCtrler) loadDelegatees(exec bool) ([]*Delegatee, xerrors.XError) {
	var dgtees []*Delegatee
	xerr := ctrler.powersState.Seek(v1.KeyPrefixDelegatee, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		dgtee, _ := item.(*Delegatee)
		dgtees = append(dgtees, dgtee)
		return nil
	}, exec)
	if xerr != nil {
		return nil, xerr
	}
	return dgtees, nil
}

func (ctrler *VPowerCtrler) readDelegatee(addr types.Address, exec bool) (*Delegatee, xerrors.XError) {
	var ret *Delegatee
	item, xerr := ctrler.powersState.Get(v1.LedgerKeyDelegatee(addr), exec)
	if xerr == nil {
		ret, _ = item.(*Delegatee)
	}
	return ret, xerr
}

func (ctrler *VPowerCtrler) delDelegatee(addr types.Address, exec bool) xerrors.XError {
	return ctrler.powersState.Del(v1.LedgerKeyDelegatee(addr), exec)
}

func (ctrler *VPowerCtrler) readVPower(from, to types.Address, exec bool) (*VPower, xerrors.XError) {
	var ret *VPower
	item, xerr := ctrler.powersState.Get(v1.LedgerKeyVPower(from, to), exec)
	if xerr == nil {
		ret, _ = item.(*VPower)
	}
	return ret, xerr
}

func (ctrler *VPowerCtrler) seekVPowersOf(from types.Address, cb v1.FuncIterate, exec bool) xerrors.XError {
	return ctrler.powersState.Seek(v1.LedgerKeyVPower(from, nil), true, cb, exec)
}

func (ctrler *VPowerCtrler) delVPower(from, to types.Address, exec bool) xerrors.XError {
	return ctrler.powersState.Del(v1.LedgerKeyVPower(from, to), exec)
}

func (ctrler *VPowerCtrler) BondPowerChunk(
	dgtee *Delegatee,
	vpow *VPower,
	power int64,
	height int64,
	txhash bytes.HexBytes,
	exec bool) xerrors.XError {

	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.bondPowerChunk(dgtee, vpow, power, height, txhash, exec)
}

func (ctrler *VPowerCtrler) bondPowerChunk(
	dgtee *Delegatee,
	vpow *VPower,
	power int64,
	height int64,
	txhash bytes.HexBytes,
	exec bool) xerrors.XError {

	_ = vpow.addPowerWithTxHash(power, height, txhash)
	if xerr := ctrler.powersState.Set(vpow.key, vpow, exec); xerr != nil {
		return xerr
	}

	dgtee.addPower(vpow.from, power)
	dgtee.addDelegator(vpow.from)

	if xerr := ctrler.powersState.Set(dgtee.key, dgtee, exec); xerr != nil {
		return xerr
	}
	return nil
}

func (ctrler *VPowerCtrler) UnbondPowerChunk(dgtee *Delegatee, vpow *VPower, txhash bytes.HexBytes, exec bool) (*PowerChunkProto, xerrors.XError) {
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	return ctrler.unbondPowerChunk(dgtee, vpow, txhash, exec)
}
func (ctrler *VPowerCtrler) unbondPowerChunk(dgtee *Delegatee, vpow *VPower, txhash bytes.HexBytes, exec bool) (*PowerChunkProto, xerrors.XError) {
	// delete the power chunk with `txhash`
	var pc = vpow.delPowerWithTxHash(txhash)
	if pc == nil {
		return nil, xerrors.ErrNotFoundStake.Wrapf("validator(%v) has no power chunk(txhash:%v) from %v", dgtee.addr, txhash, vpow.from)
	}
	// decrease the power of `dgteeProto` by `pc.Power`
	dgtee.delPower(vpow.from, pc.Power)

	if len(vpow.PowerChunks) == 0 {
		dgtee.delDelegator(vpow.from)
		if xerr := ctrler.powersState.Del(vpow.key, exec); xerr != nil {
			return nil, xerr
		}
	} else {
		if xerr := ctrler.powersState.Set(vpow.key, vpow, exec); xerr != nil {
			return nil, xerr
		}
	}

	if xerr := ctrler.powersState.Set(dgtee.key, dgtee, exec); xerr != nil {
		return nil, xerr
	}
	return pc, nil
}

func (ctrler *VPowerCtrler) freezePowerChunk(from types.Address, pc *PowerChunkProto, refundHeight int64, exec bool) xerrors.XError {
	item, xerr := ctrler.powersState.Get(v1.LedgerKeyFrozenVPower(refundHeight, from), exec)
	if xerr != nil && xerr != xerrors.ErrNotFoundResult {
		return xerr
	}
	if item == nil {
		// xerr is xerrors.ErrNotFoundResult
		item = newFrozenVPower(0)
	}

	frozen, _ := item.(*FrozenVPower)
	frozen.RefundPower += pc.Power
	frozen.PowerChunks = append(frozen.PowerChunks, pc)

	return ctrler.powersState.Set(v1.LedgerKeyFrozenVPower(refundHeight, from), frozen, exec)
}

func (ctrler *VPowerCtrler) unfreezePowerChunk(bctx *types2.BlockContext) xerrors.XError {
	return ctrler._unfreezePowerChunk(bctx.Height(), bctx.AcctHandler)
}

func (ctrler *VPowerCtrler) _unfreezePowerChunk(refundHeight int64, acctHandler types2.IAccountHandler) xerrors.XError {
	var removed []v1.LedgerKey
	defer func() {
		for _, k := range removed {
			_ = ctrler.powersState.Del(k, true)
		}
	}()

	return ctrler.powersState.Seek(
		v1.LedgerKeyFrozenVPower(refundHeight, nil),
		true,
		func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
			frozen, _ := item.(*FrozenVPower)
			refundAmt := types2.PowerToAmount(frozen.RefundPower)

			// key = prefix(1) | height(8) | from_address(20)
			from := key[9:29]

			xerr := acctHandler.Reward(from, refundAmt, true)
			if xerr != nil {
				return xerr
			}

			removed = append(removed, key)
			return nil
		}, true)
}

func (ctrler *VPowerCtrler) readFrozenVPower(refundHeight int64, from types.Address, exec bool) (*FrozenVPower, xerrors.XError) {
	var ret *FrozenVPower
	item, xerr := ctrler.powersState.Get(v1.LedgerKeyFrozenVPower(refundHeight, from), exec)
	if xerr == nil {
		ret, _ = item.(*FrozenVPower)
	}
	return ret, xerr
}

func (ctrler *VPowerCtrler) delFrozenVPower(refundHeight int64, from types.Address, exec bool) xerrors.XError {
	return ctrler.powersState.Del(v1.LedgerKeyFrozenVPower(refundHeight, from), exec)
}

func (ctrler *VPowerCtrler) countOf(keyPrefix []byte, exec bool) int {
	ret := 0
	_ = ctrler.powersState.Seek(keyPrefix, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		ret++
		return nil
	}, exec)
	return ret
}

type BlockCount int64

func (c *BlockCount) Encode() ([]byte, xerrors.XError) {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(*c))
	return b, nil
}

func (c *BlockCount) Decode(k, v []byte) xerrors.XError {
	*c = BlockCount(int64(binary.BigEndian.Uint64(v)))
	return nil
}

func (c *BlockCount) Add() {
	*c++
}

func (c *BlockCount) Int64() int64 {
	if c == nil {
		return int64(0)
	}
	return int64(*c)
}

var _ v1.ILedgerItem = (*BlockCount)(nil)

func (ctrler *VPowerCtrler) getMissedBlockCount(signer types.Address, exec bool) (BlockCount, xerrors.XError) {
	key := v1.LedgerKeyMissedBlockCount(signer)
	d, xerr := ctrler.powersState.Get(key, exec)
	if xerr != nil {
		return 0, xerr
	}
	c, _ := d.(*BlockCount)
	return *c, nil
}

func (ctrler *VPowerCtrler) setMissedBlockCount(signer types.Address, c BlockCount, exec bool) xerrors.XError {
	key := v1.LedgerKeyMissedBlockCount(signer)
	return ctrler.powersState.Set(key, &c, exec)
}

func (ctrler *VPowerCtrler) addMissedBlockCount(signer types.Address, exec bool) (BlockCount, xerrors.XError) {
	c, xerr := ctrler.getMissedBlockCount(signer, exec)
	if xerr != nil && !xerr.Contains(xerrors.ErrNotFoundResult) {
		return 0, xerr
	}

	// c is `0` when xerr is xerrors.ErrNotFoundResult
	c = c + 1
	return c, ctrler.setMissedBlockCount(signer, c, exec)
}

func (ctrler *VPowerCtrler) resetAllMissedBlockCount(exec bool) xerrors.XError {
	var rmKeys []v1.LedgerKey
	defer func() {
		for _, rmKey := range rmKeys {
			_ = ctrler.powersState.Del(rmKey, exec)
		}
	}()
	return ctrler.powersState.Seek(v1.KeyPrefixMissedBlockCount, true, func(key v1.LedgerKey, value v1.ILedgerItem) xerrors.XError {
		rmKeys = append(rmKeys, key)
		return nil
	}, exec)
}
