package vpower

import (
	"bytes"
	"encoding/binary"
	"fmt"
	beatozcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/types"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/libs/log"
	"math"
	"math/rand"
	"os"
	"sort"
	"testing"
	"time"
)

func Test_InitLedger(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, lastValUps, valWallets, xerr := initLedger(config)
	require.NoError(t, xerr)
	require.Equal(t, len(lastValUps), len(valWallets))

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)
	require.Equal(t, int64(1), lastHeight)

	totalPower0 := int64(0)
	for _, vup := range lastValUps {
		totalPower0 += vup.Power
	}

	totalPower1 := int64(0)
	xerr = ctrler.powersState.Seek(v1.KeyPrefixDelegatee, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		dgt, _ := item.(*Delegatee)
		require.EqualValues(t, v1.LedgerKeyDelegatee(dgt.addr, nil), key)
		require.EqualValues(t, v1.LedgerKeyDelegatee(dgt.addr, nil), dgt.key)

		var valUp *abcitypes.ValidatorUpdate
		var wallet *web3.Wallet
		for i, w := range valWallets {
			if bytes.Equal(w.Address(), dgt.addr) {
				// do not break to check that `dgt` is duplicated.
				// if valUp and wallet is not nil, it means that the `dgt` is duplicated.
				require.Nil(t, valUp)
				require.Nil(t, wallet)
				valUp = &lastValUps[i]
				wallet = valWallets[i]
			}
		}
		require.NotNil(t, valUp)
		require.NotNil(t, wallet)
		require.EqualValues(t, valUp.PubKey.GetSecp256K1(), dgt.PubKey)
		require.EqualValues(t, crypto.PubKeyBytes2Addr(dgt.PubKey), dgt.addr)
		require.EqualValues(t, wallet.Address(), dgt.addr)
		require.Equal(t, valUp.Power, dgt.SumPower)
		require.Equal(t, valUp.Power, dgt.SelfPower)
		require.Equal(t, 1, len(dgt.Delegators))
		require.EqualValues(t, dgt.addr, dgt.Delegators[0])

		totalPower1 += dgt.SumPower
		return nil
	}, true)

	totalPower2 := int64(0)
	xerr = ctrler.powersState.Seek(v1.KeyPrefixVPower, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		vpow, _ := item.(*VPower)
		require.EqualValues(t, v1.LedgerKeyVPower(vpow.From, vpow.to), key)
		require.EqualValues(t, v1.LedgerKeyVPower(vpow.From, vpow.to), vpow.key)

		var valUp *abcitypes.ValidatorUpdate
		var wallet *web3.Wallet
		for i, vup := range lastValUps {
			if bytes.Equal(vup.PubKey.GetSecp256K1(), vpow.PubKeyTo) {
				require.Nil(t, valUp)
				require.Nil(t, wallet)
				valUp = &lastValUps[i]
				wallet = valWallets[i]
			}
		}
		require.NotNil(t, valUp)
		require.NotNil(t, wallet)
		require.EqualValues(t, crypto.PubKeyBytes2Addr(valUp.PubKey.GetSecp256K1()), vpow.to)
		require.EqualValues(t, wallet.Address(), vpow.to)
		require.EqualValues(t, valUp.Power, vpow.SumPower)
		require.Equal(t, 1, len(vpow.PowerChunks))
		require.Equal(t, valUp.Power, vpow.PowerChunks[0].Power)
		require.EqualValues(t, bytes2.ZeroBytes(32), vpow.PowerChunks[0].TxHash)

		sum := int64(0)
		for _, pc := range vpow.PowerChunks {
			sum += pc.Power
		}
		require.EqualValues(t, sum, vpow.SumPower)

		totalPower2 += vpow.SumPower
		return nil
	}, true)
	require.NoError(t, xerr)

	require.Equal(t, totalPower0, totalPower1)
	require.Equal(t, totalPower0, totalPower2)

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func Test_LoadLedger(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, lastValUps, _, xerr := initLedger(config)
	require.NoError(t, xerr)

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)
	require.Equal(t, int64(1), lastHeight)

	require.NoError(t, ctrler.LoadDelegatees(int(govMock.MaxValidatorCnt())))

	require.Len(t, ctrler.allDelegatees, len(lastValUps))
	require.LessOrEqual(t, len(ctrler.lastValidators), int(govMock.MaxValidatorCnt()))

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
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, lastValUps, valWallets, xerr := initLedger(config)
	require.NoError(t, xerr)
	require.Equal(t, len(lastValUps), len(valWallets))

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)

	fromWals, valWals, powers, txhashes := testRandDelegate(t, 20000, ctrler, valWallets, lastHeight+1)

	_, lastHeight, xerr = ctrler.Commit()
	require.NoError(t, xerr)

	// close and re-open
	require.NoError(t, ctrler.Close())
	ctrler, xerr = NewVPowerCtrler(config, int(govMock.MaxValidatorCnt()), log.NewNopLogger())
	require.NoError(t, xerr)

	//
	// test for all from wallets
	for i, fromWal := range fromWals {
		valWal := valWals[i]
		txhash := txhashes[i]
		vpow, xerr := ctrler.readVPower(fromWal.Address(), valWal.Address(), true)
		require.NoError(t, xerr)

		found := false
		sum := int64(0)
		for _, pc := range vpow.PowerChunks {
			if bytes.Equal(txhash, pc.TxHash) {
				require.False(t, found) // to check duplication
				require.Equal(t, powers[i], pc.Power, vpow)
				found = true
			}
			sum += pc.Power
		}
		require.True(t, found)
		require.Equal(t, sum, vpow.SumPower)
	}

	// 중복 제거
	onceWals := removeDupWallets(valWals)
	onceFroms := removeDupWallets(fromWals)

	//
	// test for all delegatee(validator) wallets
	for _, valWal := range onceWals {
		dgtee, xerr := ctrler.readDelegatee(valWal.Address(), true)
		require.NoError(t, xerr)
		require.EqualValues(t, valWal.Address(), dgtee.addr)

		sumPower := int64(0)
		fromCnt := 0
		for _, fromWal := range onceFroms {
			vpow, xerr := ctrler.readVPower(fromWal.Address(), valWal.Address(), true)
			if xerr != nil && xerr.Contains(xerrors.ErrNotFoundResult) {
				continue // `fromWal` may not delegate to `valWal`.
			}
			require.NoError(t, xerr)

			sum := int64(0)
			for _, pc := range vpow.PowerChunks {
				sum += pc.Power
				//fmt.Printf("from: %x, to: %x, power: %v, txhash:%x\n", fromWal.Address(), dgtee.addr, pc.Power, pc.TxHash)
			}
			require.EqualValues(t, sum, vpow.SumPower)
			sumPower += sum
			fromCnt++
		}
		require.Equal(t, fromCnt+1, len(dgtee.Delegators)) // `fromCnt` dose not include self address.
		require.EqualValues(t, sumPower, dgtee.SumPower-dgtee.SelfPower, func() string {
			ret := ""
			for _, d := range dgtee.Delegators {
				ret += fmt.Sprintf("%x\n", d)
			}
			return fmt.Sprintf("validator:%v\ntotal:%v, self:%v, total-self:%v\ndelegators\n%s", dgtee.addr, dgtee.SumPower, dgtee.SelfPower, dgtee.SumPower-dgtee.SelfPower, ret)
		}())
	}

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func Test_Bonding_ToNotValidator(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, lastValUps, valWallets, xerr := initLedger(config)
	require.NoError(t, xerr)
	require.Equal(t, len(lastValUps), len(valWallets))

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)
	require.Equal(t, int64(1), lastHeight)

	fromWallet := acctMock.RandWallet()
	power := bytes2.RandInt64N(1_000_000) + 4000

	// not validator

	txctx, xerr := makeBondingTrxCtx(fromWallet, types.RandAddress(), power, lastHeight+1)
	require.NoError(t, xerr)
	require.Error(t, xerrors.ErrNotFoundDelegatee, executeTransaction(ctrler, txctx))

	//
	// to not validator (self bonding)
	power = govMock.MinValidatorPower()
	txctx, xerr = makeBondingTrxCtx(fromWallet, nil, power, lastHeight+1)
	require.NoError(t, xerr)
	require.Equal(t, fromWallet.Address(), txctx.Tx.From)
	require.Equal(t, fromWallet.Address(), txctx.Tx.To)
	// txctx.Tx.To is not validator and nothing must be found about txctx.Tx.To.
	_, xerr = ctrler.readDelegatee(txctx.Tx.To, txctx.Exec)
	require.Equal(t, xerrors.ErrNotFoundResult, xerr)
	//fmt.Println("validator(before)", dgtee0.Address(), dgtee0.TotalPower, dgtee0.SelfPower)
	vpow, xerr := ctrler.readVPower(txctx.Tx.From, txctx.Tx.To, true)
	require.Equal(t, xerrors.ErrNotFoundResult, xerr)
	require.Nil(t, vpow)
	//run tx

	require.NoError(t, executeTransaction(ctrler, txctx))

	// check delegatee: the `txctx.Tx.To` should be found in `dgteesLedger`.
	dgtee, xerr := ctrler.readDelegatee(txctx.Tx.To, txctx.Exec)
	require.NoError(t, xerr)
	require.NotNil(t, dgtee)

	require.Equal(t, power, dgtee.SumPower)
	require.Equal(t, power, dgtee.SelfPower)
	//fmt.Println("validator(after)", dgtee1.Address(), dgtee1.TotalPower, dgtee1.SelfPower)
	// check vpow: the vpow of `txctx.Tx.From` should be found.
	vpow, xerr = ctrler.readVPower(txctx.Tx.From, txctx.Tx.To, true)
	require.NoError(t, xerr)
	require.NotNil(t, vpow)
	require.Equal(t, vpow.SumPower, vpow.sumPowerChunk())
	require.EqualValues(t, txctx.TxHash, vpow.PowerChunks[len(vpow.PowerChunks)-1].TxHash)
	require.EqualValues(t, lastHeight+1, vpow.PowerChunks[len(vpow.PowerChunks)-1].Height)
	require.EqualValues(t, power, vpow.PowerChunks[len(vpow.PowerChunks)-1].Power)
	//------------------------------------------------------------------------------------------------------------------

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func Test_Unbonding(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, lastValUps, valWallets, xerr := initLedger(config)
	require.NoError(t, xerr)

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)
	require.Equal(t, int64(1), lastHeight)

	valWal := valWallets[rand.Intn(len(lastValUps))]

	dgtee0, xerr := ctrler.readDelegatee(valWal.Address(), true)
	require.NoError(t, xerr)
	require.NotNil(t, dgtee0)

	totalPower0 := dgtee0.SumPower
	selfPower0 := dgtee0.SelfPower

	// delegate to `valWal` from `fromWal`
	fromWal := acctMock.RandWallet()
	power := int64(5000)
	height0 := lastHeight + 1
	txctx0, xerr := doDelegate(ctrler, fromWal, valWal.Address(), power, height0)
	require.NoError(t, xerr)
	txhash0 := txctx0.TxHash

	_, lastHeight, xerr = ctrler.Commit()
	require.NoError(t, xerr)
	require.Equal(t, height0, lastHeight)

	dgtee1, xerr := ctrler.readDelegatee(valWal.Address(), true)
	require.NoError(t, xerr)
	require.Equal(t, totalPower0+power, dgtee1.SumPower)
	require.Equal(t, selfPower0, dgtee1.SelfPower)

	// -----------------------------------------------------------------------------------------------------------------
	//

	//
	// unbonding
	// 1. wrong from
	txctx1, xerr := doUndelegate(ctrler, acctMock.RandWallet(), valWal.Address(), lastHeight+1, txhash0)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundStake))
	require.Equal(t, 0, ctrler.countOf(v1.KeyPrefixFrozenVPower, true))
	// 2. wrong to
	txctx1, xerr = doUndelegate(ctrler, fromWal, types.RandAddress(), lastHeight+1, txhash0)
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundDelegatee))
	require.Equal(t, 0, ctrler.countOf(v1.KeyPrefixFrozenVPower, true))
	// 3. wrong txhash
	txctx1, xerr = doUndelegate(ctrler, fromWal, valWal.Address(), lastHeight+1, bytes2.RandBytes(32))
	require.Error(t, xerr)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundStake))
	require.Equal(t, 0, ctrler.countOf(v1.KeyPrefixFrozenVPower, true))
	// 4. all ok
	txctx1, xerr = doUndelegate(ctrler, fromWal, valWal.Address(), lastHeight+1, txhash0)
	require.NoError(t, xerr)
	require.Equal(t, 1, ctrler.countOf(v1.KeyPrefixFrozenVPower, true))

	// commit
	_, lastHeight, xerr = ctrler.Commit()
	require.NoError(t, xerr)

	// one frozen vpower has been added by executing txctx1.
	dgtee1, xerr = ctrler.readDelegatee(dgtee0.addr, txctx1.Exec)
	require.NoError(t, xerr, dgtee0.key)
	require.Equal(t, totalPower0, dgtee1.SumPower)
	require.Equal(t, selfPower0, dgtee1.SelfPower)
	require.Equal(t, 1, ctrler.countOf(v1.KeyPrefixFrozenVPower, true))
	refundHeight := lastHeight + govMock.LazyUnstakingBlocks()
	frozen, xerr := ctrler.readFrozenVPower(refundHeight, fromWal.Address(), true)
	require.NoError(t, xerr)
	require.NotNil(t, frozen)
	require.Equal(t, power, frozen.RefundPower)
	require.Equal(t, 1, len(frozen.PowerChunks))
	require.Equal(t, power, frozen.PowerChunks[0].Power)
	require.Equal(t, height0, frozen.PowerChunks[0].Height)
	require.EqualValues(t, txhash0, frozen.PowerChunks[0].TxHash)

	// nothing happens because refundHeight has not been reached.
	xerr = ctrler._unfreezePowerChunk(refundHeight-1, acctMock)
	require.NoError(t, xerr)
	require.Equal(t, 1, ctrler.countOf(v1.KeyPrefixFrozenVPower, true))
	frozen, xerr = ctrler.readFrozenVPower(refundHeight, fromWal.Address(), true)
	require.NoError(t, xerr)
	require.NotNil(t, frozen)
	require.Equal(t, power, frozen.RefundPower)
	require.Equal(t, 1, len(frozen.PowerChunks))
	require.Equal(t, power, frozen.PowerChunks[0].Power)
	require.Equal(t, height0, frozen.PowerChunks[0].Height)
	require.EqualValues(t, txhash0, frozen.PowerChunks[0].TxHash)

	// frozen vpower has been removed because refundHeight has been reached.
	xerr = ctrler._unfreezePowerChunk(refundHeight, acctMock)
	require.NoError(t, xerr)
	require.Equal(t, 0, ctrler.countOf(v1.KeyPrefixFrozenVPower, true))
	frozen, xerr = ctrler.readFrozenVPower(refundHeight, fromWal.Address(), true)
	require.Error(t, xerr)
	require.Nil(t, frozen)

	// -----------------------------------------------------------------------------------------------------------------
	//

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func Test_Unbonding_AllSelfPower(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, _, valWallets, xerr := initLedger(config)
	require.NoError(t, xerr)

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)

	froms, vals, _, _ := testRandDelegate(t, 1000, ctrler, valWallets, lastHeight+1)

	_, lastHeight, xerr = ctrler.Commit()
	require.NoError(t, xerr)

	onceWals := removeDupWallets(vals)
	for _, valWal := range onceWals {
		// unbonding self power deposited at genesis with zero txhash
		zeroHash := bytes2.ZeroBytes(32) // points to self voting power
		_, xerr = doUndelegate(ctrler, valWal, valWal.Address(), lastHeight+1, zeroHash)
		require.NoError(t, xerr)

		dgtee, xerr := ctrler.readDelegatee(valWal.Address(), true)
		require.Equal(t, xerrors.ErrNotFoundResult, xerr)
		require.Nil(t, dgtee)

		// expected that all vpowers delegated to dgtee are removed
		for _, fromWal := range froms {
			vpow, xerr := ctrler.readVPower(fromWal.Address(), valWal.Address(), true)
			require.Equal(t, xerrors.ErrNotFoundResult, xerr)
			require.Nil(t, vpow)
		}
	}

	// -----------------------------------------------------------------------------------------------------------------
	//

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func Test_Freezing(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, _, valWallets, xerr := initLedger(config)
	require.NoError(t, xerr)

	powers0 := make(map[string]int64)
	for _, v := range valWallets {
		item, xerr := ctrler.powersState.Get(v1.LedgerKeyDelegatee(v.Address(), nil), true)
		require.NoError(t, xerr)

		dgtee, _ := item.(*Delegatee)
		require.EqualValues(t, v.Address(), dgtee.addr)

		powers0[dgtee.addr.String()] = dgtee.SumPower
	}

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)
	height := lastHeight + 1

	froms, vals, powers, txhashes := testRandDelegate(t, 1000, ctrler, valWallets, lastHeight+1)

	fmt.Println("freeze ...")

	frozenTxhashes := make(map[string]struct{})
	frozenCntOf := make(map[string]int)
	refundCntAt := make(map[int64]int)
	minRefundHeight := int64(math.MaxInt64)
	maxRefundHeight := int64(0)
	for len(frozenTxhashes) < len(txhashes) {
		var idx int
		var txhash bytes2.HexBytes
		for {
			idx = rand.Int() % len(txhashes)
			txhash = txhashes[idx]
			if _, ok := frozenTxhashes[txhash.String()]; !ok {
				frozenTxhashes[txhash.String()] = struct{}{}
				frozenCntOf[fmt.Sprintf("%v|%v", height, froms[idx].Address())]++
				if frozenCntOf[fmt.Sprintf("%v|%v", height, froms[idx].Address())] == 1 {
					// newly frozen vpower, not power chunk
					refundCntAt[height+govMock.LazyUnstakingBlocks()]++
				}
				minRefundHeight = libs.MinInt64(minRefundHeight, height+govMock.LazyUnstakingBlocks())
				maxRefundHeight = libs.MaxInt64(maxRefundHeight, height+govMock.LazyUnstakingBlocks())
				break
			}
		}
		fromW := froms[idx]
		valW := vals[idx]
		power := powers[idx]

		dgtee0, xerr := ctrler.readDelegatee(valW.Address(), true)
		require.NoError(t, xerr)
		sumPower0 := dgtee0.SumPower
		selfPower0 := dgtee0.SelfPower

		// freeze ...
		_, xerr = doUndelegate(ctrler, fromW, valW.Address(), height, txhash)
		require.NoError(t, xerr)

		//fmt.Println("undelegated", txhash, "from", fromW.Address(), "to", valW.Address())

		dgtee1, xerr := ctrler.readDelegatee(valW.Address(), true)
		require.NoError(t, xerr, dgtee1.key)
		require.Equal(t, sumPower0-power, dgtee1.SumPower)
		require.Equal(t, selfPower0, dgtee1.SelfPower)

		refundHeight := height + govMock.LazyUnstakingBlocks()
		frozen, xerr := ctrler.readFrozenVPower(refundHeight, fromW.Address(), true)
		require.NoError(t, xerr)
		require.NotNil(t, frozen)

		sum := int64(0)
		found := false
		for _, pc := range frozen.PowerChunks {
			if bytes.Equal(txhash, pc.TxHash) {
				require.False(t, found)
				require.Equal(t, power, pc.Power)
				//require.Equal(t, height, pc.Height)
				found = true
			}
			sum += pc.Power
		}
		require.Equal(t, sum, frozen.RefundPower)
		require.Equal(t, frozenCntOf[fmt.Sprintf("%v|%v", height, fromW.Address())], len(frozen.PowerChunks), func() string {
			ret := fmt.Sprintf("from: %v, refundPower: %v\n", fromW.Address(), frozen.RefundPower)
			for i, pc := range frozen.PowerChunks {
				ret += fmt.Sprintf("  [%d] power:%v, txhash:%X\n", i, pc.Power, pc.TxHash)
			}
			return ret
		}())

		//if frozenCntOf[fmt.Sprintf("%v|%v", height, fromW.Address())] >= 2 {
		//	fmt.Println("freeze", frozenCntOf[fmt.Sprintf("%v|%v", height, fromW.Address())], "power chunks of", fromW.Address())
		//}

		if rand.Int()%10 == 0 {
			_, lastHeight, xerr = ctrler.powersState.Commit()
			require.NoError(t, xerr)
			height = lastHeight + 1
			//fmt.Println("Committed", "height", lastHeight)
		}
	}

	_, lastHeight, xerr = ctrler.powersState.Commit()
	require.NoError(t, xerr)
	height = lastHeight + 1

	fmt.Println("freezed all delegated vpowers - last committed height", lastHeight, "minRefundHeight", minRefundHeight, "maxRefundHeight", maxRefundHeight)
	fmt.Println("unfreeze ...")

	for h := height; h <= maxRefundHeight; h++ {
		var expectedRefundAddrs []types.Address
		var expectedRefundPower []int64
		var expectedBalances []*uint256.Int
		// frozen vpowers to be un-frozen (thawed) at height 'h'
		xerr = ctrler.powersState.Seek(
			v1.LedgerKeyFrozenVPower(h, nil),
			true,
			func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
				frozen, _ := item.(*FrozenVPower)
				sum := int64(0)
				for _, pc := range frozen.PowerChunks {
					sum += pc.Power
				}
				require.Equal(t, sum, frozen.RefundPower)
				expectedRefundPower = append(expectedRefundPower, frozen.RefundPower)

				addr := key[9:]
				expectedRefundAddrs = append(expectedRefundAddrs, addr)

				acct := acctMock.FindAccount(addr, true)
				require.NotNil(t, acct)

				expectedBalances = append(expectedBalances,
					new(uint256.Int).Add(acct.Balance, ctrlertypes.PowerToAmount(frozen.RefundPower)))

				return nil
			}, true)
		require.NoError(t, xerr)
		require.Equal(t, refundCntAt[h], len(expectedRefundPower))
		require.Equal(t, len(expectedRefundAddrs), len(expectedRefundPower))
		require.Equal(t, len(expectedBalances), len(expectedRefundPower))

		xerr = ctrler._unfreezePowerChunk(h, acctMock)
		require.NoError(t, xerr)

		for idx, addr := range expectedRefundAddrs {
			acct := acctMock.FindAccount(addr, true)
			require.NotNil(t, acct)
			require.Equal(t, expectedBalances[idx].String(), acct.Balance.String())
		}

		_, lastHeight, xerr = ctrler.powersState.Commit()
		require.NoError(t, xerr)

		for idx, addr := range expectedRefundAddrs {
			acct := acctMock.FindAccount(addr, false)
			require.NotNil(t, acct)
			require.Equal(t, expectedBalances[idx].String(), acct.Balance.String())
		}
	}

	ctrler.powersState.Seek(
		v1.KeyPrefixFrozenVPower,
		true,
		func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
			frozen := item.(*FrozenVPower)
			_h := key[1:9]
			h := binary.BigEndian.Uint64(_h)
			fmt.Println("maxRefundHeight", maxRefundHeight, "h", h, frozen)
			return nil
		},
		true,
	)
	require.Equal(t, 0, ctrler.countOf(v1.KeyPrefixFrozenVPower, true))

	// at now, all delegated power has been frozen.
	// only initial vpowers are remained.
	fmt.Println("return to initial vpowers - last committed height", lastHeight)

	for _, v := range valWallets {
		item, xerr := ctrler.powersState.Get(v1.LedgerKeyDelegatee(v.Address(), nil), true)
		require.NoError(t, xerr)

		dgtee, _ := item.(*Delegatee)
		require.EqualValues(t, v.Address(), dgtee.addr)
		require.Equal(t, powers0[dgtee.addr.String()], dgtee.SumPower)
		require.Equal(t, powers0[dgtee.addr.String()], dgtee.SelfPower)
		require.Equal(t, 1, len(dgtee.Delegators))
	}

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func testRandDelegate(t *testing.T, count int, ctrler *VPowerCtrler, valWallets []*web3.Wallet, height int64) ([]*web3.Wallet, []*web3.Wallet, []int64, []bytes2.HexBytes) {
	var fromWals0 []*web3.Wallet
	var valWals0 []*web3.Wallet
	var powers0 []int64
	var txhashes0 []bytes2.HexBytes
	addedPower0 := int64(0)

	for i := 0; i < count; i++ {
		// `fromWal` delegates to `valWal`

		fromWal := acctMock.RandWallet()
		valWal := valWallets[rand.Intn(len(valWallets))]
		power := bytes2.RandInt64N(1_000) + 4000

		txctx, xerr := doDelegate(ctrler, fromWal, valWal.Address(), power, height)
		require.NoError(t, xerr)

		fromWals0 = append(fromWals0, fromWal)
		valWals0 = append(valWals0, valWal)
		txhashes0 = append(txhashes0, txctx.TxHash)
		powers0 = append(powers0, power)
		addedPower0 += power
	}

	fromAddrsOfDgtee := make(map[string][]types.Address)
	sumPowerOfDgtee := make(map[string]int64)

	// check all vpowers
	var txhashes1 []bytes2.HexBytes
	xerr := ctrler.powersState.Seek(v1.KeyPrefixVPower, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		vpow, _ := item.(*VPower)
		require.EqualValues(t, crypto.PubKeyBytes2Addr(vpow.PubKeyTo), vpow.to)
		require.EqualValues(t, v1.LedgerKeyVPower(vpow.From, vpow.to), key)
		require.EqualValues(t, key, vpow.key)

		sum := int64(0)
		for _, pc := range vpow.PowerChunks {
			sum += pc.Power
			if bytes.Equal(pc.TxHash, bytes2.ZeroBytes(32)) {
				// this power chunk was added by `InitLedger` not this function
				continue
			}

			txhashes1 = append(txhashes1, pc.TxHash)

			for i, txhash := range txhashes0 {
				if bytes.Equal(txhash, pc.TxHash) {
					from := fromWals0[i].Address()
					to := valWals0[i].Address()
					require.EqualValues(t, from, vpow.From)
					require.EqualValues(t, to, vpow.to)
					require.EqualValues(t, to, crypto.PubKeyBytes2Addr(vpow.PubKeyTo))
					require.Equal(t, powers0[i], pc.Power)
				}
			}
		}
		require.Equal(t, sum, vpow.SumPower)

		fromAddrsOfDgtee[vpow.to.String()] = append(fromAddrsOfDgtee[vpow.to.String()], vpow.From)
		sumPowerOfDgtee[vpow.to.String()] += vpow.SumPower
		return nil
	}, true)
	require.NoError(t, xerr)

	// all in `txhashes0`( txs executed in this time) MUST be found only one time in `txhashes1`(txs on ledger).
	for _, txhash0 := range txhashes0 {
		found := false
		for _, txhash1 := range txhashes1 {
			if bytes.Equal(txhash0, txhash1) {
				require.False(t, found) // it must be found only once.
				found = true
			}
		}
		require.True(t, found)
	}

	// check delegatees
	xerr = ctrler.powersState.Seek(v1.KeyPrefixDelegatee, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		dgtee, _ := item.(*Delegatee)
		require.EqualValues(t, crypto.PubKeyBytes2Addr(dgtee.PubKey), dgtee.addr)
		require.EqualValues(t, v1.LedgerKeyDelegatee(dgtee.addr, nil), key)
		require.EqualValues(t, key, dgtee.key)
		require.Equal(t, sumPowerOfDgtee[dgtee.addr.String()], dgtee.SumPower)
		require.Equal(t, len(fromAddrsOfDgtee[dgtee.addr.String()]), len(dgtee.Delegators))
		expectedAddrArray := fromAddrsOfDgtee[dgtee.addr.String()]
		actualAddrArray := dgtee.Delegators
		sort.Slice(expectedAddrArray, func(i, j int) bool { return bytes.Compare(expectedAddrArray[i], expectedAddrArray[j]) < 0 })
		sort.Slice(actualAddrArray, func(i, j int) bool { return bytes.Compare(actualAddrArray[i], actualAddrArray[j]) < 0 })
		for i := 0; i < len(actualAddrArray); i++ {
			require.EqualValues(t, expectedAddrArray[i], actualAddrArray[i])
		}

		return nil
	}, true)
	require.NoError(t, xerr)

	return fromWals0, valWals0, powers0, txhashes0
}

