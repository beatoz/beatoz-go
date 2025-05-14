package vpower

import (
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// doPunish is executed at BeginBlock
func (ctrler *VPowerCtrler) doPunish(evi *abcitypes.Evidence, slashRate int32) (int64, xerrors.XError) {
	dgtee, xerr := ctrler.readDelegatee(evi.Validator.Address, true)
	if xerr != nil {
		return 0, xerr
	}

	//Punish the delegators as well as validator. issue #51
	slashedPower, slashedSumPower, slashedSelfPower := int64(0), int64(0), int64(0)
	for _, addr := range dgtee.Delegators {
		vpow, xerr := ctrler.readVPower(addr, dgtee.addr, true)
		if xerr != nil {
			return 0, xerr
		}

		for i := len(vpow.PowerChunks) - 1; i >= 0; i-- {
			pc := vpow.PowerChunks[i]
			slashed := (pc.Power * int64(slashRate)) / 100
			pc.Power -= slashed

			slashedPower += slashed
			slashedSumPower += pc.Power
			if bytes.Equal(addr, vpow.to) {
				slashedSelfPower += pc.Power
			}
		}
		if xerr = ctrler.writeVPower(vpow, true); xerr != nil {
			return 0, xerr
		}
	}

	dgtee.SumPower = slashedSumPower
	dgtee.SelfPower = slashedSelfPower
	if xerr = ctrler.writeDelegatee(dgtee, true); xerr != nil {
		return 0, xerr
	}
	return slashedPower, nil
}
