package stake_test

import (
	"fmt"
	beatozcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/ctrlers"
	"github.com/beatoz/beatoz-go/ctrlers/stake"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/stretchr/testify/require"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	acctMock01    *ctrlers.AcctHandlerMock
	govParams01   ctrlertypes.IGovParams
	stakeCtrler01 *stake.StakeCtrler
	validatorCnt  = 0
)

func TestLimitStaking(t *testing.T) {
	wrongIndividualLimit(t)
	wrongUpdatableLimit_ByNewValidator_Staking(t)
	wrongUpdatableLimit_ByNewValidator_Delegating(t)
	wrongUpdatableLimit_ByUnstaking(t)
}

func wrongIndividualLimit(t *testing.T) {

	resetTest(t, 3)

	//bctx := mocks.NextBlockCtx()
	//_, xerr := stakeCtrler01.BeginBlock(bctx)
	//require.NoError(t, xerr)
	require.NoError(t, mocks.DoBeginBlock(stakeCtrler01))

	//
	// for existing validator
	w := acctMock01.GetWallet(0)

	// previous total power = 30_000_000
	// previous power       = 10_000_000
	// added power          = 400_000
	// (10_000_000 + 400_000) * 100 / (30_000_000 + 400_000) = 34(34.21052632) > 33(IndividualLimitRatio)
	tx := web3.NewTrxStaking(w.Address(), w.Address(), w.GetNonce(), govParams01.MinTrxGas(), govParams01.GasPrice(), types.ToFons(uint64(400_000)))
	_, _, err := w.SignTrxRLP(tx, "test-chain")
	require.NoError(t, err)

	txbz, err := tx.Encode()
	require.NoError(t, err)

	txctx, xerr := ctrlertypes.NewTrxContext(txbz, mocks.LastBlockHeight(), time.Now().UnixNano(), true, func(ctx *ctrlertypes.TrxContext) xerrors.XError {
		ctx.AcctHandler = acctMock01
		ctx.GovParams = govParams01
		ctx.ChainID = "test-chain"
		return nil
	})
	require.NoError(t, xerr)

	xerr = stakeCtrler01.ValidateTrx(txctx)
	require.Error(t, xerr)
	require.Equal(t, xerrors.ErrUpdatableStakeRatio.Code(), xerr.Code())

	//
	// for new validator
	w = acctMock01.GetWallet(validatorCnt)

	//
	// Fail expected
	// 15_460_000 * 100 / (30_000_000 + 15_460_000) = 34(34.00791905) > 33(IndividualLimitRatio)
	tx = web3.NewTrxStaking(w.Address(), w.Address(), w.GetNonce(), govParams01.MinTrxGas(), govParams01.GasPrice(), types.ToFons(uint64(15_460_000)))
	_, _, err = w.SignTrxRLP(tx, "test-chain")
	require.NoError(t, err)

	txbz, err = tx.Encode()
	require.NoError(t, err)

	txctx, xerr = ctrlertypes.NewTrxContext(txbz, mocks.LastBlockHeight(), time.Now().UnixNano(), true, func(ctx *ctrlertypes.TrxContext) xerrors.XError {
		ctx.AcctHandler = acctMock01
		ctx.GovParams = govParams01
		ctx.ChainID = "test-chain"
		return nil
	})
	require.NoError(t, xerr)

	xerr = stakeCtrler01.ValidateTrx(txctx)
	require.Error(t, xerr)
	require.Equal(t, xerrors.ErrUpdatableStakeRatio.Code(), xerr.Code())

	//
	// Success expected
	// 15_450_000 * 100 / (30_000_000 + 15_450_000) = 33(33.99339934) == 33(IndividualLimitRatio)
	tx = web3.NewTrxStaking(w.Address(), w.Address(), w.GetNonce(), govParams01.MinTrxGas(), govParams01.GasPrice(), types.ToFons(uint64(15_450_000)))
	_, _, err = w.SignTrxRLP(tx, "test-chain")
	require.NoError(t, err)

	txbz, err = tx.Encode()
	require.NoError(t, err)

	txctx, xerr = ctrlertypes.NewTrxContext(txbz, mocks.LastBlockHeight(), time.Now().UnixNano(), true, func(ctx *ctrlertypes.TrxContext) xerrors.XError {
		ctx.AcctHandler = acctMock01
		ctx.GovParams = govParams01
		ctx.ChainID = "test-chain"
		return nil
	})
	require.NoError(t, xerr)

	require.NoError(t, stakeCtrler01.ValidateTrx(txctx))
	require.NoError(t, stakeCtrler01.ExecuteTrx(txctx))

	//
	// Fail expected
	// Updated power of `w` is 15_450_000 at now by previous tx.
	// Additional power update(10_000) should be fail.
	tx = web3.NewTrxStaking(w.Address(), w.Address(), w.GetNonce(), govParams01.MinTrxGas(), govParams01.GasPrice(), types.ToFons(uint64(10_000)))
	_, _, err = w.SignTrxRLP(tx, "test-chain")
	require.NoError(t, err)

	txbz, err = tx.Encode()
	require.NoError(t, err)

	txctx, xerr = ctrlertypes.NewTrxContext(txbz, mocks.LastBlockHeight(), time.Now().UnixNano(), true, func(ctx *ctrlertypes.TrxContext) xerrors.XError {
		ctx.AcctHandler = acctMock01
		ctx.GovParams = govParams01
		ctx.ChainID = "test-chain"
		return nil
	})
	require.NoError(t, xerr)

	xerr = stakeCtrler01.ValidateTrx(txctx)
	require.Error(t, xerr)
	require.Equal(t, xerrors.ErrUpdatableStakeRatio.Code(), xerr.Code())

	//
	// EndBlock and Commit
	//_, xerr = stakeCtrler01.EndBlock(bctx)
	//require.NoError(t, xerr)
	//_, _, xerr = stakeCtrler01.Commit()
	//require.NoError(t, xerr)
	require.NoError(t, mocks.DoEndBlockCommit(stakeCtrler01))
}

