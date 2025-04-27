package types

import (
	"encoding/json"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmtypes "github.com/tendermint/tendermint/types"
	"google.golang.org/protobuf/proto"
	"math"
	"sync"
)

var (
	//DEPRECATED
	amountPerPower         = uint256.NewInt(1_000000000_000000000) // 1BEATOZ == 1Power
	secondsPerMinute int64 = 60
	secondsPerHour   int64 = 60 * secondsPerMinute
	secondsPerDay    int64 = 24 * secondsPerHour
	secondsPerWeek   int64 = 7 * secondsPerDay
	secondsPerYear   int64 = 365 * secondsPerDay
)

type GovParams struct {
	version           int64
	maxValidatorCnt   int64
	minValidatorPower int64
	minDelegatorPower int64

	// DEPRECATED
	rewardPerPower *uint256.Int

	lazyUnstakingBlocks   int64
	lazyApplyingBlocks    int64
	gasPrice              *uint256.Int
	minTrxGas             uint64
	maxTrxGas             uint64
	maxBlockGas           uint64
	minVotingPeriodBlocks int64
	maxVotingPeriodBlocks int64

	minSelfStakeRatio       int64
	maxUpdatableStakeRatio  int64
	maxIndividualStakeRatio int64
	slashRatio              int64
	signedBlocksWindow      int64
	minSignedBlocks         int64

	// Add new params at 2025.03
	maxTotalSupply            *uint256.Int
	inflationWeightPermil     int64
	inflationCycleBlocks      int64
	minBondingBlocks          int64
	bondingBlocksWeightPermil int64
	rewardPoolAddress         types.Address
	burnAddress               types.Address
	burnRatio                 int64

	ripeningBlocks           int64
	maxValidatorsOfDelegator int64
	maxDelegatorsOfValidator int64

	mtx sync.RWMutex
}

func newGovParamsWith(interval int) *GovParams {
	// block interval = `interval` seconds
	// max blocks/1Y = 31,536,000 (if all blocks interval 1s)
	// min blocks/1Y = 31,536,000 / `interval` (if all blocks interval `interval` s)

	return &GovParams{
		version:                   1,
		maxValidatorCnt:           21,
		minValidatorPower:         7_000_000, // 7,000,000 BEATOZ
		minDelegatorPower:         4_000,     //  `0` means that the delegating is disable.
		rewardPerPower:            uint256.NewInt(4_756_468_797),
		lazyUnstakingBlocks:       2 * secondsPerWeek / int64(interval), // 2weeks blocks
		lazyApplyingBlocks:        secondsPerDay / int64(interval),      // 1days blocks
		gasPrice:                  uint256.NewInt(250_000_000_000),      // 250e9 = 250 Gfons
		minTrxGas:                 uint64(4_000),                        // 4e3 * 25e10 = 1e15 = 0.001 BEATOZ
		maxTrxGas:                 30_000_000,
		maxBlockGas:               50_000_000,
		minVotingPeriodBlocks:     secondsPerDay / int64(interval),                        // 1day blocks
		maxVotingPeriodBlocks:     30 * secondsPerDay / int64(interval),                   // 30days blocks
		minSelfStakeRatio:         50,                                                     // 50%
		maxUpdatableStakeRatio:    33,                                                     // 33%
		maxIndividualStakeRatio:   33,                                                     // 33%
		slashRatio:                50,                                                     // 50%
		signedBlocksWindow:        10_000,                                                 // 10000 blocks
		minSignedBlocks:           500,                                                    // 500 blocks
		maxTotalSupply:            uint256.MustFromDecimal("700000000000000000000000000"), // 700,000,000 BEATOZ
		inflationWeightPermil:     290,                                                    // 0.290
		inflationCycleBlocks:      2 * secondsPerWeek / int64(interval),                   // 2weeks
		minBondingBlocks:          2 * secondsPerWeek / int64(interval),                   // 2weeks
		bondingBlocksWeightPermil: 2,                                                      // 0.002
		rewardPoolAddress:         types.ZeroAddress(),
		burnAddress:               types.ZeroAddress(), // 0x0000...0000
		burnRatio:                 10,                  // 10%
		ripeningBlocks:            secondsPerYear,
		maxValidatorsOfDelegator:  1,
		maxDelegatorsOfValidator:  1000,
	}
}

func DefaultGovParams() *GovParams {
	return newGovParamsWith(1) // 1s interval
}

