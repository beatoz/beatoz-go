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
	"reflect"
	"sync"
	"unicode"
)

var (
	//DEPRECATED
	amountPerPower = uint256.NewInt(1_000000000_000000000) // 1BEATOZ == 1Power
)

type GovParams struct {
	_v  GovParamsProto
	mtx sync.RWMutex
}

func DefaultGovParams() *GovParams {
	return newGovParamsWith(1) // 7s interval
}

func newGovParamsWith(interval int) *GovParams {
	// block interval = `interval` seconds
	// max blocks/1Y = 31,536,000 (if all blocks interval 1s)
	// min blocks/1Y = 31,536,000 / `interval` (if all blocks interval `interval` s)

	ret := &GovParams{
		_v: GovParamsProto{
			Version:                   1,
			MaxValidatorCnt:           21,
			MinValidatorPower:         100_000, // 100,000 BEATOZ
			MinDelegatorPower:         100,
			MaxValidatorsOfDelegator:  1,
			MaxDelegatorsOfValidator:  1000,
			MinSelfPowerRate:          50,                                                             // 50%
			MaxUpdatablePowerRate:     33,                                                             // 33%
			MaxIndividualPowerRate:    33,                                                             // 33%
			MinBondingBlocks:          2 * WeekSeconds / int64(interval),                              // 2 weeks blocks
			LazyUnbondingBlocks:       2 * WeekSeconds / int64(interval),                              // 2 weeks blocks
			XMaxTotalSupply:           uint256.MustFromDecimal("700000000000000000000000000").Bytes(), // 700,000,000 BEATOZ
			InflationWeightPermil:     390,                                                            // 0.390
			InflationCycleBlocks:      WeekSeconds / int64(interval),                                  // 1 weeks blocks
			BondingBlocksWeightPermil: 500,                                                            // 0.500
			RipeningBlocks:            WeekSeconds / int64(interval),                                  // one year blocks
			XRewardPoolAddress:        types.ZeroAddress(),                                            // zero address
			ValidatorRewardRate:       30,                                                             // 30%
			XDeadAddress:              types.DeadAddress(),                                            // zero address
			TxFeeRewardRate:           90,                                                             // 90%
			SlashRate:                 50,                                                             // 50%
			SignedBlocksWindow:        10_000,
			MinSignedBlocks:           500,
			XGasPrice:                 uint256.NewInt(250_000_000_000).Bytes(), // 250e9 = 250 Gfons
			MinTrxGas:                 4_000,                                   // 4e3 * 25e10 = 1e15 = 0.001 BEATOZ
			MaxTrxGas:                 30_000_000,
			MaxBlockGas:               50_000_000,
			MinVotingPeriodBlocks:     DaySeconds / int64(interval),     // 1 days blocks
			MaxVotingPeriodBlocks:     7 * DaySeconds / int64(interval), // 7 day blocks
			LazyApplyingBlocks:        DaySeconds / int64(interval),     // 1days blocks
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
		} else if k == "deadAddress" || k == "rewardPoolAddress" || k == "rewardPerStake" {
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
func (govParams *GovParams) MinSelfPowerRate() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MinSelfPowerRate
}
func (govParams *GovParams) MaxUpdatablePowerRate() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MaxUpdatablePowerRate
}
func (govParams *GovParams) MaxIndividualPowerRate() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MaxIndividualPowerRate
}
func (govParams *GovParams) MinBondingBlocks() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MinBondingBlocks
}
func (govParams *GovParams) LazyUnbondingBlocks() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.LazyUnbondingBlocks
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
func (govParams *GovParams) DeadAddress() types.Address {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return types.Address(govParams._v.XDeadAddress).Copy()
}
func (govParams *GovParams) TxFeeRewardRate() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.TxFeeRewardRate
}
func (govParams *GovParams) SlashRate() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.SlashRate
}

// DEPRECATED: Use InflationCycleBlocks() instead.
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

func MergeGovParams(fromPrams, toParams *GovParams) {
	refT := reflect.TypeOf(GovParamsProto{})
	refVOld := reflect.ValueOf(fromPrams.GetValues()).Elem()
	refVNew := reflect.ValueOf(toParams.GetValues()).Elem()

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
type setFunc func(*GovParamsProto)

func (govParams *GovParams) SetValue(cb setFunc) {
	cb(&govParams._v)
}
