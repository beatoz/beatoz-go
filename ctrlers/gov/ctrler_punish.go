package gov

import (
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

// doPunish slashes the voting power of the byzantine validator(voter).
// If the voter has already voted, it will be canceled.
// This function is called from BeatozApp::BeginBlock.
func (ctrler *GovCtrler) doPunish(targetAddr types.Address) (int64, xerrors.XError) {
	slashedPower := int64(0)

	_ = ctrler.proposalState.Seek(v1.KeyPrefixProposal, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		prop, _ := item.(*proposal.GovProposal)
		for _, v := range prop.Voters {
			if bytes.Compare(v.Addr, targetAddr) == 0 {
				// the voting power of `targetAddr` will be slashed and
				// the vote of `targetAddr` will be canceled.
				slashed, _ := prop.DoPunish(targetAddr, ctrler.SlashRate())
				slashedPower += slashed

				if xerr := ctrler.proposalState.Set(v1.LedgerKeyProposal(prop.TxHash), prop, true); xerr != nil {
					return xerr
				}
				break
			}
		}
		return nil
	}, true)

	return slashedPower, nil
}