func Test1GovParams() *GovParams {
	return &GovParams{
		version:                   1,
		maxValidatorCnt:           10,
		minValidatorPower:         1, // 1 BEATOZ
		minDelegatorPower:         1, // issue(hotfix) RG78
		rewardPerPower:            uint256.NewInt(2_000_000_000),
		lazyUnstakingBlocks:       10,
		lazyApplyingBlocks:        10,
		gasPrice:                  uint256.NewInt(10),
		minTrxGas:                 uint64(10),
		maxTrxGas:                 math.MaxUint64 / 2,
		maxBlockGas:               math.MaxUint64 / 2,
		minVotingPeriodBlocks:     10,
		maxVotingPeriodBlocks:     10,
		minSelfStakeRatio:         50, // 50%
		maxUpdatableStakeRatio:    33, // 33%
		maxIndividualStakeRatio:   33, // 33%
		slashRatio:                50, // 50%
		signedBlocksWindow:        30,
		minSignedBlocks:           3,
		maxTotalSupply:            uint256.MustFromDecimal("700000000000000000000000000"), // 700,000,000 BEATOZ
		inflationWeightPermil:     290,                                                    // 0.290
		inflationCycleBlocks:      1,
		minBondingBlocks:          1,
		bondingBlocksWeightPermil: 2, // 0.002
		rewardPoolAddress:         types.ZeroAddress(),
		burnAddress:               types.ZeroAddress(), // 0x0000...0000
		burnRatio:                 10,                  // 10%
		ripeningBlocks:            secondsPerYear,
		maxValidatorsOfDelegator:  1,
		maxDelegatorsOfValidator:  1000,
	}
}

func Test2GovParams() *GovParams {
	return &GovParams{
		version:                   2,
		maxValidatorCnt:           10,
		minValidatorPower:         5, // 5 BEATOZ
		minDelegatorPower:         0, // issue(hotfix) RG78
		rewardPerPower:            uint256.NewInt(2_000_000_000),
		lazyUnstakingBlocks:       30,
		lazyApplyingBlocks:        40,
		gasPrice:                  uint256.NewInt(20),
		minTrxGas:                 uint64(20),
		maxTrxGas:                 math.MaxUint64 / 2,
		maxBlockGas:               math.MaxUint64 / 2,
		minVotingPeriodBlocks:     50,
		maxVotingPeriodBlocks:     60,
		minSelfStakeRatio:         50,                                                     // 50%
		maxUpdatableStakeRatio:    33,                                                     // 100%
		maxIndividualStakeRatio:   33,                                                     // 10000000%
		slashRatio:                50,                                                     // 50%
		signedBlocksWindow:        10000,                                                  // 10000 blocks
		minSignedBlocks:           5,                                                      // 500 blocks
		maxTotalSupply:            uint256.MustFromDecimal("700000000000000000000000000"), // 700,000,000 BEATOZ
		inflationWeightPermil:     290,                                                    // 0.290
		inflationCycleBlocks:      1,
		minBondingBlocks:          1,
		bondingBlocksWeightPermil: 2, // 0.002
		rewardPoolAddress:         types.ZeroAddress(),
		burnAddress:               types.ZeroAddress(), // 0x0000...0000
		burnRatio:                 10,                  // 10%
		ripeningBlocks:            secondsPerYear,
		maxValidatorsOfDelegator:  1,
		maxDelegatorsOfValidator:  1000,
	}
}

func Test3GovParams() *GovParams {
	return &GovParams{
		version:                   4,
		maxValidatorCnt:           13,
		minValidatorPower:         0,
		minDelegatorPower:         0, // issue(hotfix) RG78
		rewardPerPower:            uint256.NewInt(0),
		lazyUnstakingBlocks:       20,
		lazyApplyingBlocks:        0,
		gasPrice:                  nil,
		minTrxGas:                 0,
		maxTrxGas:                 math.MaxUint64 / 2,
		maxBlockGas:               math.MaxUint64 / 2,
		minVotingPeriodBlocks:     0,
		maxVotingPeriodBlocks:     0,
		minSelfStakeRatio:         0,
		maxUpdatableStakeRatio:    10,
		maxIndividualStakeRatio:   10,
		slashRatio:                50,
		signedBlocksWindow:        10000,
		minSignedBlocks:           500,
		maxTotalSupply:            uint256.MustFromDecimal("700000000000000000000000000"), // 700,000,000 BEATOZ
		inflationWeightPermil:     290,
		inflationCycleBlocks:      1,
		minBondingBlocks:          1,
		bondingBlocksWeightPermil: 2, // 0.002
		rewardPoolAddress:         types.ZeroAddress(),
		burnAddress:               types.ZeroAddress(), // 0x0000...0000
		burnRatio:                 10,                  // 10%
		ripeningBlocks:            secondsPerYear,
		maxValidatorsOfDelegator:  1,
		maxDelegatorsOfValidator:  1000,
	}
}

