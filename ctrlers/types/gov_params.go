package types

import (
	"encoding/base64"
	"encoding/json"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	tmtypes "github.com/tendermint/tendermint/types"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"math"
	"reflect"
	"sync"
	"unicode"
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

//type GovParams struct {
//	_v *GovParamsProto
//	mtx    sync.RWMutex
//}

// type GovParams GovParamsProto

type GovParams struct {
	_v  GovParamsProto
	mtx sync.RWMutex
}

func DefaultGovParams() *GovParams {
	return newGovParamsWith(1) // 1s interval
}

func Test1GovParams() *GovParams {
	params := DefaultGovParams()
	params._v.Version = 1
	params._v.MaxValidatorCnt = 10
	params._v.MinValidatorPower = 1
	params._v.MinDelegatorPower = 1
	params._v.XRewardPerStake = uint256.NewInt(2_000_000_000).Bytes()
	params._v.LazyUnstakingBlocks = 10
	params._v.LazyApplyingBlocks = 10
	params._v.XGasPrice = uint256.NewInt(10).Bytes()
	params._v.MinTrxGas = 10
	params._v.MaxTrxGas = math.MaxUint64 / 2
	params._v.MaxBlockGas = math.MaxUint64 / 2
	params._v.MinVotingPeriodBlocks = 10
	params._v.MaxVotingPeriodBlocks = 10
	params._v.SignedBlocksWindow = 30
	params._v.MinSignedBlocks = 3
	params._v.InflationWeightPermil = 290
	params._v.InflationCycleBlocks = 1
	params._v.MinBondingBlocks = 1
	params._v.BondingBlocksWeightPermil = 2
	params._v.XRewardPoolAddress = types.ZeroAddress()
	params._v.XBurnAddress = types.ZeroAddress()
	return params
}

//	func Test2GovParams() *GovParams {
//		return &GovParams{
//			version:                   2,
//			maxValidatorCnt:           10,
//			minValidatorPower:         5, // 5 BEATOZ
//			minDelegatorPower:         0, // issue(hotfix) RG78
//			rewardPerPower:            uint256.NewInt(2_000_000_000),
//			lazyUnstakingBlocks:       30,
//			lazyApplyingBlocks:        40,
//			gasPrice:                  uint256.NewInt(20),
//			minTrxGas:                 uint64(20),
//			maxTrxGas:                 math.MaxUint64 / 2,
//			maxBlockGas:               math.MaxUint64 / 2,
//			minVotingPeriodBlocks:     50,
//			maxVotingPeriodBlocks:     60,
//			minSelfStakeRatio:         50,                                                     // 50%
//			maxUpdatableStakeRatio:    33,                                                     // 100%
//			maxIndividualStakeRatio:   33,                                                     // 10000000%
//			slashRatio:                50,                                                     // 50%
//			signedBlocksWindow:        10000,                                                  // 10000 blocks
//			minSignedBlocks:           5,                                                      // 500 blocks
//			maxTotalSupply:            uint256.MustFromDecimal("700000000000000000000000000"), // 700,000,000 BEATOZ
//			inflationWeightPermil:     290,                                                    // 0.290
//			inflationCycleBlocks:      1,
//			minBondingBlocks:          1,
//			bondingBlocksWeightPermil: 2, // 0.002
//			rewardPoolAddress:         types.ZeroAddress(),
//			burnAddress:               types.ZeroAddress(), // 0x0000...0000
//			burnRatio:                 10,                  // 10%
//			ripeningBlocks:            secondsPerYear,
//			maxValidatorsOfDelegator:  1,
//			maxDelegatorsOfValidator:  1000,
//		}
//	}
func Test3GovParams() *GovParams {
	params := DefaultGovParams()
	params._v.Version = 3
	params._v.MaxValidatorCnt = 13
	params._v.MinValidatorPower = 0
	params._v.MinDelegatorPower = 0 // issue(hotfix) RG78
	params._v.XRewardPerStake = uint256.NewInt(0).Bytes()
	params._v.LazyUnstakingBlocks = 20
	params._v.LazyApplyingBlocks = 0
	params._v.XGasPrice = nil
	params._v.MinTrxGas = 0
	params._v.MaxTrxGas = math.MaxUint64 / 2
	params._v.MaxBlockGas = math.MaxUint64 / 2
	params._v.MinVotingPeriodBlocks = 0
	params._v.MaxVotingPeriodBlocks = 0
	params._v.MinSelfStakeRate = 0
	params._v.MaxUpdatableStakeRate = 10
	params._v.MaxIndividualStakeRate = 10
	params._v.MinSelfStakeRate = 50
	params._v.SignedBlocksWindow = 10000
	params._v.MinSignedBlocks = 500
	params._v.InflationWeightPermil = 290
	params._v.InflationCycleBlocks = 1
	params._v.MinBondingBlocks = 1
	params._v.BondingBlocksWeightPermil = 2
	params._v.XRewardPoolAddress = types.ZeroAddress()
	params._v.XBurnAddress = types.ZeroAddress()

	return params
}

//
//func Test4GovParams() *GovParams {
//	return &GovParams{
//		version:                   4,
//		maxValidatorCnt:           13,
//		minValidatorPower:         7_000_000,
//		minDelegatorPower:         0, // issue(hotfix) RG78
//		rewardPerPower:            uint256.NewInt(4_756_468_797),
//		lazyUnstakingBlocks:       20,
//		lazyApplyingBlocks:        259200,
//		gasPrice:                  uint256.NewInt(10_000_000_000),
//		minTrxGas:                 uint64(100_000),
//		maxTrxGas:                 math.MaxUint64 / 2,
//		maxBlockGas:               math.MaxUint64 / 2,
//		minVotingPeriodBlocks:     259200,
//		maxVotingPeriodBlocks:     2592000,
//		minSelfStakeRatio:         50,
//		maxUpdatableStakeRatio:    10,
//		maxIndividualStakeRatio:   10,
//		slashRatio:                50,
//		signedBlocksWindow:        10000,
//		minSignedBlocks:           500,
//		maxTotalSupply:            uint256.MustFromDecimal("700000000000000000000000000"), // 700,000,000 BEATOZ
//		inflationWeightPermil:     290,
//		inflationCycleBlocks:      1,
//		minBondingBlocks:          1,
//		bondingBlocksWeightPermil: 2, // 0.002
//		rewardPoolAddress:         types.ZeroAddress(),
//		burnAddress:               types.ZeroAddress(), // 0x0000...0000
//		burnRatio:                 10,                  // 10%
//		ripeningBlocks:            secondsPerYear,
//		maxValidatorsOfDelegator:  1,
//		maxDelegatorsOfValidator:  1000,
//	}
//}
//
//func Test5GovParams() *GovParams {
//	return &GovParams{
//		version:                   3,
//		minValidatorPower:         0,
//		minSelfStakeRatio:         40,
//		maxUpdatableStakeRatio:    50,
//		maxIndividualStakeRatio:   50,
//		slashRatio:                60,
//		maxTotalSupply:            uint256.MustFromDecimal("700000000000000000000000000"), // 700,000,000 BEATOZ
//		inflationWeightPermil:     290,
//		inflationCycleBlocks:      1,
//		minBondingBlocks:          1,
//		bondingBlocksWeightPermil: 2, // 0.002
//		rewardPoolAddress:         types.ZeroAddress(),
//		burnAddress:               types.ZeroAddress(), // 0x0000...0000
//		burnRatio:                 10,                  // 10%
//		ripeningBlocks:            secondsPerYear,
//		maxValidatorsOfDelegator:  1,
//		maxDelegatorsOfValidator:  1000,
//	}
//}
//
//func Test6GovParams_NoStakeLimiter() *GovParams {
//	return &GovParams{
//		version:                   2,
//		maxValidatorCnt:           10,
//		minValidatorPower:         5, // 5 BEATOZ
//		minDelegatorPower:         0, // issue(hotfix) RG78
//		rewardPerPower:            uint256.NewInt(2_000_000_000),
//		lazyUnstakingBlocks:       30,
//		lazyApplyingBlocks:        40,
//		gasPrice:                  uint256.NewInt(20),
//		minTrxGas:                 uint64(20),
//		maxTrxGas:                 math.MaxUint64 / 2,
//		maxBlockGas:               math.MaxUint64 / 2,
//		minVotingPeriodBlocks:     50,
//		maxVotingPeriodBlocks:     60,
//		minSelfStakeRatio:         50,                                                     // 50%
//		maxUpdatableStakeRatio:    100,                                                    // 100%
//		maxIndividualStakeRatio:   10000000,                                               // 10000000%
//		slashRatio:                50,                                                     // 50%
//		signedBlocksWindow:        10000,                                                  // 10000 blocks
//		minSignedBlocks:           5,                                                      // 500 blocks
//		maxTotalSupply:            uint256.MustFromDecimal("700000000000000000000000000"), // 700,000,000 BEATOZ
//		inflationWeightPermil:     290,
//		inflationCycleBlocks:      1,
//		minBondingBlocks:          1,
//		bondingBlocksWeightPermil: 2, // 0.002
//		rewardPoolAddress:         types.ZeroAddress(),
//		burnAddress:               types.ZeroAddress(), // 0x0000...0000
//		burnRatio:                 10,                  // 10%
//		ripeningBlocks:            secondsPerYear,
//		maxValidatorsOfDelegator:  1,
//		maxDelegatorsOfValidator:  1000,
//	}
//}

func emptyGovParams() *GovParams {
	return &GovParams{
		_v: GovParamsProto{},
	}
}

func newGovParamsWith(interval int) *GovParams {
	// block interval = `interval` seconds
	// max blocks/1Y = 31,536,000 (if all blocks interval 1s)
	// min blocks/1Y = 31,536,000 / `interval` (if all blocks interval `interval` s)

	ret := &GovParams{
		_v: GovParamsProto{
			Version:                   1,
			MaxValidatorCnt:           21,
			MinValidatorPower:         7_000_000, // 7,000,000 BEATOZ
			MinDelegatorPower:         4_000,     //  `0` means that the delegating is disable.
			MaxValidatorsOfDelegator:  1,
			MaxDelegatorsOfValidator:  1000,
			MinSelfStakeRate:          50,                                                             // 50%
			MaxUpdatableStakeRate:     33,                                                             // 33%
			MaxIndividualStakeRate:    33,                                                             // 33%
			MinBondingBlocks:          2 * secondsPerWeek / int64(interval),                           // 2 weeks blocks
			LazyUnstakingBlocks:       2 * secondsPerWeek / int64(interval),                           // 2 weeks blocks
			XMaxTotalSupply:           uint256.MustFromDecimal("700000000000000000000000000").Bytes(), // 700,000,000 BEATOZ
			InflationWeightPermil:     390,                                                            // 0.390
			InflationCycleBlocks:      secondsPerWeek / int64(interval),                               // 1 weeks blocks
			BondingBlocksWeightPermil: 500,                                                            // 0.500
			RipeningBlocks:            secondsPerYear / int64(interval),                               // one year blocks
			XRewardPoolAddress:        types.ZeroAddress(),                                            // zero address
			ValidatorRewardRate:       30,                                                             // 30%
			XBurnAddress:              types.ZeroAddress(),                                            // zero address
			BurnRate:                  10,                                                             // 10%
			SlashRate:                 50,                                                             // 50%
			SignedBlocksWindow:        10_000,
			MinSignedBlocks:           500,
			XGasPrice:                 uint256.NewInt(250_000_000_000).Bytes(), // 250e9 = 250 Gfons
			MinTrxGas:                 4_000,                                   // 4e3 * 25e10 = 1e15 = 0.001 BEATOZ
			MaxTrxGas:                 30_000_000,
			MaxBlockGas:               50_000_000,
			MinVotingPeriodBlocks:     secondsPerDay / int64(interval),     // 1 days blocks
			MaxVotingPeriodBlocks:     7 * secondsPerDay / int64(interval), // 7 day blocks
			LazyApplyingBlocks:        secondsPerDay / int64(interval),     // 1days blocks
			XRewardPerStake:           uint256.NewInt(4_756_468_797).Bytes(),
		},
		mtx: sync.RWMutex{},
	}
	return ret
}

func (govParams *GovParams) Encode() ([]byte, xerrors.XError) {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	if bz, err := proto.Marshal(&govParams._v); err != nil {
		return nil, xerrors.From(err)
	} else {
		return bz, nil
	}
}

func (govParams *GovParams) Decode(k, v []byte) xerrors.XError {
	govParams.mtx.Lock()
	defer govParams.mtx.Unlock()

	govParams._v = GovParamsProto{}
	if err := proto.Unmarshal(v, &govParams._v); err != nil {
		return xerrors.From(err)
	}
	return nil
}

func (govParams *GovParams) MarshalJSON() ([]byte, error) {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	raw, err := protojson.Marshal(&govParams._v)
	if err != nil {
		return nil, err
	}

	var tmp map[string]interface{}
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	for k, v := range tmp {
		// 이름 바꾸기
		// newKey := "AAA_" + k[4:] // 예: "val_0" → "AAA_0"
		if k == "MaxTotalSupply" {
			result["maxTotalSupply"] = govParams.MaxTotalSupply().String()
		} else if k == "GasPrice" {
			result["gasPrice"] = govParams.GasPrice().String()
		} else {
			result[lowercaseFirstIfUpper(k)] = v
		}
	}

	return json.Marshal(result)
}

func (govParams *GovParams) UnmarshalJSON(d []byte) error {
	var tmp map[string]interface{}
	if err := json.Unmarshal(d, &tmp); err != nil {
		return err
	}

	result := make(map[string]interface{})
	for k, v := range tmp {
		if k == "maxTotalSupply" || k == "gasPrice" {
			result[uppercaseFirstIfUpper(k)] = base64.StdEncoding.EncodeToString(uint256.MustFromDecimal(v.(string)).Bytes())
		} else if k == "burnAddress" || k == "rewardPoolAddress" || k == "rewardPerStake" {
			result[uppercaseFirstIfUpper(k)] = v
		} else {
			result[k] = v
		}
	}

	d0, err := json.Marshal(result)
	if err != nil {
		return err
	}

	govParams._v = GovParamsProto{}
	if err := protojson.Unmarshal(d0, &govParams._v); err != nil {
		return err
	}
	return nil
}

func uppercaseFirstIfUpper(s string) string {
	if s == "" {
		return s
	}

	firstRune, size := utf8DecodeRuneInString(s)
	if unicode.IsLower(firstRune) {
		lower := unicode.ToUpper(firstRune)
		return string(lower) + s[size:]
	}
	return s
}

func lowercaseFirstIfUpper(s string) string {
	if s == "" {
		return s
	}

	firstRune, size := utf8DecodeRuneInString(s)
	if unicode.IsUpper(firstRune) {
		lower := unicode.ToLower(firstRune)
		return string(lower) + s[size:]
	}
	return s
}

// 안전한 utf8 첫 글자 추출 (rune, size)
func utf8DecodeRuneInString(s string) (rune, int) {
	if s == "" {
		return rune(0), 0
	}
	return []rune(s)[0], len(string([]rune(s)[0]))
}

func (govParams *GovParams) Version() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.Version
}

