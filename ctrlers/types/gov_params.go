package types

import (
	"encoding/base64"
	"encoding/hex"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	tmtypes "github.com/tendermint/tendermint/types"
	"google.golang.org/protobuf/proto"
	"reflect"
	"sync"
)

type GovParams struct {
	_v  GovParamsProto
	mtx sync.RWMutex
}

func DefaultGovParams() *GovParams {
	return NewGovParams(1) // 1s interval
}

func NewGovParams(interval int) *GovParams {
	// block interval = `interval` seconds
	// max blocks/1Y = 31,536,000 (if all blocks interval 1s)
	// min blocks/1Y = 31,536,000 / `interval` (if all blocks interval `interval` s)

	ret := &GovParams{
		_v: GovParamsProto{
			Version:                   1,
			AssumedBlockInterval:      int32(interval), // `interval` seconds
			MaxValidatorCnt:           21,
			MinValidatorPower:         100_000, // 100,000 BEATOZ
			MinDelegatorPower:         100,
			MaxValidatorsOfDelegator:  1,
			MaxDelegatorsOfValidator:  1000,
			MinSelfPowerRate:          50,                                // 50%
			MaxUpdatablePowerRate:     33,                                // 33%
			MaxIndividualPowerRate:    33,                                // 33%
			MinBondingBlocks:          2 * WeekSeconds / int64(interval), // 2 weeks blocks
			MinSignedBlocks:           500,
			LazyUnbondingBlocks:       2 * WeekSeconds / int64(interval),                              // 2 weeks blocks
			XMaxTotalSupply:           uint256.MustFromDecimal("700000000000000000000000000").Bytes(), // 700,000,000 BEATOZ
			InflationWeightPermil:     3,                                                              // 0.003
			InflationCycleBlocks:      WeekSeconds / int64(interval),                                  // 1 weeks blocks
			BondingBlocksWeightPermil: 500,                                                            // 0.500
			RipeningBlocks:            YearSeconds / int64(interval),                                  // one year blocks
			XRewardPoolAddress:        types.ZeroAddress(),                                            // zero address
			XDeadAddress:              types.DeadAddress(),                                            // zero address
			ValidatorRewardRate:       30,                                                             // 30%
			TxFeeRewardRate:           90,                                                             // 90%
			SlashRate:                 50,                                                             // 50%
			XGasPrice:                 uint256.NewInt(48_000_000_000).Bytes(),                         // 48e9 * 21e3(evm_tx_gas) = 1008e12 = 0.001008 BTOZ
			MinTrxGas:                 5_000,                                                          // 5e3 * 48e9 = 240e12 = 0.00024 BTOZ
			MinBlockGasLimit:          36_000_000,
			MaxBlockGasLimit:          150_000_000,
			MinVotingPeriodBlocks:     DaySeconds / int64(interval),     // 1 days blocks
			MaxVotingPeriodBlocks:     7 * DaySeconds / int64(interval), // 7 day blocks
			LazyApplyingBlocks:        DaySeconds / int64(interval),     // 1days blocks
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

	jz, err := jsonx.Marshal(&govParams._v)
	if err != nil {
		return nil, err
	}

	tmp := make(map[string]interface{})
	if err := jsonx.Unmarshal(jz, &tmp); err != nil {
		return nil, err
	}
	for k, v := range tmp {
		if k == "maxTotalSupply" || k == "gasPrice" {
			// v is base64 string
			_v, err := base64.StdEncoding.DecodeString(v.(string))
			if err != nil {
				return nil, err
			}
			tmp[k] = new(uint256.Int).SetBytes(_v).String() // decimal string
		} else if k == "deadAddress" || k == "rewardPoolAddress" {
			// v is base64 string
			_v, err := base64.StdEncoding.DecodeString(v.(string))
			if err != nil {
				return nil, err
			}
			tmp[k] = types.Address(_v).String()
		}
	}

	return jsonx.Marshal(tmp)
}

func (govParams *GovParams) UnmarshalJSON(d []byte) error {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	tmp := make(map[string]interface{})
	if err := jsonx.Unmarshal(d, &tmp); err != nil {
		return err
	}

	for k, v := range tmp {
		if k == "maxTotalSupply" || k == "gasPrice" {
			tmp[k] = base64.StdEncoding.EncodeToString(uint256.MustFromDecimal(v.(string)).Bytes())
		} else if k == "deadAddress" || k == "rewardPoolAddress" {
			_v, err := hex.DecodeString(v.(string))
			if err != nil {
				return err
			}
			tmp[k] = base64.StdEncoding.EncodeToString(_v)
		}
	}
	jz, err := jsonx.Marshal(tmp)
	if err != nil {
		return err
	}

	return jsonx.Unmarshal(jz, &govParams._v)
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
func (govParams *GovParams) MinSignedBlocks() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MinSignedBlocks
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

func (govParams *GovParams) AssumedBlockInterval() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.AssumedBlockInterval
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
func (govParams *GovParams) DeadAddress() types.Address {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return types.Address(govParams._v.XDeadAddress).Copy()
}
func (govParams *GovParams) ValidatorRewardRate() int32 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.ValidatorRewardRate
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
func (govParams *GovParams) MinBlockGasLimit() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MinBlockGasLimit
}
func (govParams *GovParams) MaxBlockGasLimit() int64 {
	govParams.mtx.RLock()
	defer govParams.mtx.RUnlock()

	return govParams._v.MaxBlockGasLimit
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

	if bz, err := jsonx.MarshalIndent(govParams, "", "  "); err != nil {
		return err.Error()
	} else {
		return string(bz)
	}
}

// utility methods
func MaxTotalPower() int64 {
	return tmtypes.MaxTotalVotingPower
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
