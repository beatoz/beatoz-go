package supply

import (
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	btztypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
)

type Reward struct {
	addr btztypes.Address
	amt  *uint256.Int
}

func (rwd *Reward) Encode() ([]byte, xerrors.XError) {
	return rwd.amt.Bytes(), nil
}

func (rwd *Reward) Decode(k, v []byte) xerrors.XError {
	rwd.addr = v1.UnwrapKeyPrefix(k)
	rwd.amt = new(uint256.Int).SetBytes(v)
	return nil
}

var _ v1.ILedgerItem = (*Reward)(nil)
