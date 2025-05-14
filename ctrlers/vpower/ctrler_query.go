package vpower

import (
	"fmt"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/types"
	btztypes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"sort"
)

func (ctrler *VPowerCtrler) Query(req abcitypes.RequestQuery, opts ...ctrlertypes.Option) ([]byte, xerrors.XError) {
	//TODO implement me
	ctrler.mtx.RLock()
	defer ctrler.mtx.RUnlock()

	switch req.Path {
	case "stakes":
		return ctrler.queryStakes(req.Height, req.Data)
	case "delegatee":
		return ctrler.queryDelegatee(req.Height, req.Data)
	case "stakes/total_power":
		return ctrler.queryTotalPower(req.Height)
	case "stakes/voting_power":
		return ctrler.queryVotingPower(req.Height, opts[0], opts[1])
	default:
		return nil, xerrors.ErrQuery.Wrapf("unknown query path")
	}
}

func (ctrler *VPowerCtrler) queryStakes(height int64, addr types.Address) ([]byte, xerrors.XError) {
	type respStake struct {
		From        types.Address     `json:"owner"`
		To          types.Address     `json:"to"`
		TxHash      btztypes.HexBytes `json:"txhash"`
		StartHeight int64             `json:"startHeight,string"`
		//RefundHeight int64           `json:"refundHeight,string"`
		Power int64 `json:"power,string"`
	}
	var ret []*respStake

	atledger, xerr := ctrler.powersState.ImitableLedgerAt(height)
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	xerr = atledger.Seek(v1.LedgerKeyVPower(addr, nil), true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		vpow, _ := item.(*VPower)
		for _, pc := range vpow.PowerChunks {
			ret = append(ret, &respStake{
				From:        vpow.from,
				To:          vpow.to,
				TxHash:      pc.TxHash,
				StartHeight: pc.Height,
				Power:       pc.Power,
			})
		}
		return nil
	})
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	bz, err := tmjson.Marshal(ret)
	if err != nil {
		return nil, xerrors.ErrQuery.Wrap(err)
	}
	return bz, nil
}

func (ctrler *VPowerCtrler) queryDelegatee(height int64, addr types.Address) ([]byte, xerrors.XError) {
	type respDelegatee struct {
		Addr                types.Address     `json:"address"`
		PubKey              btztypes.HexBytes `json:"pubKey"`
		SelfPower           int64             `json:"selfPower,string"`
		TotalPower          int64             `json:"totalPower,string"`
		SlashedPower        int64             `json:"slashedPower,string"`
		Delegators          []types.Address   `json:"delegators"`
		NotSignedBlockCount int64             `json:"notSingedBlockCount,string"`
		// DEPRECATED: only for backward compatibility
		Stakes []interface{} `json:"stakes,omitempty"`
		// DEPRECATED: only for backward compatibility
		NotSignedHeights interface{} `json:"notSignedBlocks,omitempty"`
	}

	var ret *respDelegatee

	atledger, xerr := ctrler.powersState.ImitableLedgerAt(height)
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	bc, xerr := atledger.Get(v1.LedgerKeyMissedBlockCount(addr))
	if xerr != nil && !xerr.Contains(xerrors.ErrNotFoundResult) {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}
	_ptr, _ := bc.(*BlockCount)
	n := _ptr.Int64()

	item, xerr := atledger.Get(v1.LedgerKeyDelegatee(addr))
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	dgtee, _ := item.(*Delegatee)
	_delegators := make([]types.Address, len(dgtee.Delegators))
	for i, dg := range dgtee.Delegators {
		_delegators[i] = dg
	}
	ret = &respDelegatee{
		Addr:                dgtee.addr,
		PubKey:              dgtee.PubKey,
		SelfPower:           dgtee.SelfPower,
		TotalPower:          dgtee.SumPower,
		SlashedPower:        0, // todo: Add slashed power data to Delegatee
		Delegators:          _delegators,
		NotSignedBlockCount: n,
		Stakes:              nil,
		NotSignedHeights:    nil,
	}

	bz, err := tmjson.Marshal(ret)
	if err != nil {
		return nil, xerrors.ErrQuery.Wrap(err)
	}
	return bz, nil
}

func (ctrler *VPowerCtrler) queryTotalPower(height int64) ([]byte, xerrors.XError) {
	atledger, xerr := ctrler.powersState.ImitableLedgerAt(height)
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	ret := int64(0)
	xerr = atledger.Seek(v1.KeyPrefixDelegatee, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		d, _ := item.(*Delegatee)
		ret += d.SumPower
		return nil
	})
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}
	return []byte(fmt.Sprintf("%v", ret)), nil
}

func (ctrler *VPowerCtrler) queryVotingPower(height int64, getMaxValCnt, getMinValPower ctrlertypes.Option) ([]byte, xerrors.XError) {
	atledger, xerr := ctrler.powersState.ImitableLedgerAt(height)
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	maxValCnt := getMaxValCnt().(int)
	minValPower := getMinValPower().(int64)

	var delegatees orderByPowerDelegatees
	xerr = atledger.Seek(v1.KeyPrefixDelegatee, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		d, _ := item.(*Delegatee)
		if d.SelfPower < minValPower {
			return nil // continue
		}
		delegatees = append(delegatees, d)
		return nil
	})
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}
	sort.Sort(delegatees)

	n := libs.MinInt(len(delegatees), maxValCnt)
	validators := delegatees[:n]

	retPower := int64(0)
	for _, v := range validators {
		retPower += v.SumPower
	}
	return []byte(fmt.Sprintf("%v", retPower)), nil
}
