package vpower

import (
	"context"
	"fmt"
	"sort"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ledger/common"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	btztypes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func (ctrler *VPowerCtrler) Query(req abcitypes.RequestQuery, opts ...ctrlertypes.Option) ([]byte, xerrors.XError) {
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
	ctx, cancel := context.WithTimeout(context.Background(), ctrler.queryTimeout)
	defer cancel()

	type respStake struct {
		From        types.Address     `json:"owner"`
		To          types.Address     `json:"to"`
		TxHash      btztypes.HexBytes `json:"txhash"`
		StartHeight int64             `json:"startHeight,string"`
		Power       int64             `json:"power,string"`
	}
	var ret []*respStake

	atledger, xerr := ctrler.vpowerState.ImitableLedgerAt(height)
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	xerr = atledger.Seek(common.LedgerKeyVPower(addr, nil), true, func(key common.LedgerKey, item common.ILedgerItem) xerrors.XError {
		vpow, _ := item.(*VPower)
		for _, pc := range vpow.PowerChunks {
			if err := ctx.Err(); err != nil {
				return xerrors.ErrQuery.Wrap(err)
			}
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
	if err := ctx.Err(); err != nil {
		return nil, xerrors.ErrQuery.Wrap(err)
	}
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	bz, err := jsonx.Marshal(ret)
	if err != nil {
		return nil, xerrors.ErrQuery.Wrap(err)
	}
	return bz, nil
}

func (ctrler *VPowerCtrler) queryDelegatee(height int64, addr types.Address) ([]byte, xerrors.XError) {
	ctx, cancel := context.WithTimeout(context.Background(), ctrler.queryTimeout)
	defer cancel()

	type respStake struct {
		From        types.Address     `json:"owner"`
		To          types.Address     `json:"to"`
		TxHash      btztypes.HexBytes `json:"txhash"`
		StartHeight int64             `json:"startHeight,string"`
		Power       int64             `json:"power,string"`
	}

	type respDelegatee struct {
		Addr                types.Address     `json:"address"`
		PubKey              btztypes.HexBytes `json:"pubKey"`
		SelfPower           int64             `json:"selfPower,string"`
		TotalPower          int64             `json:"totalPower,string"`
		SlashedPower        int64             `json:"slashedPower,string"`
		Delegators          []types.Address   `json:"delegators"`
		NotSignedBlockCount int64             `json:"notSingedBlockCount,string"`
		// DEPRECATED: only for backward compatibility
		Stakes []*respStake `json:"stakes,omitempty"`
		// DEPRECATED: only for backward compatibility
		NotSignedHeights interface{} `json:"notSignedBlocks,omitempty"`
	}

	var ret *respDelegatee

	atledger, xerr := ctrler.vpowerState.ImitableLedgerAt(height)
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	bc, xerr := atledger.Get(common.LedgerKeyMissedBlockCount(addr))
	if xerr != nil && !xerr.Contains(xerrors.ErrNotFoundResult) {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}
	_ptr, _ := bc.(*BlockCount)
	n := _ptr.Int64()

	item, xerr := atledger.Get(common.LedgerKeyDelegatee(addr))
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	dgtee, _ := item.(*Delegatee)
	dgtors := make([]types.Address, len(dgtee.Delegators))
	var stakes []*respStake
	for i, _addr := range dgtee.Delegators {
		if err := ctx.Err(); err != nil {
			return nil, xerrors.ErrQuery.Wrap(err)
		}
		dgtors[i] = _addr

		item, xerr = atledger.Get(common.LedgerKeyVPower(_addr, dgtee.addr))
		if xerr != nil {
			return nil, xerrors.ErrQuery.Wrap(xerr)
		}
		vpow, _ := item.(*VPower)
		for _, pc := range vpow.PowerChunks {
			if err := ctx.Err(); err != nil {
				return nil, xerrors.ErrQuery.Wrap(err)
			}
			stakes = append(stakes, &respStake{
				From:        vpow.from,
				To:          vpow.to,
				TxHash:      pc.TxHash,
				StartHeight: pc.Height,
				Power:       pc.Power,
			})
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, xerrors.ErrQuery.Wrap(err)
	}
	ret = &respDelegatee{
		Addr:                dgtee.addr,
		PubKey:              dgtee.PubKey,
		SelfPower:           dgtee.SelfPower,
		TotalPower:          dgtee.SumPower,
		SlashedPower:        0, // todo: Add slashed power data to Delegatee
		Delegators:          dgtors,
		NotSignedBlockCount: n,
		Stakes:              stakes,
		NotSignedHeights:    nil,
	}

	bz, err := jsonx.Marshal(ret)
	if err != nil {
		return nil, xerrors.ErrQuery.Wrap(err)
	}
	return bz, nil
}

func (ctrler *VPowerCtrler) queryTotalPower(height int64) ([]byte, xerrors.XError) {
	atledger, xerr := ctrler.vpowerState.ImitableLedgerAt(height)
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	ret := int64(0)
	xerr = atledger.Seek(common.KeyPrefixDelegatee, true, func(key common.LedgerKey, item common.ILedgerItem) xerrors.XError {
		d, _ := item.(*Delegatee)
		ret += d.SumPower
		return nil
	})
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}
	return []byte(fmt.Sprintf("\"%v\"", ret)), nil
}

// queryVotingPower returns the sum of voting power of validators.
func (ctrler *VPowerCtrler) queryVotingPower(height int64, getMaxValCnt, getMinValPower ctrlertypes.Option) ([]byte, xerrors.XError) {
	atledger, xerr := ctrler.vpowerState.ImitableLedgerAt(height)
	if xerr != nil {
		return nil, xerrors.ErrQuery.Wrap(xerr)
	}

	maxValCnt := getMaxValCnt().(int32)
	minValPower := getMinValPower().(int64)

	var delegatees OrderByPowerDelegatees
	xerr = atledger.Seek(common.KeyPrefixDelegatee, true, func(key common.LedgerKey, item common.ILedgerItem) xerrors.XError {
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

	n := libs.MinInt(len(delegatees), int(maxValCnt))
	validators := delegatees[:n]

	retPower := int64(0)
	for _, v := range validators {
		retPower += v.SumPower
	}
	return []byte(fmt.Sprintf("\"%v\"", retPower)), nil
}
