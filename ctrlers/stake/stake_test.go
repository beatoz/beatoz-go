package stake

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"testing"
)

type stakeTestObj struct {
	s              *Stake
	expectedReward *uint256.Int
}

func TestNewStake(t *testing.T) {
	amt := new(uint256.Int).Mul(ctrlertypes.AmountPerPower(), uint256.NewInt(bytes.RandUint64N(1_000_000_000_000_000_000)))
	s0 := NewStakeWithAmount(
		types.RandAddress(),
		types.RandAddress(),
		amt, 1, nil)

	require.True(t, s0.Power > int64(0))
	require.Equal(t, ctrlertypes.AmountToPower(amt), s0.Power)
}
