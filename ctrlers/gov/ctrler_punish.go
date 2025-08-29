package gov

import (
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

// doSlash slashes the voting power of the byzantine validator(voter).
// If the voter has already voted, it will be canceled.
// This function is called from BeatozApp::BeginBlock.
func (ctrler *GovCtrler) doSlash(targetAddr types.Address) (int64, xerrors.XError) {
	var updatedProp []*proposal.GovProposal
	defer func() {
		for _, prop := range updatedProp {
			_ = ctrler.govState.Set(v1.LedgerKeyProposal(prop.Header().TxHash), prop, true)
		}
	}()

	slashPower := int64(0)
	_ = ctrler.govState.Seek(v1.KeyPrefixProposal, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		prop, _ := item.(*proposal.GovProposal)

		if prop.FindVoter(targetAddr) != nil {
			slash, _ := prop.DoSlash(targetAddr, ctrler.SlashRate())
			slashPower += slash
			updatedProp = append(updatedProp, prop)
		}
		return nil
	}, true)

	return slashPower, nil
}
