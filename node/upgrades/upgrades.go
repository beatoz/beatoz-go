package upgrades

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

const UpgradeName = ""

func Migrate(_ *ctrlertypes.BlockContext) xerrors.XError {
	return nil
}