func (govParams *GovParams) MaxValidatorCnt() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MaxValidatorCnt
}
func (govParams *GovParams) MinValidatorPower() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MinValidatorPower
}
func (govParams *GovParams) MinDelegatorPower() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MinDelegatorPower
}
func (govParams *GovParams) MaxValidatorsOfDelegator() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MaxValidatorsOfDelegator
}
func (govParams *GovParams) MaxDelegatorsOfValidator() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MaxDelegatorsOfValidator
}
func (govParams *GovParams) MinSelfStakeRate() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MinSelfStakeRate
}
func (govParams *GovParams) MaxUpdatableStakeRate() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MaxUpdatableStakeRate
}
func (govParams *GovParams) MaxIndividualStakeRate() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MaxIndividualStakeRate
}
func (govParams *GovParams) MinBondingBlocks() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MinBondingBlocks
}
func (govParams *GovParams) LazyUnstakingBlocks() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.LazyUnstakingBlocks
}
func (govParams *GovParams) MaxTotalSupply() *uint256.Int {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return new(uint256.Int).SetBytes(govParams._v.XMaxTotalSupply)
}
func (govParams *GovParams) InflationWeightPermil() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.InflationWeightPermil
}
func (govParams *GovParams) InflationCycleBlocks() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.InflationCycleBlocks
}
func (govParams *GovParams) BondingBlocksWeightPermil() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.BondingBlocksWeightPermil
}
func (govParams *GovParams) RipeningBlocks() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.RipeningBlocks
}
func (govParams *GovParams) RewardPoolAddress() types.Address {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return types.Address(govParams._v.XRewardPoolAddress).Copy()
}
func (govParams *GovParams) ValidatorRewardRate() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.ValidatorRewardRate
}
func (govParams *GovParams) BurnAddress() types.Address {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return types.Address(govParams._v.XBurnAddress).Copy()
}
func (govParams *GovParams) BurnRate() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.BurnRate
}
func (govParams *GovParams) SlashRate() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.SlashRate
}
func (govParams *GovParams) SignedBlocksWindow() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.SignedBlocksWindow
}
func (govParams *GovParams) MinSignedBlocks() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MinSignedBlocks
}
func (govParams *GovParams) GasPrice() *uint256.Int {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return new(uint256.Int).SetBytes(govParams._v.XGasPrice)
}
func (govParams *GovParams) MinTrxFee() *uint256.Int {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	gasPrice := new(uint256.Int).SetBytes(govParams._v.XGasPrice)
	return new(uint256.Int).Mul(uint256.NewInt(uint64(govParams._v.MinTrxGas)), gasPrice)
}
func (govParams *GovParams) MinTrxGas() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MinTrxGas
}
func (govParams *GovParams) MaxTrxGas() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MaxTrxGas
}
func (govParams *GovParams) MaxBlockGas() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MaxBlockGas
}
func (govParams *GovParams) MaxVotingPeriodBlocks() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MaxVotingPeriodBlocks
}
func (govParams *GovParams) MinVotingPeriodBlocks() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MinVotingPeriodBlocks
}
func (govParams *GovParams) LazyApplyingBlocks() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.LazyApplyingBlocks
}