func wrongUpdatableLimit_ByNewValidator_Staking(t *testing.T) {

	resetTest(t, int(govParams01.MaxValidatorCnt()))

	//bctx := mocks.NextBlockCtx()
	//_, xerr := stakeCtrler01.BeginBlock(bctx)
	//require.NoError(t, xerr)
	require.NoError(t, mocks.DoBeginBlock(stakeCtrler01))

	//
	// the total validator's power is 100_000_000. (when validator count is 10)
	// the updatable limit is 30_000_000. (when updatable limit ratio is 30%)

	for i := 0; i < 3; i++ {
		w := acctMock01.GetWallet(validatorCnt + i) // this is not validator
		//
		// new validator
		// the power 10_000_000 is excluded by the following tx.
		tx := web3.NewTrxStaking(w.Address(), w.Address(), w.GetNonce(), govParams01.MinTrxGas(), govParams01.GasPrice(), types.ToFons(uint64(10_000_001)))
		_, _, err := w.SignTrxRLP(tx, "test-chain")
		require.NoError(t, err)

		txbz, err := tx.Encode()
		require.NoError(t, err)

		txctx, xerr := ctrlertypes.NewTrxContext(txbz, mocks.LastBlockHeight(), time.Now().UnixNano(), true, func(ctx *ctrlertypes.TrxContext) xerrors.XError {
			ctx.AcctHandler = acctMock01
			ctx.GovParams = govParams01
			ctx.ChainID = "test-chain"
			return nil
		})
		require.NoError(t, xerr)

		require.NoError(t, stakeCtrler01.ValidateTrx(txctx), fmt.Sprintf("index: %v", i))
		require.NoError(t, stakeCtrler01.ExecuteTrx(txctx), fmt.Sprintf("index: %v", i))
	}

	w := acctMock01.GetWallet(validatorCnt + 4)
	//
	// the power 10_000_000 is excluded by the following tx.
	tx := web3.NewTrxStaking(w.Address(), w.Address(), w.GetNonce(), govParams01.MinTrxGas(), govParams01.GasPrice(), types.ToFons(uint64(10_000_001)))
	_, _, err := w.SignTrxRLP(tx, "test-chain")
	require.NoError(t, err)

	txbz, err := tx.Encode()
	require.NoError(t, err)

	txctx, xerr := ctrlertypes.NewTrxContext(txbz, mocks.LastBlockHeight(), time.Now().UnixNano(), true, func(ctx *ctrlertypes.TrxContext) xerrors.XError {
		ctx.AcctHandler = acctMock01
		ctx.GovParams = govParams01
		ctx.ChainID = "test-chain"
		return nil
	})
	require.NoError(t, xerr)
	xerr = stakeCtrler01.ValidateTrx(txctx)
	require.Error(t, xerr)
	require.Equal(t, xerrors.ErrUpdatableStakeRatio.Code(), xerr.Code())

	//_, xerr = stakeCtrler01.EndBlock(bctx)
	//require.NoError(t, xerr)
	//_, _, xerr = stakeCtrler01.Commit()
	//require.NoError(t, xerr)
	require.NoError(t, mocks.DoEndBlockCommit(stakeCtrler01))
}

