package vpower

import (
	"bytes"
	"fmt"
	beatozcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
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
	rootDir := filepath.Join(os.TempDir(), "test-vpowctrler")
	config = beatozcfg.DefaultConfig()
	config.SetRoot(rootDir)
	acctMock = mocks.NewAccountHandlerMock(1000)
	acctMock.Iterate(func(idx int, w *web3.Wallet) bool {
		w.GetAccount().SetBalance(types.ToFons(1_000_000_000))
		return true
	})

	govParams = ctrlertypes.DefaultGovParams()
}

func Test_InitLedger(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, lastValUps, valWallets, xerr := initCtrler(config)
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
		dgt, _ := item.(*DelegateeV1)
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
		require.EqualValues(t, valUp.PubKey.GetSecp256K1(), vpow.PubKeyTo)
		require.EqualValues(t, crypto.PubKeyBytes2Addr(vpow.PubKeyTo), vpow.to)
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

	ctrler, lastValUps, _, xerr := initCtrler(config)
	require.NoError(t, xerr)

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)
	require.Equal(t, int64(1), lastHeight)

	require.NoError(t, ctrler.LoadLedger(int(govParams.MaxValidatorCnt())))

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
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, lastValUps, valWallets, xerr := initCtrler(config)
	require.NoError(t, xerr)
	require.Equal(t, len(lastValUps), len(valWallets))

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)

	fromWals, valWals, powers, txhashes := testRandDelegating(t, 20000, ctrler, valWallets, lastHeight)

	_, lastHeight, xerr = ctrler.Commit()
	require.NoError(t, xerr)

	// close and re-open
	require.NoError(t, ctrler.Close())
	ctrler, xerr = NewVPowerCtrler(config, 0, log.NewNopLogger())
	require.NoError(t, xerr)

	for i, fromWal := range fromWals {
		valWal := valWals[i]
		txhash := txhashes[i]
		vpow, xerr := ctrler.readVPower(fromWal.Address(), valWal.Address(), true)
		require.NoError(t, xerr)

		sum := int64(0)
		found := false
		for _, pc := range vpow.PowerChunks {
			if bytes.Equal(txhash, pc.TxHash) {
				require.Equal(t, powers[i], pc.Power, vpow)
				found = true
			}
			sum += pc.Power
		}
		require.Equal(t, sum, vpow.SumPower)
		require.True(t, found)
	}

	fromCountOfDgtee := make(map[string]int)
	sumPowerOfDgtee := make(map[string]int64)
	xerr = ctrler.powersState.Seek(v1.KeyPrefixVPower, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		vpow, _ := item.(*VPower)
		require.EqualValues(t, crypto.PubKeyBytes2Addr(vpow.PubKeyTo), vpow.to)
		require.EqualValues(t, v1.LedgerKeyVPower(vpow.From, vpow.to), key)
		require.EqualValues(t, key, vpow.key)

		sum := int64(0)
		for _, pc := range vpow.PowerChunks {
			sum += pc.Power
			if bytes.Equal(pc.TxHash, bytes2.ZeroBytes(32)) {
				continue
			}

			found := false
			for i, txhash := range txhashes {
				if bytes.Equal(txhash, pc.TxHash) {
					require.False(t, found) // it must be found only once.
					from := fromWals[i].Address()
					to := valWals[i].Address()
					require.EqualValues(t, from, vpow.From)
					require.EqualValues(t, to, vpow.to)
					require.EqualValues(t, to, crypto.PubKeyBytes2Addr(vpow.PubKeyTo))
					require.Equal(t, powers[i], pc.Power)
					found = true
				}
			}
			require.True(t, found, "power chunk txhash", pc.TxHash)
		}
		require.Equal(t, sum, vpow.SumPower)

		fromCountOfDgtee[vpow.to.String()]++
		sumPowerOfDgtee[vpow.to.String()] += vpow.SumPower
		return nil
	}, true)
	require.NoError(t, xerr)

	xerr = ctrler.powersState.Seek(v1.KeyPrefixDelegatee, true, func(key v1.LedgerKey, item v1.ILedgerItem) xerrors.XError {
		dgtee, _ := item.(*DelegateeV1)
		require.EqualValues(t, crypto.PubKeyBytes2Addr(dgtee.PubKey), dgtee.addr)
		require.EqualValues(t, v1.LedgerKeyDelegatee(dgtee.addr, nil), key)
		require.EqualValues(t, v1.LedgerKeyDelegatee(dgtee.addr, nil), dgtee.key)
		require.EqualValues(t, key, dgtee.key)
		require.Equal(t, fromCountOfDgtee[dgtee.addr.String()], len(dgtee.Delegators))
		require.Equal(t, sumPowerOfDgtee[dgtee.addr.String()], dgtee.SumPower)
		return nil
	}, true)
	require.NoError(t, xerr)

	// 중복 제거
	onceWals := removeDupWallets(valWals)
	onceFroms := removeDupWallets(fromWals)

	for _, valWal := range onceWals {
		dgtee, xerr := ctrler.readDelegatee(valWal.Address(), true)
		require.NoError(t, xerr)
		require.NotNil(t, dgtee)
		require.EqualValues(t, valWal.Address(), dgtee.addr)

		sumPower := int64(0)
		for _, fromWal := range onceFroms {
			vpow, xerr := ctrler.readVPower(fromWal.Address(), valWal.Address(), true)
			if xerr != nil && xerr.Contains(xerrors.ErrNotFoundResult) {
				continue // `fromWal` may not delegate to `valWal`. So, it may be not found.
			}
			require.NoError(t, xerr)

			sum := int64(0)
			for _, pc := range vpow.PowerChunks {
				sum += pc.Power
				//fmt.Printf("from: %x, to: %x, power: %v, txhash:%x\n", fromWal.Address(), dgtee.addr, pc.Power, pc.TxHash)
			}
			require.EqualValues(t, sum, vpow.SumPower)
			sumPower += sum

		}
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

	ctrler, lastValUps, valWallets, xerr := initCtrler(config)
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
	power = govParams.MinValidatorPower()
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
	// check vpow: the vpow of `txctx.Tx.From` should be found to `vpowsLedger`.
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

	ctrler, lastValUps, valWallets, xerr := initCtrler(config)
	require.NoError(t, xerr)

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)
	require.Equal(t, int64(1), lastHeight)

	//
	// delegate to a validator
	fromWallet := acctMock.RandWallet()
	valWallet := valWallets[rand.Intn(len(lastValUps))]
	valAddr := valWallet.Address()
	power := int64(5000)

	txctx0, xerr := makeBondingTrxCtx(fromWallet, valAddr, power, lastHeight+1)
	require.NoError(t, xerr)
	require.Equal(t, valAddr, txctx0.Tx.To)

	dgtee0, xerr := ctrler.readDelegatee(valAddr, txctx0.Exec)
	require.NoError(t, xerr)
	require.NotNil(t, dgtee0)

	totalPower0 := dgtee0.SumPower
	selfPower0 := dgtee0.SelfPower

	// run tx
	require.NoError(t, executeTransaction(ctrler, txctx0))

	_, lastHeight, xerr = ctrler.Commit()
	require.NoError(t, xerr)

	dgtee1, xerr := ctrler.readDelegatee(valAddr, txctx0.Exec)
	require.NoError(t, xerr)
	require.Equal(t, totalPower0+power, dgtee1.SumPower)
	require.Equal(t, selfPower0, dgtee1.SelfPower)

	// -----------------------------------------------------------------------------------------------------------------
	//

	//
	// unbonding
	// 1. wrong from
	txctx1, xerr := makeUnbondingTrxCtx(acctMock.RandWallet(), valAddr, lastHeight+1, txctx0.TxHash)
	require.NoError(t, xerr)
	xerr = ctrler.ValidateTrx(txctx1)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundStake))
	// 2. wrong to
	txctx1, xerr = makeUnbondingTrxCtx(fromWallet, types.RandAddress(), lastHeight+1, txctx0.TxHash)
	require.NoError(t, xerr)
	xerr = ctrler.ValidateTrx(txctx1)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundDelegatee))
	// 3. wrong txhash
	txctx1, xerr = makeUnbondingTrxCtx(fromWallet, valAddr, lastHeight+1, bytes2.RandBytes(32))
	require.NoError(t, xerr)
	xerr = ctrler.ValidateTrx(txctx1)
	require.True(t, xerr.Contains(xerrors.ErrNotFoundStake))
	//
	// 4. all ok
	txctx1, xerr = makeUnbondingTrxCtx(fromWallet, valAddr, lastHeight+1, txctx0.TxHash)
	require.NoError(t, xerr)
	require.NoError(t, executeTransaction(ctrler, txctx1))
	// commit
	_, lastHeight, xerr = ctrler.Commit()
	require.NoError(t, xerr)

	dgtee1, xerr = ctrler.readDelegatee(dgtee0.addr, txctx1.Exec)
	require.NoError(t, xerr, dgtee0.key)
	require.Equal(t, totalPower0, dgtee1.SumPower)
	require.Equal(t, selfPower0, dgtee1.SelfPower)
	// -----------------------------------------------------------------------------------------------------------------
	//

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.DBDir()))
}

