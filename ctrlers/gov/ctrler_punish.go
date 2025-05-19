package gov

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

// doSlash slashes the voting power of the byzantine validator(voter).
// If the voter has already voted, it will be canceled.
// This function is called from BeatozApp::BeginBlock.
func (ctrler *GovCtrler) doSlash(targetAddr types.Address) (int64, xerrors.XError) {
	var punishedProp []*proposal.GovProposal
	defer func() {
		for _, prop := range punishedProp {
			_ = ctrler.govState.Set(v1.LedgerKeyProposal(prop.TxHash), prop, true)
			fmt.Println("punishment", "txhash", prop.TxHash, "total.power", prop.TotalVotingPower, "majore.power", prop.MajorityPower)
			for _, voter := range prop.Voters {
				fmt.Println("voter", voter.Addr, "power", voter.Power)
			}

			d, _ := ctrler.govState.Get(v1.LedgerKeyProposal(prop.TxHash), true)
			prop2, _ := d.(*proposal.GovProposal)
			fmt.Println("read", "txhash", prop2.TxHash, "total.power", prop2.TotalVotingPower, "majore.power", prop2.MajorityPower)
			for _, voter := range prop2.Voters {
				fmt.Println("read voter", voter.Addr, "power", voter.Power)
			}
		}
	}()

	slashedPower := int64(0)
	_ = ctrler.govState.Seek(v1.KeyPrefixProposal, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		prop, _ := item.(*proposal.GovProposal)
		for _, v := range prop.Voters {
			if bytes.Compare(v.Addr, targetAddr) == 0 {
				// the voting power of `targetAddr` will be slashed, and
				// the vote of `targetAddr` will be canceled.
				slashed, _ := prop.DoPunish(targetAddr, ctrler.SlashRate())
				slashedPower += slashed
				punishedProp = append(punishedProp, prop)
				break
			}
		}
		return nil
	}, true)

	return slashedPower, nil
}
