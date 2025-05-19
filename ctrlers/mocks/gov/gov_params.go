package gov

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/holiman/uint256"
	"math"
)

func ForTest1GovParams() *ctrlertypes.GovParams {
	params := ctrlertypes.DefaultGovParams()
	params.SetValue(func(v *ctrlertypes.GovParamsProto) {
		v.Version = 1
		v.MaxValidatorCnt = 10
		v.MinValidatorPower = 1
		v.MinDelegatorPower = 1
		v.XRewardPerStake = uint256.NewInt(2_000_000_000).Bytes()
		v.LazyUnstakingBlocks = 10
		v.LazyApplyingBlocks = 10
		v.XGasPrice = uint256.NewInt(10).Bytes()
		v.MinTrxGas = 10
		v.MaxTrxGas = math.MaxUint64 / 2
		v.MaxBlockGas = math.MaxUint64 / 2
		v.MinVotingPeriodBlocks = 10
		v.MaxVotingPeriodBlocks = 10
		v.SignedBlocksWindow = 30
		v.MinSignedBlocks = 3
		v.InflationWeightPermil = 290
		v.InflationCycleBlocks = 1
		v.MinBondingBlocks = 1
		v.BondingBlocksWeightPermil = 2
		v.XRewardPoolAddress = types.ZeroAddress()
		v.XBurnAddress = types.ZeroAddress()
	})
	return params
}
func ForTest3GovParams() *ctrlertypes.GovParams {
	params := ctrlertypes.DefaultGovParams()
	params.SetValue(func(v *ctrlertypes.GovParamsProto) {
		v.Version = 3
		v.MaxValidatorCnt = 13
		v.MinValidatorPower = 0
		v.MinDelegatorPower = 0 // issue(hotfix) RG78
		v.XRewardPerStake = uint256.NewInt(0).Bytes()
		v.LazyUnstakingBlocks = 20
		v.LazyApplyingBlocks = 0
		v.XGasPrice = nil
		v.MinTrxGas = 0
		v.MaxTrxGas = math.MaxUint64 / 2
		v.MaxBlockGas = math.MaxUint64 / 2
		v.MinVotingPeriodBlocks = 0
		v.MaxVotingPeriodBlocks = 0
		v.MinSelfStakeRate = 0
		v.MaxUpdatableStakeRate = 10
		v.MaxIndividualStakeRate = 10
		v.MinSelfStakeRate = 50
		v.SignedBlocksWindow = 10000
		v.MinSignedBlocks = 500
		v.InflationWeightPermil = 290
		v.InflationCycleBlocks = 1
		v.MinBondingBlocks = 1
		v.BondingBlocksWeightPermil = 2
		v.XRewardPoolAddress = types.ZeroAddress()
		v.XBurnAddress = types.ZeroAddress()
	})
	return params
}
