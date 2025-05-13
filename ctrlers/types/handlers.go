package types

import (
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

type ILedgerHandler interface {
	InitLedger(interface{}) xerrors.XError
	Commit() ([]byte, int64, xerrors.XError)
	Query(abcitypes.RequestQuery) ([]byte, xerrors.XError)
	Close() xerrors.XError
}

type IBlockHandler interface {
	BeginBlock(*BlockContext) ([]abcitypes.Event, xerrors.XError)
	EndBlock(*BlockContext) ([]abcitypes.Event, xerrors.XError)
}

type ITrxHandler interface {
	ValidateTrx(*TrxContext) xerrors.XError
	ExecuteTrx(*TrxContext) xerrors.XError
}

type IGovParams interface {
	Version() int32
	MaxValidatorCnt() int32
	MinValidatorPower() int64
	MinDelegatorPower() int64
	MaxValidatorsOfDelegator() int32
	MaxDelegatorsOfValidator() int32
	MinSelfStakeRate() int32
	MaxUpdatableStakeRate() int32
	MaxIndividualStakeRate() int32
	MinBondingBlocks() int64
	LazyUnstakingBlocks() int64
	MaxTotalSupply() *uint256.Int
	InflationWeightPermil() int32
	InflationCycleBlocks() int64
	BondingBlocksWeightPermil() int32
	RipeningBlocks() int64
	RewardPoolAddress() types.Address
	ValidatorRewardRate() int32
	BurnAddress() types.Address
	BurnRate() int32
	SlashRate() int32
	SignedBlocksWindow() int64
	MinSignedBlocks() int64
	GasPrice() *uint256.Int
	MinTrxFee() *uint256.Int
	MinTrxGas() int64
	MaxTrxGas() int64
	MaxBlockGas() int64
	MaxVotingPeriodBlocks() int64
	MinVotingPeriodBlocks() int64
	LazyApplyingBlocks() int64

	// deprecated
	RewardPerPower() *uint256.Int
}

type IGovHandler interface {
	IGovParams
	ITrxHandler
	IBlockHandler
}

type IAccountHandler interface {
	ITrxHandler
	IBlockHandler
	SetAccount(*Account, bool) xerrors.XError
	FindOrNewAccount(types.Address, bool) *Account
	FindAccount(types.Address, bool) *Account
	Transfer(types.Address, types.Address, *uint256.Int, bool) xerrors.XError
	// DEPRECATED: Add `AddBlance` and replace it.
	Reward(types.Address, *uint256.Int, bool) xerrors.XError
	SimuAcctCtrlerAt(int64) (IAccountHandler, xerrors.XError)
}

type IStakeHandler interface {
	ITrxHandler
	IBlockHandler
	Validators() ([]*abcitypes.Validator, int64)
	IsValidator(types.Address) bool
	TotalPowerOf(types.Address) int64
	SelfPowerOf(types.Address) int64
	DelegatedPowerOf(types.Address) int64
}

type IEVMHandler interface {
	ITrxHandler
	IBlockHandler
}

type IVPowerHandler interface {
	IStakeHandler
	ITrxHandler
	IBlockHandler
	//Validators() ([]*vpower.Delegatee, int64)
	//IsValidator(types.Address) bool
	//TotalPowerOf(types.Address) int64
	//SelfPowerOf(types.Address) int64
	//DelegatedPowerOf(types.Address) int64
	ComputeWeight(int64, int64, int64, int32, *uint256.Int) (*Weight, xerrors.XError)
}

type ISupplyHandler interface {
	ITrxHandler
	IBlockHandler
	RequestMint(bctx *BlockContext)
	Burn(bctx *BlockContext, amt *uint256.Int) xerrors.XError
}
