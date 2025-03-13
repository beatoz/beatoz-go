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
	Version() int64
	MaxValidatorCnt() int64
	MinValidatorStake() *uint256.Int
	MinDelegatorStake() *uint256.Int
	// DEPRECATED
	RewardPerPower() *uint256.Int
	LazyUnstakingBlocks() int64
	LazyApplyingBlocks() int64
	GasPrice() *uint256.Int
	MinTrxGas() uint64
	MinTrxFee() *uint256.Int
	MaxTrxGas() uint64
	MaxTrxFee() *uint256.Int
	MaxBlockGas() uint64
	MinVotingPeriodBlocks() int64
	MaxVotingPeriodBlocks() int64
	MinSelfStakeRatio() int64
	MaxUpdatableStakeRatio() int64
	MaxIndividualStakeRatio() int64
	SlashRatio() int64
	SignedBlocksWindow() int64
	MinSignedBlocks() int64
}

type IAccountHandler interface {
	SetAccount(*Account, bool) xerrors.XError
	FindOrNewAccount(types.Address, bool) *Account
	FindAccount(types.Address, bool) *Account
	Transfer(types.Address, types.Address, *uint256.Int, bool) xerrors.XError
	Reward(types.Address, *uint256.Int, bool) xerrors.XError
	SimuAcctCtrlerAt(int64) (IAccountHandler, xerrors.XError)
}

type IStakeHandler interface {
	Validators() ([]*abcitypes.Validator, int64)
	IsValidator(types.Address) bool
	TotalPowerOf(types.Address) int64
	SelfPowerOf(types.Address) int64
	DelegatedPowerOf(types.Address) int64
}

type IDelegatee interface {
	GetAddress() types.Address
	GetTotalPower() int64
	GetSelfPower() int64
}