func Test4GovParams() *GovParams {
	return &GovParams{
		version:                   4,
		maxValidatorCnt:           13,
		minValidatorPower:         7_000_000,
		minDelegatorPower:         0, // issue(hotfix) RG78
		rewardPerPower:            uint256.NewInt(4_756_468_797),
		lazyUnstakingBlocks:       20,
		lazyApplyingBlocks:        259200,
		gasPrice:                  uint256.NewInt(10_000_000_000),
		minTrxGas:                 uint64(100_000),
		maxTrxGas:                 math.MaxUint64 / 2,
		maxBlockGas:               math.MaxUint64 / 2,
		minVotingPeriodBlocks:     259200,
		maxVotingPeriodBlocks:     2592000,
		minSelfStakeRatio:         50,
		maxUpdatableStakeRatio:    10,
		maxIndividualStakeRatio:   10,
		slashRatio:                50,
		signedBlocksWindow:        10000,
		minSignedBlocks:           500,
		maxTotalSupply:            uint256.MustFromDecimal("700000000000000000000000000"), // 700,000,000 BEATOZ
		inflationWeightPermil:     290,
		inflationCycleBlocks:      1,
		minBondingBlocks:          1,
		bondingBlocksWeightPermil: 2, // 0.002
		rewardPoolAddress:         types.ZeroAddress(),
		burnAddress:               types.ZeroAddress(), // 0x0000...0000
		burnRatio:                 10,                  // 10%
		ripeningBlocks:            secondsPerYear,
		maxValidatorsOfDelegator:  1,
		maxDelegatorsOfValidator:  1000,
	}
}

func Test5GovParams() *GovParams {
	return &GovParams{
		version:                   3,
		minValidatorPower:         0,
		minSelfStakeRatio:         40,
		maxUpdatableStakeRatio:    50,
		maxIndividualStakeRatio:   50,
		slashRatio:                60,
		maxTotalSupply:            uint256.MustFromDecimal("700000000000000000000000000"), // 700,000,000 BEATOZ
		inflationWeightPermil:     290,
		inflationCycleBlocks:      1,
		minBondingBlocks:          1,
		bondingBlocksWeightPermil: 2, // 0.002
		rewardPoolAddress:         types.ZeroAddress(),
		burnAddress:               types.ZeroAddress(), // 0x0000...0000
		burnRatio:                 10,                  // 10%
		ripeningBlocks:            secondsPerYear,
		maxValidatorsOfDelegator:  1,
		maxDelegatorsOfValidator:  1000,
	}
}

func Test6GovParams_NoStakeLimiter() *GovParams {
	return &GovParams{
		version:                   2,
		maxValidatorCnt:           10,
		minValidatorPower:         5, // 5 BEATOZ
		minDelegatorPower:         0, // issue(hotfix) RG78
		rewardPerPower:            uint256.NewInt(2_000_000_000),
		lazyUnstakingBlocks:       30,
		lazyApplyingBlocks:        40,
		gasPrice:                  uint256.NewInt(20),
		minTrxGas:                 uint64(20),
		maxTrxGas:                 math.MaxUint64 / 2,
		maxBlockGas:               math.MaxUint64 / 2,
		minVotingPeriodBlocks:     50,
		maxVotingPeriodBlocks:     60,
		minSelfStakeRatio:         50,                                                     // 50%
		maxUpdatableStakeRatio:    100,                                                    // 100%
		maxIndividualStakeRatio:   10000000,                                               // 10000000%
		slashRatio:                50,                                                     // 50%
		signedBlocksWindow:        10000,                                                  // 10000 blocks
		minSignedBlocks:           5,                                                      // 500 blocks
		maxTotalSupply:            uint256.MustFromDecimal("700000000000000000000000000"), // 700,000,000 BEATOZ
		inflationWeightPermil:     290,
		inflationCycleBlocks:      1,
		minBondingBlocks:          1,
		bondingBlocksWeightPermil: 2, // 0.002
		rewardPoolAddress:         types.ZeroAddress(),
		burnAddress:               types.ZeroAddress(), // 0x0000...0000
		burnRatio:                 10,                  // 10%
		ripeningBlocks:            secondsPerYear,
		maxValidatorsOfDelegator:  1,
		maxDelegatorsOfValidator:  1000,
	}
}

func DecodeGovParams(bz []byte) (*GovParams, xerrors.XError) {
	ret := &GovParams{}
	if xerr := ret.Decode(bz); xerr != nil {
		return nil, xerr
	}
	return ret, nil
}

