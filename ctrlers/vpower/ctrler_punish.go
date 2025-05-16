package vpower

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

// doPunish is executed at BeginBlock
func (ctrler *VPowerCtrler) doPunish(targetAddr types.Address, slashRate int32) (int64, xerrors.XError) {
	dgtee, xerr := ctrler.readDelegatee(targetAddr, true)
	if xerr != nil {
		return 0, xerr
	}

	//Punish the delegators as well as validator. issue #51
	slashedPower := int64(0)
	for _, addr := range dgtee.Delegators {
		vpow, xerr := ctrler.readVPower(addr, dgtee.addr, true)
		if xerr != nil {
			return 0, xerr
		}

		for i := len(vpow.PowerChunks) - 1; i >= 0; i-- {
			pc := vpow.PowerChunks[i]
			slashed := (pc.Power * int64(slashRate)) / 100
			pc.Power -= slashed

			vpow.SumPower -= slashed
			dgtee.SumPower -= slashed
			if bytes.Equal(addr, vpow.to) {
				dgtee.SelfPower -= slashed
			}

			slashedPower += slashed
		}
		if xerr = ctrler.writeVPower(vpow, true); xerr != nil {
			return 0, xerr
		}
	}

	if xerr = ctrler.writeDelegatee(dgtee, true); xerr != nil {
		return 0, xerr
	}
	return slashedPower, nil
}