// deprecated
func (govParams *GovParams) RewardPerPower() *uint256.Int {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return new(uint256.Int).SetBytes(govParams._v.XRewardPerStake)
}

func (govParams *GovParams) GetValues() *GovParamsProto {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return &govParams._v
}

func (govParams *GovParams) Equal(o *GovParams) bool {
	return proto.Equal(&govParams._v, &o._v)
}

func (govParams *GovParams) String() string {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	if bz, err := json.MarshalIndent(govParams, "", "  "); err != nil {
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

func GasToFee(gas int64, price *uint256.Int) *uint256.Int {
	return new(uint256.Int).Mul(uint256.NewInt(uint64(gas)), price)
}

func MergeGovParams(oldParams, newParams *GovParams) {
	refT := reflect.TypeOf(GovParamsProto{})
	refVOld := reflect.ValueOf(oldParams.GetValues()).Elem()
	refVNew := reflect.ValueOf(newParams.GetValues()).Elem()

	for i := 0; i < refT.NumField(); i++ {
		field0 := refT.Field(i)
		fieldName := field0.Name
		fieldType := field0.Type

		newVal := refVNew.FieldByName(fieldName)
		if !newVal.IsValid() || !newVal.CanSet() {
			//fmt.Printf("skip %v\n", fieldName)
			continue
		}

		zeroInf := reflect.Zero(fieldType).Interface()

		newInf := newVal.Interface()
		if reflect.DeepEqual(newInf, zeroInf) {
			oldVal := refVOld.FieldByName(fieldName)
			newVal.Set(oldVal)
			//fmt.Printf("%-10s | %-20s | %#v -> %#v\n", fieldName, fieldType, newInf, newVal.Interface())
		}
	}
}

var _ v1.ILedgerItem = (*GovParams)(nil)
var _ IGovParams = (*GovParams)(nil)

// functions for test

// DEPRECATED
func (govParams *GovParams) SetLazyUnstakingBlocks(n int64) {
	govParams.mtx.Lock()
	defer govParams.mtx.Unlock()

	govParams._v.LazyUnstakingBlocks = n
}

func uint256ToString(i *uint256.Int) string {
	if i == nil {
		return ""
	}
	return i.String()
}