func wrongUpdatableLimit_ByNewValidator_Delegating(t *testing.T) {

	resetTest(t, int(govParams01.MaxValidatorCnt()))

	//bctx := mocks.NextBlockCtx()
	//_, xerr := stakeCtrler01.BeginBlock(bctx)
	//require.NoError(t, xerr)
	require.NoError(t, mocks.DoBeginBlock(stakeCtrler01))

	//
	// the total validator's power is 100_000_000. (when validator count is 10)
	// the updatable limit is 30_000_000. (when updatable limit ratio is 30%)

	// make 4 candidates
	var candiWals []*web3.Wallet
	cnt := (validatorCnt * 1 / 3) + 1
	for i := 0; i < cnt; i++ {
		w := acctMock01.GetWallet(validatorCnt + i) // this is not validator
		//
		// new candidate
		tx := web3.NewTrxStaking(w.Address(), w.Address(), w.GetNonce(), govParams01.MinTrxGas(), govParams01.GasPrice(), types.ToFons(uint64(9_999_999)))
		_, _, err := w.SignTrxRLP(tx, "test-chain")
		require.NoError(t, err)

		txbz, err := tx.Encode()
		require.NoError(t, err)

		txctx, xerr := ctrlertypes.NewTrxContext(txbz, mocks.LastBlockHeight(), time.Now().UnixNano(), true, func(ctx *ctrlertypes.TrxContext) xerrors.XError {
			ctx.AcctHandler = acctMock01
			ctx.GovParams = govParams01
			ctx.ChainID = "test-chain"
			return nil
		})
		require.NoError(t, xerr)

		require.NoError(t, stakeCtrler01.ValidateTrx(txctx), fmt.Sprintf("index: %v", i))
		require.NoError(t, stakeCtrler01.ExecuteTrx(txctx), fmt.Sprintf("index: %v", i))

		candiWals = append(candiWals, w)
	}

	wallets := acctMock01.GetAllWallets()[validatorCnt+len(candiWals):]

	for i := 0; i < cnt-1; i++ {
		candidate := candiWals[i]
		w := wallets[i]

		// candidate power = 9_999_999
		// delegating to candidate = 2
		// candidate final power = 10_000_001. it makes a validator to be excluded.
		tx := web3.NewTrxStaking(w.Address(), candidate.Address(), w.GetNonce(), govParams01.MinTrxGas(), govParams01.GasPrice(), types.ToFons(uint64(2)))
		_, _, err := w.SignTrxRLP(tx, "test-chain")
		require.NoError(t, err)

		txbz, err := tx.Encode()
		require.NoError(t, err)

		txctx, xerr := ctrlertypes.NewTrxContext(txbz, mocks.LastBlockHeight(), time.Now().UnixNano(), true, func(ctx *ctrlertypes.TrxContext) xerrors.XError {
			ctx.AcctHandler = acctMock01
			ctx.GovParams = govParams01
			ctx.ChainID = "test-chain"
			return nil
		})
		require.NoError(t, xerr)

		require.NoError(t, stakeCtrler01.ValidateTrx(txctx), fmt.Sprintf("index: %v", i))
		require.NoError(t, stakeCtrler01.ExecuteTrx(txctx), fmt.Sprintf("index: %v", i))
	}

	candidate := candiWals[cnt-1]
	w := wallets[cnt-1]

	// candidate power = 9_999_999
	// delegating to candidate = 2
	// candidate final power = 10_000_001. it makes a validator to be excluded.
	tx := web3.NewTrxStaking(w.Address(), candidate.Address(), w.GetNonce(), govParams01.MinTrxGas(), govParams01.GasPrice(), types.ToFons(uint64(2)))
	_, _, err := w.SignTrxRLP(tx, "test-chain")
	require.NoError(t, err)

	txbz, err := tx.Encode()
	require.NoError(t, err)

	txctx, xerr := ctrlertypes.NewTrxContext(txbz, mocks.LastBlockHeight(), time.Now().UnixNano(), true, func(ctx *ctrlertypes.TrxContext) xerrors.XError {
		ctx.AcctHandler = acctMock01
		ctx.GovParams = govParams01
		ctx.ChainID = "test-chain"
		return nil
	})
	require.NoError(t, xerr)
	xerr = stakeCtrler01.ValidateTrx(txctx)
	require.Error(t, xerr)
	require.Equal(t, xerrors.ErrUpdatableStakeRatio.Code(), xerr.Code())

	//_, xerr = stakeCtrler01.EndBlock(bctx)
	//require.NoError(t, xerr)
	//_, _, xerr = stakeCtrler01.Commit()
	//require.NoError(t, xerr)
	require.NoError(t, mocks.DoEndBlockCommit(stakeCtrler01))
}