//func (r *GovParams) Key() v1.LedgerKey {
//	return LedgerKeyGovParams()
//}

func (r *GovParams) Decode(bz []byte) xerrors.XError {
	pm := &GovParamsProto{}
	if err := proto.Unmarshal(bz, pm); err != nil {
		return xerrors.From(err)
	}
	r.fromProto(pm)
	return nil
}

func (r *GovParams) Encode() ([]byte, xerrors.XError) {
	if bz, err := proto.Marshal(r.toProto()); err != nil {
		return nil, xerrors.From(err)
	} else {
		return bz, nil
	}
}

func (r *GovParams) fromProto(pm *GovParamsProto) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.version = pm.Version
	r.maxValidatorCnt = pm.MaxValidatorCnt
	r.minValidatorPower = pm.XMinValidatorPower
	r.minDelegatorPower = pm.XMinDelegatorPower
	r.rewardPerPower = new(uint256.Int).SetBytes(pm.XRewardPerPower)
	r.lazyUnstakingBlocks = pm.LazyUnstakingBlocks
	r.lazyApplyingBlocks = pm.LazyApplyingBlocks
	r.gasPrice = new(uint256.Int).SetBytes(pm.XGasPrice)
	r.minTrxGas = pm.MinTrxGas
	r.maxTrxGas = pm.MaxTrxGas
	r.maxBlockGas = pm.MaxBlockGas
	r.minVotingPeriodBlocks = pm.MinVotingPeriodBlocks
	r.maxVotingPeriodBlocks = pm.MaxVotingPeriodBlocks
	r.minSelfStakeRatio = pm.MinSelfStakeRatio
	r.maxUpdatableStakeRatio = pm.MaxUpdatableStakeRatio
	r.maxIndividualStakeRatio = pm.MaxIndividualStakeRatio
	r.slashRatio = pm.SlashRatio
	r.signedBlocksWindow = pm.SignedBlocksWindow
	r.minSignedBlocks = pm.MinSignedBlocks
	r.maxTotalSupply = new(uint256.Int).SetBytes(pm.XMaxTotalSupply)
	r.inflationWeightPermil = pm.InflationWeightPermil
	r.inflationCycleBlocks = pm.InflationCycleBlocks
	r.minBondingBlocks = pm.MinBondingBlocks
	r.bondingBlocksWeightPermil = pm.BondingBlocksWeightPermil
	r.rewardPoolAddress = pm.XRewardPoolAddress
	r.burnAddress = pm.XBurnAddress
	r.burnRatio = pm.BurnRatio
	r.ripeningBlocks = pm.RipeningBlocks
	r.maxValidatorsOfDelegator = pm.MaxValidatorsOfDelegator
	r.maxDelegatorsOfValidator = pm.MaxDelegatorsOfValidator
}

func (r *GovParams) toProto() *GovParamsProto {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	a := &GovParamsProto{
		Version:                   r.version,
		MaxValidatorCnt:           r.maxValidatorCnt,
		XMinValidatorPower:        r.minValidatorPower,
		XMinDelegatorPower:        r.minDelegatorPower,
		XRewardPerPower:           r.rewardPerPower.Bytes(),
		LazyUnstakingBlocks:       r.lazyUnstakingBlocks,
		LazyApplyingBlocks:        r.lazyApplyingBlocks,
		XGasPrice:                 r.gasPrice.Bytes(),
		MinTrxGas:                 r.minTrxGas,
		MaxTrxGas:                 r.maxTrxGas,
		MaxBlockGas:               r.maxBlockGas,
		MinVotingPeriodBlocks:     r.minVotingPeriodBlocks,
		MaxVotingPeriodBlocks:     r.maxVotingPeriodBlocks,
		MinSelfStakeRatio:         r.minSelfStakeRatio,
		MaxUpdatableStakeRatio:    r.maxUpdatableStakeRatio,
		MaxIndividualStakeRatio:   r.maxIndividualStakeRatio,
		SlashRatio:                r.slashRatio,
		SignedBlocksWindow:        r.signedBlocksWindow,
		MinSignedBlocks:           r.minSignedBlocks,
		XMaxTotalSupply:           r.maxTotalSupply.Bytes(),
		InflationWeightPermil:     r.inflationWeightPermil,
		InflationCycleBlocks:      r.inflationCycleBlocks,
		MinBondingBlocks:          r.minBondingBlocks,
		BondingBlocksWeightPermil: r.bondingBlocksWeightPermil,
		XRewardPoolAddress:        r.rewardPoolAddress,
		XBurnAddress:              r.burnAddress,
		BurnRatio:                 r.burnRatio,
		RipeningBlocks:            r.ripeningBlocks,
		MaxValidatorsOfDelegator:  r.maxValidatorsOfDelegator,
		MaxDelegatorsOfValidator:  r.maxDelegatorsOfValidator,
	}
	return a
}

