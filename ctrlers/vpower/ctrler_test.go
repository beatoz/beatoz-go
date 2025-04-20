package vpower

import (
	"bytes"
	beatozcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/libs/log"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	config    *beatozcfg.Config
	acctMock  *mocks.AcctHandlerMock
	govParams *ctrlertypes.GovParams
)

func init() {
	config = beatozcfg.DefaultConfig()
	config.SetRoot(filepath.Join(os.TempDir(), "test-vpowctrler"))
	acctMock = mocks.NewAccountHandlerMock(10000)
	acctMock.Iterate(func(idx int, w *web3.Wallet) bool {
		w.GetAccount().SetBalance(types.ToFons(1_000_000_000))
		return true
	})

	govParams = ctrlertypes.DefaultGovParams()

}

func Test_InitLedger(t *testing.T) {
	ctrler, lastValUps, xerr := initCtrler(config)
	require.NoError(t, xerr)

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)
	require.Equal(t, int64(1), lastHeight)

	require.NoError(t, ctrler.LoadLedger(lastHeight, govParams.RipeningBlocks(), int(govParams.MaxValidatorCnt())))

	require.Len(t, ctrler.allDelegatees, len(lastValUps))
	require.LessOrEqual(t, len(ctrler.lastValidators), int(govParams.MaxValidatorCnt()))

	for _, dgt := range ctrler.allDelegatees {
		require.True(t, checkExistDelegatee(dgt, lastValUps))
	}
	for _, dgt := range ctrler.lastValidators {
		require.True(t, checkExistDelegatee(dgt, lastValUps))
	}

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func Test_Bonding(t *testing.T) {
	ctrler, lastValUps, xerr := initCtrler(config)
	require.NoError(t, xerr)

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)
	require.Equal(t, int64(1), lastHeight)

	// to not validator
	txctx, xerr := makeTrxCtx(types.RandAddress(), 4000, lastHeight+1)
	require.NoError(t, xerr)
	xerr = ctrler.ValidateTrx(txctx)
	require.Error(t, xerrors.ErrNotFoundDelegatee, xerr)

	//
	// to not validator (self bonding)
	txctx, xerr = makeTrxCtx(nil, 4000, lastHeight+1)
	require.NoError(t, xerr)
	// txctx.Tx.To is not validator and nothing must be not found about txctx.Tx.To.
	dgtee0, xerr := ctrler.dgteesLedger.Get(dgteeProtoKey(txctx.Tx.To), txctx.Exec)
	require.Equal(t, xerrors.ErrNotFoundResult, xerr)
	dgtee0 = newDelegateeProto(txctx.SenderPubKey)
	//fmt.Println("validator(before)", dgtee0.Address(), dgtee0.TotalPower, dgtee0.SelfPower)
	vpow, xerr := ctrler.vpowsLedger.Get(vpowerProtoKey(txctx.Tx.From, txctx.Tx.To), true)
	require.Nil(t, vpow)
	require.Equal(t, xerrors.ErrNotFoundResult, xerr)
	//------------------------------------------------------------------------------------------------------------------

	//run tx
	xerr = ctrler.ValidateTrx(txctx)
	require.Error(t, xerrors.ErrNotFoundDelegatee, xerr)
	xerr = ctrler.ExecuteTrx(txctx)
	require.NoError(t, xerr)
	// check delegatee: the `txctx.Tx.To` should found to `dgteesLedger`.
	dgtee1, xerr := ctrler.dgteesLedger.Get(dgteeProtoKey(txctx.Tx.To), txctx.Exec)
	require.NoError(t, xerr)
	pw, _ := types.FromFons(txctx.Tx.Amount)
	require.Equal(t, dgtee0.TotalPower+int64(pw), dgtee1.TotalPower)
	require.Equal(t, dgtee0.SelfPower+int64(pw), dgtee1.SelfPower)
	//fmt.Println("validator(after)", dgtee1.Address(), dgtee1.TotalPower, dgtee1.SelfPower)
	// check vpow: the vpow of `txctx.Tx.From` should be found to `vpowsLedger`.
	vpow, xerr = ctrler.vpowsLedger.Get(vpowerProtoKey(txctx.Tx.From, txctx.Tx.To), true)
	require.NoError(t, xerr)
	require.NotNil(t, vpow)
	require.Equal(t, vpow.SumPower, vpow.sumPowerChunk())
	require.EqualValues(t, txctx.TxHash, vpow.PowerChunks[len(vpow.PowerChunks)-1].TxHash)
	require.EqualValues(t, lastHeight+1, vpow.PowerChunks[len(vpow.PowerChunks)-1].Height)
	require.EqualValues(t, pw, vpow.PowerChunks[len(vpow.PowerChunks)-1].Power)
	//------------------------------------------------------------------------------------------------------------------

	//
	// delegating to `to`: `to` is validator
	to := crypto.PubKeyBytes2Addr(lastValUps[rand.Intn(len(lastValUps))].PubKey.GetSecp256K1())
	txctx, xerr = makeTrxCtx(to, 4000, lastHeight+1)
	require.NoError(t, xerr)
	// the `txctx.Tx.To` should be found in `dgteesLedger`.
	dgtee0, xerr = ctrler.dgteesLedger.Get(dgteeProtoKey(txctx.Tx.To), txctx.Exec)
	require.NoError(t, xerr)
	// Because `dgtee0` will be updated in `ExecuteTrx`, it's origin should be copied at here.
	dgtee0 = dgtee0.Clone()
	//fmt.Println("validator(before)", dgtee0.Address(), dgtee0.TotalPower, dgtee0.SelfPower)
	// check vpow: the vpow of `txctx.TxFrom` should be not found in `vpowsLedger` yet.
	vpow, xerr = ctrler.vpowsLedger.Get(vpowerProtoKey(txctx.Tx.From, txctx.Tx.To), true)
	require.Nil(t, vpow)
	require.Equal(t, xerrors.ErrNotFoundResult, xerr)

	// run tx
	xerr = ctrler.ValidateTrx(txctx)
	require.NoError(t, xerr)
	xerr = ctrler.ExecuteTrx(txctx)
	require.NoError(t, xerr)

	// check delegatee:  the `txctx.Tx.To` should be found in `dgteesLedger`
	// and is't power should be updated by `txctx.Tx.Amount`.
	dgtee1, xerr = ctrler.dgteesLedger.Get(dgteeProtoKey(txctx.Tx.To), txctx.Exec)
	require.NoError(t, xerr)
	pw, _ = types.FromFons(txctx.Tx.Amount)
	require.Equal(t, dgtee0.TotalPower+int64(pw), dgtee1.TotalPower)
	require.Equal(t, dgtee0.SelfPower, dgtee1.SelfPower)
	//fmt.Println("validator(after)", dgtee1.Address(), dgtee1.TotalPower, dgtee1.SelfPower)
	// check vpow: the vpow of `txctx.Tx.From` should be found in `vpowsLedger`.
	vpow, xerr = ctrler.vpowsLedger.Get(vpowerProtoKey(txctx.Tx.From, txctx.Tx.To), true)
	require.NoError(t, xerr)
	require.NotNil(t, vpow)
	require.Equal(t, vpow.SumPower, vpow.sumPowerChunk())
	require.EqualValues(t, txctx.TxHash, vpow.PowerChunks[len(vpow.PowerChunks)-1].TxHash)
	require.EqualValues(t, lastHeight+1, vpow.PowerChunks[len(vpow.PowerChunks)-1].Height)
	require.EqualValues(t, pw, vpow.PowerChunks[len(vpow.PowerChunks)-1].Power)
	//------------------------------------------------------------------------------------------------------------------

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func initCtrler(cfg *beatozcfg.Config) (*VPowerCtrler, []abcitypes.ValidatorUpdate, xerrors.XError) {
	ctrler, xerr := NewVPowerCtrler(cfg, 0, log.NewNopLogger())
	if xerr != nil {
		return nil, nil, xerr
	}

	var vals []abcitypes.ValidatorUpdate
	for i := 0; i < 21; i++ {
		_, pub := crypto.NewKeypairBytes()
		pke := secp256k1.PubKey(pub)
		pkp, err := cryptoenc.PubKeyToProto(pke)
		if err != nil {
			return nil, nil, xerrors.From(err)
		}

		vals = append(vals, abcitypes.ValidatorUpdate{
			// Address:
			PubKey: pkp,
			Power:  1_000_000,
		})
	}
	xerr = ctrler.InitLedger(vals)
	if xerr != nil {
		return nil, nil, xerr
	}
	return ctrler, vals, nil
}