func initLedger(cfg *beatozcfg.Config) (*VPowerCtrler, []abcitypes.ValidatorUpdate, []*web3.Wallet, xerrors.XError) {

	ctrler, xerr := NewVPowerCtrler(cfg, int(govMock.MaxValidatorCnt()), log.NewNopLogger())
	if xerr != nil {
		return nil, nil, nil, xerr
	}

	// make random validator set.
	var valWallets []*web3.Wallet
	var vals []abcitypes.ValidatorUpdate
	for i := 0; i < 21; i++ {
		w := web3.NewWallet(nil)
		pub := w.GetPubKey()

		pke := secp256k1.PubKey(pub)
		pkp, err := cryptoenc.PubKeyToProto(pke)
		if err != nil {
			return nil, nil, nil, xerrors.From(err)
		}

		vals = append(vals, abcitypes.ValidatorUpdate{
			// Address:
			PubKey: pkp,
			Power:  10_000_000,
		})

		valWallets = append(valWallets, w)
	}
	xerr = ctrler.InitLedger(vals)
	if xerr != nil {
		return nil, nil, nil, xerr
	}
	return ctrler, vals, valWallets, nil
}

func makeBondingTrxCtx(fromAcct *web3.Wallet, to types.Address, power int64, height int64) (*ctrlertypes.TrxContext, xerrors.XError) {
	if to == nil {
		to = fromAcct.Address()
	}
	tx := web3.NewTrxStaking(
		fromAcct.Address(),
		to,
		fromAcct.GetNonce(),
		govMock.MinTrxGas(), govMock.GasPrice(),
		types.ToFons(uint64(power)),
	)
	if _, _, err := fromAcct.SignTrxRLP(tx, config.ChainID); err != nil {
		return nil, xerrors.From(err)
	}

	txCtx, xerr := mocks.MakeTrxCtxWithTrx(tx, config.ChainID, height, time.Now(), true, govMock, acctMock, nil, nil, nil)
	if xerr != nil {
		return nil, xerr
	}
	return txCtx, nil
}