func (r *GovParams) MarshalJSON() ([]byte, error) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	tm := &struct {
		Version                   int64  `json:"version"`
		MaxValidatorCnt           int64  `json:"maxValidatorCnt"`
		MinValidatorPower         int64  `json:"minValidatorPower"`
		MinDelegatorPower         int64  `json:"minDelegatorPower"`
		RewardPerPower            string `json:"rewardPerPower"`
		LazyUnstakingBlocks       int64  `json:"lazyUnstakingBlocks"`
		LazyApplyingBlocks        int64  `json:"lazyApplyingBlocks"`
		GasPrice                  string `json:"gasPrice"`
		MinTrxGas                 uint64 `json:"minTrxGas"`
		MaxTrxGas                 uint64 `json:"maxTrxGas"`
		MaxBlockGas               uint64 `json:"maxBlockGas"`
		MinVotingBlocks           int64  `json:"minVotingPeriodBlocks"`
		MaxVotingBlocks           int64  `json:"maxVotingPeriodBlocks"`
		MinSelfStakeRatio         int64  `json:"minSelfStakeRatio"`
		MaxUpdatableStakeRatio    int64  `json:"maxUpdatableStakeRatio"`
		MaxIndividualStakeRatio   int64  `json:"maxIndividualStakeRatio"`
		SlashRatio                int64  `json:"slashRatio"`
		SignedBlocksWindow        int64  `json:"signedBlocksWindow"`
		MinSignedBlocks           int64  `json:"minSignedBlocks"`
		MaxTotalSupply            string `json:"maxTotalSupply"`
		InflationWeightPermil     int64  `json:"inflationWeightPermil"`
		InflationCycleBlocks      int64  `json:"inflationCycleBlocks"`
		MinBondingBlocks          int64  `json:"minBondingBlocks"`
		BondingBlocksWeightPermil int64  `json:"bondingBlocksWeightPermil"`
		RewardPoolAddress         string `json:"rewardPoolAddress"`
		BurnAddress               string `json:"burnAddress"`
		BurnRatio                 int64  `json:"burnRatio"`
		RipeningBlocks            int64  `json:"ripeningBlocks"`
		MaxValidatorsOfDelegator  int64  `json:"maxValidatorsOfDelegator"`
		MaxDelegatorsOfValidator  int64  `json:"maxDelegatorsOfValidator"`
	}{
		Version:                   r.version,
		MaxValidatorCnt:           r.maxValidatorCnt,
		MinValidatorPower:         r.minValidatorPower,
		MinDelegatorPower:         r.minDelegatorPower,
		RewardPerPower:            uint256ToString(r.rewardPerPower),
		LazyUnstakingBlocks:       r.lazyUnstakingBlocks,
		LazyApplyingBlocks:        r.lazyApplyingBlocks,
		GasPrice:                  uint256ToString(r.gasPrice),
		MinTrxGas:                 r.minTrxGas,
		MaxTrxGas:                 r.maxTrxGas,
		MaxBlockGas:               r.maxBlockGas,
		MinVotingBlocks:           r.minVotingPeriodBlocks,
		MaxVotingBlocks:           r.maxVotingPeriodBlocks,
		MinSelfStakeRatio:         r.minSelfStakeRatio,
		MaxUpdatableStakeRatio:    r.maxUpdatableStakeRatio,
		MaxIndividualStakeRatio:   r.maxIndividualStakeRatio,
		SlashRatio:                r.slashRatio,
		SignedBlocksWindow:        r.signedBlocksWindow,
		MinSignedBlocks:           r.minSignedBlocks,
		MaxTotalSupply:            uint256ToString(r.maxTotalSupply),
		InflationWeightPermil:     r.inflationWeightPermil,
		InflationCycleBlocks:      r.inflationCycleBlocks,
		MinBondingBlocks:          r.minBondingBlocks,
		BondingBlocksWeightPermil: r.bondingBlocksWeightPermil,
		RewardPoolAddress:         r.rewardPoolAddress.String(),
		BurnAddress:               r.burnAddress.String(),
		BurnRatio:                 r.burnRatio,
		RipeningBlocks:            r.ripeningBlocks,
		MaxValidatorsOfDelegator:  r.maxValidatorsOfDelegator,
		MaxDelegatorsOfValidator:  r.maxDelegatorsOfValidator,
	}
	return tmjson.Marshal(tm)
}