func wrongUpdatableLimit_ByUnstaking(t *testing.T) {

	resetTest(t, 3)

	//bctx := mocks.NextBlockCtx()
	//_, xerr := stakeCtrler01.BeginBlock(bctx)
	//require.NoError(t, xerr)
	require.NoError(t, mocks.DoBeginBlock(stakeCtrler01))

	// staking...
	var trxCtxs []*ctrlertypes.TrxContext
	for i := validatorCnt; int64(i) < govParams01.MaxValidatorCnt(); i++ {
		w := acctMock01.GetWallet(i)

		tx := web3.NewTrxStaking(w.Address(), w.Address(), w.GetNonce(), govParams01.MinTrxGas(), govParams01.GasPrice(), types.ToFons(uint64(10_000_000)))
		_, _, err := w.SignTrxRLP(tx, "test-chain")
		require.NoError(t, err)

		txbz, err := tx.Encode()
		require.NoError(t, err)

		txctx, xerr := ctrlertypes.NewTrxContext(txbz, mocks.LastBlockHeight(), time.Now().UnixNano(), true, func(ctx *ctrlertypes.TrxContext) xerrors.XError {
			ctx.AcctHandler = acctMock01
			ctx.GovParams = govParams01
			ctx.ChainID = "test-chain"
			return nil
		})
		require.NoError(t, xerr)

		require.NoError(t, stakeCtrler01.ValidateTrx(txctx), fmt.Sprintf("index: %v", i))
		require.NoError(t, stakeCtrler01.ExecuteTrx(txctx), fmt.Sprintf("index: %v", i))

		trxCtxs = append(trxCtxs, txctx)
	}

	//_, xerr = stakeCtrler01.EndBlock(bctx)
	//require.NoError(t, xerr)
	//_, _, xerr = stakeCtrler01.Commit()
	//require.NoError(t, xerr)
	require.NoError(t, mocks.DoEndBlockCommit(stakeCtrler01))

	//bctx = mocks.NextBlockCtx()
	//_, xerr = stakeCtrler01.BeginBlock(bctx)
	//require.NoError(t, xerr)
	require.NoError(t, mocks.DoBeginBlock(stakeCtrler01))

	for i, ctx := range trxCtxs {
		w := acctMock01.FindWallet(ctx.Sender.Address)
		require.NotNil(t, w)

		tx := web3.NewTrxUnstaking(w.Address(), w.Address(), w.GetNonce(), govParams01.MinTrxGas(), govParams01.GasPrice(), ctx.TxHash)
		_, _, err := w.SignTrxRLP(tx, "test-chain")
		require.NoError(t, err)

		txbz, err := tx.Encode()
		require.NoError(t, err)

		txctx, xerr := ctrlertypes.NewTrxContext(txbz, mocks.LastBlockHeight(), time.Now().UnixNano(), true, func(ctx *ctrlertypes.TrxContext) xerrors.XError {
			ctx.AcctHandler = acctMock01
			ctx.GovParams = govParams01
			ctx.ChainID = "test-chain"
			return nil
		})
		require.NoError(t, xerr)

		if i <= 2 {
			// unstaking: 30_000_000, basePower: 100_000_000, ratio: 30, when i==2
			require.NoError(t, stakeCtrler01.ValidateTrx(txctx), fmt.Sprintf("index: %v, txhash:%v", i, ctx.TxHash))
			require.NoError(t, stakeCtrler01.ExecuteTrx(txctx), fmt.Sprintf("index: %v, txhash:%v", i, ctx.TxHash))
		} else {
			// unstaking: 30_000_000+10_000_000, basePower: 100_000_000, ratio: 40
			xerr := stakeCtrler01.ValidateTrx(txctx)
			require.Error(t, xerr)
			require.Equal(t, xerrors.ErrUpdatableStakeRatio.Code(), xerr.Code())
			break
		}
	}

	//_, xerr = stakeCtrler01.EndBlock(bctx)
	//require.NoError(t, xerr)
	//_, _, xerr = stakeCtrler01.Commit()
	//require.NoError(t, xerr)
	require.NoError(t, mocks.DoEndBlockCommit(stakeCtrler01))
}