func checkExistDelegatee(dgt *DelegateeProto, vups []abcitypes.ValidatorUpdate) bool {
	for _, vup := range vups {
		if euqalDelegatee(dgt, vup) {
			return true
		}
	}
	return false
}
func euqalDelegatee(dgt *DelegateeProto, vup abcitypes.ValidatorUpdate) bool {
	return bytes.Equal(dgt.PubKey, vup.PubKey.GetSecp256K1()) && dgt.TotalPower == vup.Power
}

func makeTrxCtx(to types.Address, power int64, height int64) (*ctrlertypes.TrxContext, xerrors.XError) {
	from := acctMock.RandWallet()

	if to == nil {
		to = from.Address()
	}
	tx := web3.NewTrxStaking(
		from.Address(),
		to,
		from.GetNonce(),
		govParams.MinTrxGas(), govParams.GasPrice(),
		types.ToFons(uint64(power)),
	)
	if _, _, err := from.SignTrxRLP(tx, config.ChainID); err != nil {
		return nil, xerrors.From(err)
	}

	bz, xerr := tx.Encode()
	if xerr != nil {
		return nil, xerr
	}

	txCtx, xerr := ctrlertypes.NewTrxContext(bz, height, time.Now().Unix(), true,
		func(_ctx *ctrlertypes.TrxContext) xerrors.XError {
			_ctx.ChainID = config.ChainID
			_ctx.AcctHandler = acctMock
			_ctx.GovParams = govParams
			return nil
		},
	)
	if xerr != nil {
		return nil, xerr
	}
	return txCtx, nil
}