func (r *GovParams) UnmarshalJSON(bz []byte) error {
	tm := &struct {
		Version                   int64  `json:"version"`
		MaxValidatorCnt           int64  `json:"maxValidatorCnt"`
		MinValidatorPower         int64  `json:"minValidatorPower"`
		MinDelegatorPower         int64  `json:"minDelegatorPower"`
		RewardPerPower            string `json:"rewardPerPower"`
		LazyUnstakingBlocks       int64  `json:"lazyUnstakingBlocks"`
		LazyApplyingBlocks        int64  `json:"lazyApplyingBlocks"`
		GasPrice                  string `json:"gasPrice"`
		MinTrxGas                 uint64 `json:"minTrxGas"`
		MaxTrxGas                 uint64 `json:"maxTrxGas"`
		MaxBlockGas               uint64 `json:"maxBlockGas"`
		MinVotingBlocks           int64  `json:"minVotingPeriodBlocks"`
		MaxVotingBlocks           int64  `json:"maxVotingPeriodBlocks"`
		MinSelfStakeRatio         int64  `json:"minSelfStakeRatio"`
		MaxUpdatableStakeRatio    int64  `json:"maxUpdatableStakeRatio"`
		MaxIndividualStakeRatio   int64  `json:"maxIndividualStakeRatio"`
		SlashRatio                int64  `json:"slashRatio"`
		SignedBlocksWindow        int64  `json:"signedBlocksWindow"`
		MinSignedBlocks           int64  `json:"minSignedBlocks"`
		MaxTotalSupply            string `json:"maxTotalSupply"`
		InflationWeightPermil     int64  `json:"inflationWeightPermil"`
		InflationCycleBlocks      int64  `json:"inflationCycleBlocks"`
		MinBondingBlocks          int64  `json:"minBondingBlocks"`
		BondingBlocksWeightPermil int64  `json:"bondingBlocksWeightPermil"`
		RewardPoolAddress         string `json:"rewardPoolAddress"`
		BurnAddress               string `json:"burnAddress"`
		BurnRatio                 int64  `json:"burnRatio"`
		RipeningBlocks            int64  `json:"ripeningBlocks"`
		MaxValidatorsOfDelegator  int64  `json:"maxValidatorsOfDelegator"`
		MaxDelegatorsOfValidator  int64  `json:"maxDelegatorsOfValidator"`
	}{}

	err := tmjson.Unmarshal(bz, tm)
	if err != nil {
		return err
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.version = tm.Version
	r.maxValidatorCnt = tm.MaxValidatorCnt
	r.minValidatorPower = tm.MinValidatorPower
	r.minDelegatorPower = tm.MinDelegatorPower

	r.rewardPerPower, err = stringToUint256(tm.RewardPerPower)
	if err != nil {
		return err
	}
	r.lazyUnstakingBlocks = tm.LazyUnstakingBlocks
	r.lazyApplyingBlocks = tm.LazyApplyingBlocks
	r.gasPrice, err = stringToUint256(tm.GasPrice)
	if err != nil {
		return err
	}
	r.minTrxGas = tm.MinTrxGas
	r.maxTrxGas = tm.MaxTrxGas
	r.maxBlockGas = tm.MaxBlockGas
	r.minVotingPeriodBlocks = tm.MinVotingBlocks
	r.maxVotingPeriodBlocks = tm.MaxVotingBlocks
	r.minSelfStakeRatio = tm.MinSelfStakeRatio
	r.maxUpdatableStakeRatio = tm.MaxUpdatableStakeRatio
	r.maxIndividualStakeRatio = tm.MaxIndividualStakeRatio
	r.slashRatio = tm.SlashRatio
	r.signedBlocksWindow = tm.SignedBlocksWindow
	r.minSignedBlocks = tm.MinSignedBlocks
	r.maxTotalSupply, err = stringToUint256(tm.MaxTotalSupply)
	if err != nil {
		return err
	}
	r.inflationWeightPermil = tm.InflationWeightPermil
	r.inflationCycleBlocks = tm.InflationCycleBlocks
	r.minBondingBlocks = tm.MinBondingBlocks
	r.bondingBlocksWeightPermil = tm.BondingBlocksWeightPermil
	r.rewardPoolAddress, err = types.HexToAddress(tm.RewardPoolAddress)
	if err != nil {
		return err
	}
	r.burnAddress, err = types.HexToAddress(tm.BurnAddress)
	if err != nil {
		return err
	}
	r.burnRatio = tm.BurnRatio
	r.ripeningBlocks = tm.RipeningBlocks
	r.maxValidatorsOfDelegator = tm.MaxValidatorsOfDelegator
	r.maxDelegatorsOfValidator = tm.MaxDelegatorsOfValidator
	return nil
}

func uint256ToString(value *uint256.Int) string {
	if value == nil {
		return ""
	}
	return value.Dec()
}

func stringToUint256(value string) (*uint256.Int, error) {
	if value == "" {
		return nil, nil
	}
	returnValue, err := uint256.FromDecimal(value)
	if err != nil {
		return nil, err
	}
	return returnValue, nil
}

func (r *GovParams) Version() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.version
}

