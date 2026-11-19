package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ctrlers/upgrade"
	"github.com/beatoz/beatoz-go/node/upgrades"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

const upgradeInfoFileName = "upgrade-info.json"

type upgradeInfo struct {
	Name   string `json:"name"`
	Height int64  `json:"height"`
	Info   string `json:"info"`
}

func (ctrler *BeatozApp) checkUpgradeBinary(lastHeight int64) xerrors.XError {
	lastName := ""
	lastDone, xerr := ctrler.upgradeCtrler.LastCompletedUpgrade(false)
	if xerr != nil {
		return xerr
	}
	if lastDone != nil {
		lastName = lastDone.Name()
	}
	if lastName == upgrades.UpgradeName {
		return nil
	}

	plan, xerr := ctrler.upgradeCtrler.ReservedUpgrade(false)
	if xerr != nil {
		return xerr
	}
	if plan != nil && plan.Name() == upgrades.UpgradeName && plan.Height() == lastHeight+1 {
		return nil
	}

	planName, planHeight := "", int64(0)
	if plan != nil {
		planName, planHeight = plan.Name(), plan.Height()
	}
	return xerrors.NewOrdinary(fmt.Sprintf(
		"wrong binary: binary upgrade name %q, last applied upgrade %q, pending plan %q at %d, last height %d",
		upgrades.UpgradeName, lastName, planName, planHeight, lastHeight))
}

func (ctrler *BeatozApp) executeUpgrade(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	plan, xerr := ctrler.upgradeCtrler.ReservedUpgrade(true)
	if xerr != nil {
		return nil, xerr
	}
	if plan == nil || plan.Height() != bctx.Height() {
		return nil, nil
	}

	if plan.Name() != upgrades.UpgradeName {
		if xerr := ctrler.writeUpgradeInfo(plan); xerr != nil {
			ctrler.logger.Error("failed to write upgrade info", "error", xerr)
		}
		panic(fmt.Sprintf("UPGRADE %q NEEDED at height %d: %s", plan.Name(), plan.Height(), plan.Info()))
	}

	if xerr := upgrades.Migrate(bctx); xerr != nil {
		return nil, xerr
	}
	if xerr := ctrler.upgradeCtrler.CompleteUpgrade(plan); xerr != nil {
		return nil, xerr
	}

	return []abcitypes.Event{
		{
			Type: "upgrade",
			Attributes: []abcitypes.EventAttribute{
				{Key: []byte("name"), Value: []byte(plan.Name()), Index: true},
				{Key: []byte("height"), Value: []byte(strconv.FormatInt(plan.Height(), 10)), Index: false},
			},
		},
	}, nil
}

func (ctrler *BeatozApp) writeUpgradeInfo(plan *upgrade.UpgradePlan) xerrors.XError {
	bz, err := json.Marshal(&upgradeInfo{Name: plan.Name(), Height: plan.Height(), Info: plan.Info()})
	if err != nil {
		return xerrors.From(err)
	}
	if err := os.WriteFile(filepath.Join(ctrler.rootConfig.DBDir(), upgradeInfoFileName), bz, 0o644); err != nil {
		return xerrors.From(err)
	}
	return nil
}
