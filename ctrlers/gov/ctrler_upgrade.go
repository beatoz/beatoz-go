package gov

import (
	"github.com/beatoz/beatoz-go/ctrlers/gov/proposal"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

func (ctrler *GovCtrler) validateUpgradeProposal(ctx *ctrlertypes.TrxContext, txpayload *ctrlertypes.TrxPayloadProposal) xerrors.XError {
	if !types.IsBTIP64(ctx.ChainID(), ctx.Height()) {
		return nil
	}

	switch txpayload.OptType {
	case proposal.PROPOSAL_GOVPARAMS:
		return nil
	case proposal.PROPOSAL_UPGRADE:
		if len(txpayload.Options) != 1 {
			return xerrors.ErrInvalidTrxPayloadParams.Wrapf("wrong options: upgrade proposal must have exactly one option")
		}
		if ctx.UpgradeHandler == nil {
			return xerrors.ErrInvalidTrx.Wrapf("upgrade handler is not set")
		}
		opt, xerr := ctrlertypes.ParseUpgradePlanOption(txpayload.Options[0])
		if xerr != nil {
			return xerr
		}
		return ctx.UpgradeHandler.ValidateUpgrade(opt, txpayload.ApplyingHeight, ctrler.MinUpgradeDelayBlocks(), ctx.Exec)
	case proposal.PROPOSAL_CANCEL_UPGRADE:
		if len(txpayload.Options) != 1 {
			return xerrors.ErrInvalidTrxPayloadParams.Wrapf("wrong options: cancel upgrade proposal must have exactly one option")
		}
		_, xerr := ctrlertypes.ParseUpgradeCancelOption(txpayload.Options[0])
		return xerr
	default:
		if txpayload.OptType&proposal.PROPOSAL_ONCHAIN != 0 {
			return xerrors.ErrInvalidTrxPayloadParams.Wrapf("unknown onchain proposal type: 0x%x", txpayload.OptType)
		}
		return nil
	}
}

func (ctrler *GovCtrler) applyUpgradeProposal(bctx *ctrlertypes.BlockContext, prop *proposal.GovProposal) xerrors.XError {
	if bctx.UpgradeHandler == nil {
		return xerrors.NewOrdinary("upgrade handler is not set")
	}

	switch prop.Header().PropType {
	case proposal.PROPOSAL_UPGRADE:
		opt, xerr := ctrlertypes.ParseUpgradePlanOption(prop.MajorOption().Option)
		if xerr != nil {
			return xerr
		}
		return bctx.UpgradeHandler.ReserveUpgrade(opt, prop.Header().TxHash, bctx.Height(), ctrler.MinUpgradeDelayBlocks())
	case proposal.PROPOSAL_CANCEL_UPGRADE:
		opt, xerr := ctrlertypes.ParseUpgradeCancelOption(prop.MajorOption().Option)
		if xerr != nil {
			return xerr
		}
		return bctx.UpgradeHandler.CancelUpgrade(opt.Name)
	default:
		return xerrors.NewOrdinary("not upgrade proposal")
	}
}
