package gov

import (
	"math"

	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/holiman/uint256"
)

func ForTest1GovParams() *ctrlertypes.GovParams {
	params := ctrlertypes.DefaultGovParams()
	params.SetValue(func(v *ctrlertypes.GovParamsProto) {
		v.Version = 1
		v.MaxValidatorCnt = 10
		v.MinValidatorPower = 1
		v.MinDelegatorPower = 1
		v.LazyUnbondingBlocks = 10
		v.LazyApplyingBlocks = 10
		v.XGasPrice = uint256.NewInt(10).Bytes()
		v.MinTrxGas = 10
		v.BlockSizeLimit = 22_020_096
		v.BlockGasLimit = math.MaxUint64 / 2
		v.MinVotingPeriodBlocks = 10
		v.MaxVotingPeriodBlocks = 10
		v.MinSignedBlocks = 3
		v.InflationWeightPermil = 290
		v.InflationCycleBlocks = 1
		v.MinBondingBlocks = 1
		v.BondingBlocksWeightPermil = 2
		v.XRewardPoolAddress = types.ZeroAddress()
		v.XDeadAddress = types.ZeroAddress()
	})
	return params
}
func ForTest3GovParams() *ctrlertypes.GovParams {
	params := ctrlertypes.DefaultGovParams()
	params.SetValue(func(v *ctrlertypes.GovParamsProto) {
		v.Version = 3
		v.MaxValidatorCnt = 13
		v.MinValidatorPower = 0
		v.MinDelegatorPower = 0
		v.LazyUnbondingBlocks = 20
		v.LazyApplyingBlocks = 0
		v.XGasPrice = nil
		v.MinTrxGas = 0
		v.BlockSizeLimit = 22_020_096
		v.BlockGasLimit = math.MaxUint64 / 2
		v.MinVotingPeriodBlocks = 0
		v.MaxVotingPeriodBlocks = 0
		v.MinSelfPowerRate = 0
		v.MaxUpdatablePowerRate = 10
		v.MaxIndividualPowerRate = 10
		v.MinSelfPowerRate = 50
		v.MinSignedBlocks = 500
		v.InflationWeightPermil = 290
		v.InflationCycleBlocks = 1
		v.MinBondingBlocks = 1
		v.BondingBlocksWeightPermil = 2
		v.XRewardPoolAddress = types.ZeroAddress()
		v.XDeadAddress = types.ZeroAddress()
	})
	return params
}