func (r *GovParams) MaxValidatorCnt() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.maxValidatorCnt
}

func (r *GovParams) MinValidatorPower() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.minValidatorPower
}

func (r *GovParams) MinDelegatorPower() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.minDelegatorPower
}

// DEPRECATED
func (r *GovParams) RewardPerPower() *uint256.Int {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return new(uint256.Int).Set(r.rewardPerPower)
}

func (r *GovParams) LazyUnstakingBlocks() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.lazyUnstakingBlocks
}

func (r *GovParams) LazyApplyingBlocks() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.lazyApplyingBlocks
}

func (r *GovParams) GasPrice() *uint256.Int {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return new(uint256.Int).Set(r.gasPrice)
}

func (r *GovParams) MinTrxGas() uint64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.minTrxGas
}

func (r *GovParams) MinTrxFee() *uint256.Int {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return new(uint256.Int).Mul(uint256.NewInt(r.minTrxGas), r.gasPrice)
}

func (r *GovParams) MaxTrxGas() uint64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.maxTrxGas
}

func (r *GovParams) MaxTrxFee() *uint256.Int {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return new(uint256.Int).Mul(uint256.NewInt(r.maxTrxGas), r.gasPrice)
}

func (r *GovParams) MaxBlockGas() uint64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.maxBlockGas
}

func (r *GovParams) MinVotingPeriodBlocks() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.minVotingPeriodBlocks
}

func (r *GovParams) MaxVotingPeriodBlocks() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.maxVotingPeriodBlocks
}
func (r *GovParams) MinSelfStakeRatio() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.minSelfStakeRatio
}
func (r *GovParams) MaxUpdatableStakeRatio() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.maxUpdatableStakeRatio
}

func (r *GovParams) MaxIndividualStakeRatio() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.maxIndividualStakeRatio
}
func (r *GovParams) SlashRatio() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.slashRatio
}

func (r *GovParams) SignedBlocksWindow() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.signedBlocksWindow
}

func (r *GovParams) MinSignedBlocks() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.minSignedBlocks
}

func (r *GovParams) MaxTotalSupply() *uint256.Int {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return new(uint256.Int).Set(r.maxTotalSupply)
}

func (r *GovParams) InflationWeightPermil() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.inflationWeightPermil
}

func (r *GovParams) InflationCycleBlocks() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.inflationCycleBlocks
}

func (r *GovParams) MinBondingBlocks() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.minBondingBlocks
}

func (r *GovParams) BondingBlocksWeightPermil() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.bondingBlocksWeightPermil
}

func (r *GovParams) RewardPoolAddress() types.Address {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.rewardPoolAddress
}

func (r *GovParams) BurnAddress() types.Address {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.burnAddress
}

func (r *GovParams) BurnRatio() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.burnRatio
}

func (r *GovParams) RipeningBlocks() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.ripeningBlocks
}

func (r *GovParams) MaxValidatorsOfDelegator() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.maxValidatorsOfDelegator
}

func (r *GovParams) MaxDelegatorsOfValidator() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.maxDelegatorsOfValidator
}

func (r *GovParams) String() string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	if bz, err := json.MarshalIndent(r, "", "  "); err != nil {
		return err.Error()
	} else {
		return string(bz)
	}
}

// utility methods
func MaxTotalPower() int64 {
	return tmtypes.MaxTotalVotingPower
}

// DEPRECATED
func AmountToPower(amt *uint256.Int) (int64, xerrors.XError) {
	// 1 VotingPower == 1 BEATOZ
	_vp := new(uint256.Int).Div(amt, amountPerPower)
	vp := int64(_vp.Uint64())
	if vp < 0 {
		return -1, xerrors.ErrOverFlow.Wrapf("voting power is converted as negative(%v) from amount(%v)", vp, amt.Dec())
	}
	return vp, nil
}

// DEPRECATED
func PowerToAmount(power int64) *uint256.Int {
	// 1 VotingPower == 1 BEATOZ = 10^18 amount
	return new(uint256.Int).Mul(uint256.NewInt(uint64(power)), amountPerPower)
}

// DEPRECATED
func AmountPerPower() *uint256.Int {
	return amountPerPower.Clone()
}