func Test_Unbonding_AllSelfPower(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	ctrler, _, valWallets, xerr := initCtrler(config)
	require.NoError(t, xerr)

	_, lastHeight, xerr := ctrler.Commit()
	require.NoError(t, xerr)

	froms, vals, _, _ := testRandDelegating(t, 1000, ctrler, valWallets, lastHeight)

	_, lastHeight, xerr = ctrler.Commit()
	require.NoError(t, xerr)

	onceWals := removeDupWallets(vals)
	for _, valWal := range onceWals {
		// unbonding self power deposited at genesis with zero txhash
		txctx, xerr := makeUnbondingTrxCtx(valWal, valWal.Address(), lastHeight+1, bytes2.ZeroBytes(32))
		require.NoError(t, xerr)

		require.NoError(t, executeTransaction(ctrler, txctx))

		dgtee, xerr := ctrler.readDelegatee(valWal.Address(), true)
		require.Equal(t, xerrors.ErrNotFoundResult, xerr)
		require.Nil(t, dgtee)
	}

	for _, valWal := range onceWals {
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

func testRandDelegating(t *testing.T, count int, ctrler *VPowerCtrler, valWallets []*web3.Wallet, lastHeight int64) ([]*web3.Wallet, []*web3.Wallet, []int64, []bytes2.HexBytes) {

	var fromWals0 []*web3.Wallet
	var valWals0 []*web3.Wallet
	var powers0 []int64
	var txhashes []bytes2.HexBytes
	addedPower0 := int64(0)

	for i := 0; i < count; i++ {
		// `fromWallet` delegates to `valWallet`

		fromWallet := acctMock.RandWallet()
		valWallet := valWallets[rand.Intn(len(valWallets))]
		power := bytes2.RandInt64N(1_000) + 4000

		txctx0, xerr := makeBondingTrxCtx(fromWallet, valWallet.Address(), power, lastHeight+1)
		require.NoError(t, xerr)
		require.EqualValues(t, valWallet.Address(), txctx0.Tx.To)

		require.NoError(t, executeTransaction(ctrler, txctx0))

		fromWals0 = append(fromWals0, fromWallet)
		valWals0 = append(valWals0, valWallet)
		txhashes = append(txhashes, txctx0.TxHash)
		powers0 = append(powers0, power)
		addedPower0 += power
	}

	for i, fromWal := range fromWals0 {
		valWal := valWals0[i]
		txhash := txhashes[i]
		vpow, xerr := ctrler.readVPower(fromWal.Address(), valWal.Address(), true)
		require.NoError(t, xerr)

		found := false
		for _, pc := range vpow.PowerChunks {
			if bytes.Equal(txhash, pc.TxHash) {
				require.False(t, found) // to check duplication
				require.Equal(t, powers0[i], pc.Power, vpow)
				found = true
			}
		}
		require.True(t, found)
	}

	// 중복 제거
	onceWals := removeDupWallets(valWals0)
	onceFroms := removeDupWallets(fromWals0)

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

	return fromWals0, valWals0, powers0, txhashes
}

func initCtrler(cfg *beatozcfg.Config) (*VPowerCtrler, []abcitypes.ValidatorUpdate, []*web3.Wallet, xerrors.XError) {

	ctrler, xerr := NewVPowerCtrler(cfg, 0, log.NewNopLogger())
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

func checkExistDelegatee(dgt *DelegateeV1, vups []abcitypes.ValidatorUpdate) bool {
	for _, vup := range vups {
		if euqalDelegatee(dgt, vup) {
			return true
		}
	}
	return false
}
func euqalDelegatee(dgt *DelegateeV1, vup abcitypes.ValidatorUpdate) bool {
	return bytes.Equal(dgt.PubKey, vup.PubKey.GetSecp256K1()) && dgt.SumPower == vup.Power
}

func makeBondingTrxCtx(fromAcct *web3.Wallet, to types.Address, power int64, height int64) (*ctrlertypes.TrxContext, xerrors.XError) {
	if to == nil {
		to = fromAcct.Address()
	}
	tx := web3.NewTrxStaking(
		fromAcct.Address(),
		to,
		fromAcct.GetNonce(),
		govParams.MinTrxGas(), govParams.GasPrice(),
		types.ToFons(uint64(power)),
	)
	if _, _, err := fromAcct.SignTrxRLP(tx, config.ChainID); err != nil {
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

func makeUnbondingTrxCtx(fromAcct *web3.Wallet, to types.Address, height int64, txhash bytes2.HexBytes) (*ctrlertypes.TrxContext, xerrors.XError) {
	if to == nil {
		to = fromAcct.Address()
	}
	tx := web3.NewTrxUnstaking(
		fromAcct.Address(),
		to,
		fromAcct.GetNonce(),
		govParams.MinTrxGas(), govParams.GasPrice(),
		txhash,
	)
	if _, _, err := fromAcct.SignTrxRLP(tx, config.ChainID); err != nil {
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

func executeTransaction(ctrler *VPowerCtrler, txctx *ctrlertypes.TrxContext) xerrors.XError {
	if xerr := ctrler.ValidateTrx(txctx); xerr != nil {
		return xerr
	}
	if xerr := ctrler.ExecuteTrx(txctx); xerr != nil {
		return xerr
	}
	return nil
}