func makeUnbondingTrxCtx(fromAcct *web3.Wallet, to types.Address, height int64, txhash bytes2.HexBytes) (*ctrlertypes.TrxContext, xerrors.XError) {
	if to == nil {
		to = fromAcct.Address()
	}
	tx := web3.NewTrxUnstaking(
		fromAcct.Address(),
		to,
		fromAcct.GetNonce(),
		govMock.MinTrxGas(), govMock.GasPrice(),
		txhash,
	)
	if _, _, err := fromAcct.SignTrxRLP(tx, config.ChainID); err != nil {
		return nil, xerrors.From(err)
	}

	txCtx, xerr := mocks.MakeTrxCtxWithTrx(tx, config.ChainID, height, time.Now(), true, govMock, acctMock, nil, nil, nil)
	if xerr != nil {
		return nil, xerr
	}
	return txCtx, nil
}

func doDelegate(ctrler *VPowerCtrler, fromWal *web3.Wallet, toAddr types.Address, power, height int64) (*ctrlertypes.TrxContext, xerrors.XError) {
	txctx, xerr := makeBondingTrxCtx(fromWal, toAddr, power, height)
	if xerr != nil {
		return nil, xerr
	}
	if xerr = executeTransaction(ctrler, txctx); xerr != nil {
		return nil, xerr
	}
	return txctx, nil
}