func FeeToGas(fee, price *uint256.Int) uint64 {
	gas := new(uint256.Int).Div(fee, price)
	return gas.Uint64()
}

func GasToFee(gas uint64, price *uint256.Int) *uint256.Int {
	return new(uint256.Int).Mul(uint256.NewInt(gas), price)
}

func MergeGovParams(oldParams, newParams *GovParams) {
	if newParams.version == 0 {
		newParams.version = oldParams.version
	}

	if newParams.maxValidatorCnt == 0 {
		newParams.maxValidatorCnt = oldParams.maxValidatorCnt
	}

	if newParams.minValidatorPower == 0 {
		newParams.minValidatorPower = oldParams.minValidatorPower
	}

	if newParams.minDelegatorPower == 0 {
		newParams.minDelegatorPower = oldParams.minDelegatorPower
	}

	if newParams.rewardPerPower == nil || newParams.rewardPerPower.IsZero() {
		newParams.rewardPerPower = oldParams.rewardPerPower
	}

	if newParams.lazyUnstakingBlocks == 0 {
		newParams.lazyUnstakingBlocks = oldParams.lazyUnstakingBlocks
	}

	if newParams.lazyApplyingBlocks == 0 {
		newParams.lazyApplyingBlocks = oldParams.lazyApplyingBlocks
	}

	if newParams.gasPrice == nil || newParams.gasPrice.IsZero() {
		newParams.gasPrice = oldParams.gasPrice
	}

	if newParams.minTrxGas == 0 {
		newParams.minTrxGas = oldParams.minTrxGas
	}

	if newParams.maxTrxGas == 0 {
		newParams.maxTrxGas = oldParams.maxTrxGas
	}

	if newParams.maxBlockGas == 0 {
		newParams.maxBlockGas = oldParams.maxBlockGas
	}

	if newParams.minVotingPeriodBlocks == 0 {
		newParams.minVotingPeriodBlocks = oldParams.minVotingPeriodBlocks
	}

	if newParams.maxVotingPeriodBlocks == 0 {
		newParams.maxVotingPeriodBlocks = oldParams.maxVotingPeriodBlocks
	}

	if newParams.minSelfStakeRatio == 0 {
		newParams.minSelfStakeRatio = oldParams.minSelfStakeRatio
	}

	if newParams.maxUpdatableStakeRatio == 0 {
		newParams.maxUpdatableStakeRatio = oldParams.maxUpdatableStakeRatio
	}

	if newParams.maxIndividualStakeRatio == 0 {
		newParams.maxIndividualStakeRatio = oldParams.maxIndividualStakeRatio
	}

	if newParams.slashRatio == 0 {
		newParams.slashRatio = oldParams.slashRatio
	}

	if newParams.signedBlocksWindow == 0 {
		newParams.signedBlocksWindow = oldParams.signedBlocksWindow
	}

	if newParams.minSignedBlocks == 0 {
		newParams.minSignedBlocks = oldParams.minSignedBlocks
	}

	if newParams.maxTotalSupply == nil || newParams.maxTotalSupply.IsZero() {
		newParams.maxTotalSupply = oldParams.maxTotalSupply
	}

	if newParams.inflationWeightPermil == 0 {
		newParams.inflationWeightPermil = oldParams.inflationWeightPermil
	}

	if newParams.inflationCycleBlocks == 0 {
		newParams.inflationCycleBlocks = oldParams.inflationCycleBlocks
	}

	if newParams.minBondingBlocks == 0 {
		newParams.minBondingBlocks = oldParams.minBondingBlocks
	}

	if newParams.bondingBlocksWeightPermil == 0 {
		newParams.bondingBlocksWeightPermil = oldParams.bondingBlocksWeightPermil
	}

	if newParams.rewardPoolAddress == nil {
		newParams.rewardPoolAddress = oldParams.rewardPoolAddress
	}

	if newParams.burnAddress == nil {
		newParams.burnAddress = oldParams.burnAddress
	}

	if newParams.burnRatio == 0 {
		newParams.burnRatio = oldParams.burnRatio
	}

	if newParams.ripeningBlocks == 0 {
		newParams.ripeningBlocks = oldParams.ripeningBlocks
	}

	if newParams.maxValidatorsOfDelegator == 0 {
		newParams.maxValidatorsOfDelegator = oldParams.maxValidatorsOfDelegator
	}
	if newParams.maxDelegatorsOfValidator == 0 {
		newParams.maxDelegatorsOfValidator = oldParams.maxDelegatorsOfValidator
	}
}

var _ v1.ILedgerItem = (*GovParams)(nil)
var _ IGovParams = (*GovParams)(nil)