func resetTest(t *testing.T, valCnt int) {
	acctMock01 = ctrlers.NewAccountHandlerMock(100)
	acctMock01.Iterate(func(idx int, w *web3.Wallet) bool {
		w.GetAccount().SetBalance(types.ToFons(1_000_000_000))
		return true
	})

	// create stakek controller
	cfg := beatozcfg.DefaultConfig()
	cfg.DBPath = filepath.Join(os.TempDir(), "stake-limiter-test")
	os.RemoveAll(cfg.DBPath)

	govParams01 = ctrlertypes.Test1GovParams()

	ctrler, xerr := stake.NewStakeCtrler(cfg, govParams01, tmlog.NewNopLogger())
	require.NoError(t, xerr)

	stakeCtrler01 = ctrler

	genesisStaking(t, valCnt)
}

func genesisStaking(t *testing.T, cnt int) {
	validatorCnt = 0
	power0 := int64(10_000_000)
	totalPower := int64(0)
	// gensis staking

	bctx := mocks.InitBlockCtxWith(1, acctMock01, govParams01, nil)

	//_, xerr := stakeCtrler01.BeginBlock(bctx)
	//require.NoError(t, xerr)
	require.NoError(t, mocks.DoBeginBlock(stakeCtrler01))

	for i, w := range acctMock01.GetAllWallets() {
		if i == cnt {
			break
		}
		tx := web3.NewTrxStaking(w.Address(), w.Address(), w.GetNonce(), govParams01.MinTrxGas(), govParams01.GasPrice(), types.ToFons(uint64(power0)))
		_, _, err := w.SignTrxRLP(tx, "test-chain")
		require.NoError(t, err)

		txbz, err := tx.Encode()
		require.NoError(t, err)

		txctx, xerr := ctrlertypes.NewTrxContext(txbz, bctx.Height(), time.Now().UnixNano(), true, func(ctx *ctrlertypes.TrxContext) xerrors.XError {
			ctx.AcctHandler = acctMock01
			ctx.GovParams = govParams01
			ctx.ChainID = "test-chain"
			return nil
		})
		require.NoError(t, xerr)
		require.NoError(t, stakeCtrler01.ValidateTrx(txctx))
		require.NoError(t, stakeCtrler01.ExecuteTrx(txctx))

		validatorCnt++
		totalPower += power0
	}
	//_, xerr = stakeCtrler01.EndBlock(bctx)
	//require.NoError(t, xerr)
	//_, h, xerr := stakeCtrler01.Commit()
	//require.Equal(t, bctx.Height(), h)
	//require.NoError(t, xerr)
	require.NoError(t, mocks.DoEndBlockCommit(stakeCtrler01))

	for i, w := range acctMock01.GetAllWallets() {
		if i < validatorCnt {
			require.Equal(t, power0, stakeCtrler01.TotalPowerOf(w.Address()), fmt.Sprintf("index:%d, address:%v", i, w.Address()))
		} else {
			require.Equal(t, int64(0), stakeCtrler01.TotalPowerOf(w.Address()), fmt.Sprintf("index:%d, address:%v", i, w.Address()))
		}
	}

	//bctx = mocks.NextBlockCtx()
	//_, _ = stakeCtrler01.BeginBlock(bctx)
	require.NoError(t, mocks.DoBeginBlock(stakeCtrler01))

	//_, _ = stakeCtrler01.EndBlock(bctx) // at here, stakeCtrler01.lastValidators is set.
	//_, h, _ = stakeCtrler01.Commit()
	//require.Equal(t, bctx.Height(), h)
	require.NoError(t, mocks.DoEndBlockCommit(stakeCtrler01))

	vals, tp := stakeCtrler01.Validators()
	require.Equal(t, totalPower, tp)
	require.Equal(t, validatorCnt, len(vals))
	for _, v := range vals {
		require.Equal(t, power0, v.Power)
	}
}