func doUndelegate(ctrler *VPowerCtrler, fromWal *web3.Wallet, toAddr types.Address, height int64, txhash bytes2.HexBytes) (*ctrlertypes.TrxContext, xerrors.XError) {
	txctx, xerr := makeUnbondingTrxCtx(fromWal, toAddr, height, txhash)
	if xerr != nil {
		return nil, xerr
	}
	if xerr = executeTransaction(ctrler, txctx); xerr != nil {
		return nil, xerr
	}
	return txctx, nil
}

func executeTransaction(ctrler *VPowerCtrler, txctx *ctrlertypes.TrxContext) xerrors.XError {
	if xerr := ctrler.ValidateTrx(txctx); xerr != nil {
		return xerr
	}
	if xerr := ctrler.ExecuteTrx(txctx); xerr != nil {
		return xerr
	}
	return nil
}

func checkExistDelegatee(dgt *Delegatee, vups []abcitypes.ValidatorUpdate) bool {
	for _, vup := range vups {
		if euqalDelegatee(dgt, vup) {
			return true
		}
	}
	return false
}
func euqalDelegatee(dgt *Delegatee, vup abcitypes.ValidatorUpdate) bool {
	return bytes.Equal(dgt.PubKey, vup.PubKey.GetSecp256K1()) && dgt.SumPower == vup.Power
}

func removeDupWallets(walllets []*web3.Wallet) []*web3.Wallet {
	_map := make(map[string]*web3.Wallet)
	for _, v := range walllets {
		_map[v.Address().String()] = v
	}
	var result []*web3.Wallet
	for _, v := range _map {
		result = append(result, v)
	}
	return result
}
