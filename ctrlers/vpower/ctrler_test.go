package vpower

import (
	"bytes"
	"fmt"
	beatozcfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
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
	xerr = ctrler.dgteesLedger.Iterate(func(dgt *DelegateeV1) xerrors.XError {
		var valUp *abcitypes.ValidatorUpdate
		var wallet *web3.Wallet
		for i, w := range valWallets {
			if bytes.Equal(w.Address(), dgt.addr) {
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
		require.EqualValues(t, valUp.Power, dgt.TotalPower)
		require.EqualValues(t, valUp.Power, dgt.SelfPower)

		totalPower1 += dgt.TotalPower
		return nil
	}, true)

	totalPower2 := int64(0)
	xerr = ctrler.vpowsLedger.Iterate(func(vpow *VPower) xerrors.XError {
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
		vpow, xerr := ctrler.vpowsLedger.Get(vpowerProtoKey(fromWal.Address(), valWal.Address()), true)
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

	xerr = ctrler.vpowsLedger.Iterate(func(vpow *VPower) xerrors.XError {
		sum := int64(0)
		for _, pc := range vpow.PowerChunks {
			sum += pc.Power
			if bytes.Equal(pc.TxHash, bytes2.ZeroBytes(32)) {
				continue
			}

			found := false
			for i, txhash := range txhashes {
				if bytes.Equal(txhash, pc.TxHash) {
					from := fromWals[i].Address()
					to := valWals[i].Address()
					require.EqualValues(t, from, vpow.From)
					require.EqualValues(t, to, vpow.to)
					require.EqualValues(t, to, crypto.PubKeyBytes2Addr(vpow.PubKeyTo))

					require.Equal(t, powers[i], pc.Power)
					found = true
					break
				}
			}
			require.True(t, found, "power chunk txhash", pc.TxHash)
		}
		require.Equal(t, sum, vpow.SumPower)
		return nil
	}, true)
	require.NoError(t, xerr)

	// 중복 제거
	onceWals := removeDupWallets(valWals)
	onceFroms := removeDupWallets(fromWals)

	for _, valWal := range onceWals {
		dgtee, xerr := ctrler.dgteesLedger.Get(dgteeProtoKey(valWal.Address()), true)
		require.NoError(t, xerr)
		require.EqualValues(t, valWal.Address(), dgtee.addr)

		sumPower := int64(0)
		for _, fromWal := range onceFroms {
			vpow, xerr := ctrler.vpowsLedger.Get(vpowerProtoKey(fromWal.Address(), valWal.Address()), true)
			if xerr != nil && xerr.Contains(xerrors.ErrNotFoundResult) {
				continue
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
		require.EqualValues(t, sumPower, dgtee.TotalPower-dgtee.SelfPower, func() string {
			ret := ""
			for _, d := range dgtee.Delegators {
				ret += fmt.Sprintf("%x\n", d)
			}
			return fmt.Sprintf("validator:%v\ntotal:%v, self:%v, total-self:%v\ndelegators\n%s", dgtee.addr, dgtee.TotalPower, dgtee.SelfPower, dgtee.TotalPower-dgtee.SelfPower, ret)
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
	xerr = ctrler.ValidateTrx(txctx)
	require.Error(t, xerrors.ErrNotFoundDelegatee, xerr)

	//
	// to not validator (self bonding)
	power = govParams.MinValidatorPower()
	txctx, xerr = makeBondingTrxCtx(fromWallet, nil, power, lastHeight+1)
	require.NoError(t, xerr)
	require.Equal(t, fromWallet.Address(), txctx.Tx.From)
	require.Equal(t, fromWallet.Address(), txctx.Tx.To)
	// txctx.Tx.To is not validator and nothing must be found about txctx.Tx.To.
	_, xerr = ctrler.dgteesLedger.Get(dgteeProtoKey(txctx.Tx.To), txctx.Exec)
	require.Equal(t, xerrors.ErrNotFoundResult, xerr)
	//fmt.Println("validator(before)", dgtee0.Address(), dgtee0.TotalPower, dgtee0.SelfPower)
	vpow, xerr := ctrler.vpowsLedger.Get(vpowerProtoKey(txctx.Tx.From, txctx.Tx.To), true)
	require.Nil(t, vpow)
	require.Equal(t, xerrors.ErrNotFoundResult, xerr)
	//run tx
	xerr = ctrler.ValidateTrx(txctx)
	require.NoError(t, xerr)
	xerr = ctrler.ExecuteTrx(txctx)
	require.NoError(t, xerr)
	// check delegatee: the `txctx.Tx.To` should be found in `dgteesLedger`.
	dgtee1, xerr := ctrler.dgteesLedger.Get(dgteeProtoKey(txctx.Tx.To), txctx.Exec)
	require.NoError(t, xerr)
	require.NotNil(t, dgtee1)
	require.Equal(t, power, dgtee1.TotalPower)
	require.Equal(t, power, dgtee1.SelfPower)
	//fmt.Println("validator(after)", dgtee1.Address(), dgtee1.TotalPower, dgtee1.SelfPower)
	// check vpow: the vpow of `txctx.Tx.From` should be found to `vpowsLedger`.
	vpow, xerr = ctrler.vpowsLedger.Get(vpowerProtoKey(txctx.Tx.From, txctx.Tx.To), true)
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

	dgtee0, xerr := ctrler.dgteesLedger.Get(dgteeProtoKey(valAddr), txctx0.Exec)
	require.NoError(t, xerr)
	totalPower0 := dgtee0.TotalPower
	selfPower0 := dgtee0.SelfPower

	// run tx
	xerr = ctrler.ValidateTrx(txctx0)
	require.NoError(t, xerr)
	xerr = ctrler.ExecuteTrx(txctx0)
	require.NoError(t, xerr)

	_, lastHeight, xerr = ctrler.Commit()
	require.NoError(t, xerr)

	dgtee1, xerr := ctrler.dgteesLedger.Get(dgteeProtoKey(valAddr), txctx0.Exec)
	require.NoError(t, xerr)
	require.Equal(t, totalPower0+power, dgtee1.TotalPower)
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
	xerr = ctrler.ValidateTrx(txctx1)
	require.NoError(t, xerr)
	xerr = ctrler.ExecuteTrx(txctx1)
	require.NoError(t, xerr)
	// commit
	_, lastHeight, xerr = ctrler.Commit()
	require.NoError(t, xerr)

	dgtee1, xerr = ctrler.dgteesLedger.Get(dgtee0.Key(), txctx1.Exec)
	require.NoError(t, xerr)
	require.Equal(t, totalPower0, dgtee1.TotalPower)
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

		require.NoError(t, ctrler.ValidateTrx(txctx), valWal.Address())
		require.NoError(t, ctrler.ExecuteTrx(txctx))

		dgtee, xerr := ctrler.dgteesLedger.Get(dgteeProtoKey(valWal.Address()), true)
		require.Equal(t, xerrors.ErrNotFoundResult, xerr)
		require.Nil(t, dgtee)
	}

	for _, valWal := range onceWals {
		for _, fromWal := range froms {
			vpow, xerr := ctrler.vpowsLedger.Get(vpowerProtoKey(fromWal.Address(), valWal.Address()), true)
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
		// `fromWallet` delegates to `valWals0`

		fromWallet := acctMock.RandWallet()
		fromWals0 = append(fromWals0, fromWallet)

		valWallet := valWallets[rand.Intn(len(valWallets))]
		valAddr := valWallet.Address()
		valWals0 = append(valWals0, valWallet)

		power := bytes2.RandInt64N(1_000) + 4000
		powers0 = append(powers0, power)
		addedPower0 += power

		txctx0, xerr := makeBondingTrxCtx(fromWallet, valAddr, power, lastHeight+1)
		txhashes = append(txhashes, txctx0.TxHash)
		require.NoError(t, xerr)
		require.Equal(t, valAddr, txctx0.Tx.To)

		// run tx
		xerr = ctrler.ValidateTrx(txctx0)
		require.NoError(t, xerr)
		xerr = ctrler.ExecuteTrx(txctx0)
		require.NoError(t, xerr)
	}

	for i, fromWal := range fromWals0 {
		valWal := valWals0[i]
		txhash := txhashes[i]
		vpow, xerr := ctrler.vpowsLedger.Get(vpowerProtoKey(fromWal.Address(), valWal.Address()), true)
		require.NoError(t, xerr)

		found := false
		for _, pc := range vpow.PowerChunks {
			if bytes.Equal(txhash, pc.TxHash) {
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
		dgtee, xerr := ctrler.dgteesLedger.Get(dgteeProtoKey(valWal.Address()), true)
		require.NoError(t, xerr)
		require.EqualValues(t, valWal.Address(), dgtee.addr)

		sumPower := int64(0)
		for _, fromWal := range onceFroms {
			vpow, xerr := ctrler.vpowsLedger.Get(vpowerProtoKey(fromWal.Address(), valWal.Address()), true)
			if xerr != nil && xerr.Contains(xerrors.ErrNotFoundResult) {
				continue
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
		require.EqualValues(t, sumPower, dgtee.TotalPower-dgtee.SelfPower, func() string {
			ret := ""
			for _, d := range dgtee.Delegators {
				ret += fmt.Sprintf("%x\n", d)
			}
			return fmt.Sprintf("validator:%v\ntotal:%v, self:%v, total-self:%v\ndelegators\n%s", dgtee.addr, dgtee.TotalPower, dgtee.SelfPower, dgtee.TotalPower-dgtee.SelfPower, ret)
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
	return bytes.Equal(dgt.PubKey, vup.PubKey.GetSecp256K1()) && dgt.TotalPower == vup.Power
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
