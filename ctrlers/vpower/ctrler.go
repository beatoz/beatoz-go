package vpower

import (
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"sync"
)

type VPowerCtrler struct {
	vpowLedger v1.IStateLedger[*VPower]

	logger tmlog.Logger
	mtx    sync.RWMutex
}

func NewVPowerCtrler(config *cfg.Config, logger tmlog.Logger) (*VPowerCtrler, xerrors.XError) {
	lg := logger.With("module", "beatoz_VPowerCtrler")

	vpowLedger, xerr := v1.NewStateLedger[*VPower]("vpowers", config.DBDir(), 2048, func() *VPower { return &VPower{} }, lg)
	if xerr != nil {
		return nil, xerr
	}

	return &VPowerCtrler{
		vpowLedger: vpowLedger,
		logger:     lg,
	}, nil
}

func (ctrler *VPowerCtrler) InitLedger(req interface{}) xerrors.XError {
	// init validators
	ctrler.mtx.Lock()
	defer ctrler.mtx.Unlock()

	//initValidators, ok := req.([]abcitypes.ValidatorUpdate)
	//if !ok {
	//	return xerrors.ErrInitChain.Wrapf("wrong parameter: StakeCtrler::InitLedger() requires []*InitStake")
	//}

	//for _, val := range initValidators {
	//	for _, s0 := range initS0.Stakes {
	//		d := NewDelegatee(s0.To, initS0.PubKeys)
	//		if xerr := d.AddStake(s0); xerr != nil {
	//			return xerr
	//		}
	//		if xerr := ctrler.delegateeLedger.Set(d, true); xerr != nil {
	//			return xerr
	//		}
	//	}
	//}

	return nil
}

func (ctrler *VPowerCtrler) BeginBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) ValidateTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) ExecuteTrx(ctx *ctrlertypes.TrxContext) xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) EndBlock(bctx *ctrlertypes.BlockContext) ([]abcitypes.Event, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) Commit() ([]byte, int64, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) Close() xerrors.XError {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) Validators() ([]*abcitypes.Validator, int64) {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) IsValidator(addr types.Address) bool {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) TotalPowerOf(addr types.Address) int64 {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) SelfPowerOf(addr types.Address) int64 {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) DelegatedPowerOf(addr types.Address) int64 {
	//TODO implement me
	panic("implement me")
}

func (ctrler *VPowerCtrler) Query(query abcitypes.RequestQuery) ([]byte, xerrors.XError) {
	//TODO implement me
	panic("implement me")
}

var _ ctrlertypes.ILedgerHandler = (*VPowerCtrler)(nil)
var _ ctrlertypes.ITrxHandler = (*VPowerCtrler)(nil)
var _ ctrlertypes.IBlockHandler = (*VPowerCtrler)(nil)
var _ ctrlertypes.IStakeHandler = (*VPowerCtrler)(nil)
