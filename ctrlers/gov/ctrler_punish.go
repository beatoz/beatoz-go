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
			//fmt.Println("punishment", "txhash", prop.TxHash, "total.power", prop.TotalVotingPower, "majore.power", prop.MajorityPower)
			//for _, voter := range prop.Voters {
			//	fmt.Println("voter", voter.Addr, "power", voter.Power)
			//}

			//d, _ := ctrler.govState.Get(v1.LedgerKeyProposal(prop.TxHash), true)
			//prop2, _ := d.(*proposal.GovProposal)
			//fmt.Println("read", "txhash", prop2.TxHash, "total.power", prop2.TotalVotingPower, "majore.power", prop2.MajorityPower)
			//for _, voter := range prop2.Voters {
			//fmt.Println("read voter", voter.Addr, "power", voter.Power)
			//}
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
