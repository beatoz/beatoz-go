package vpower

import (
	"fmt"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/rand"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	powerRipeningCycle = oneYearSeconds
)

// Test_Validator_AddDel_TotalPower checks the amount of xxx powers after executing AddPower and DelPower
// with static `from` and `validator`
func Test_Validator_DelPower(t *testing.T) {
	_, pubTo := crypto.NewKeypairBytes()
	val := NewDelegatee(pubTo)
	from := rand.Bytes(20)

	var pows []int64
	expectedTotalPower := int64(0)

	for i := 0; i < 10000; i++ {
		pow := rand.Int63n(700_000_000)
		val.AddPowerWithTxHash(from, pow, int64(i+1), rand.Bytes(32))
		pows = append(pows, pow)
		expectedTotalPower += pow
	}

	sumReducedPower := int64(0)
	originTotalPower := val.TotalPower()
	require.Equal(t, expectedTotalPower, originTotalPower)

	for {
		totalPower0 := val.TotalPower()
		require.Equal(t, expectedTotalPower-sumReducedPower, totalPower0)

		// the request value may be greater than `totalPower0`.
		reqReduce := rand.Int63n(totalPower0) + 1
		popedPows, _ := val.DelPower(from, reqReduce)
		reducedPower := popedPows.SumPower
		sumReducedPower += reducedPower

		//fmt.Println("total", totalPower0, "req", reqReduce, "reduced", reducedPower, "sum_reduced", sumReducedPower)

		require.Equal(t, totalPower0-reducedPower, val.TotalPower(), "subPow", reducedPower)
		require.GreaterOrEqual(t, val.TotalPower(), int64(0))
		if val.TotalPower() == 0 {
			break
		}
	}
	require.Equal(t, int64(0), val.TotalPower())
	require.Equal(t, originTotalPower, sumReducedPower)
}

// Test_Validator_Pop_XPowers checks the amount of xxx powers after executing `PopQuantity`
// with static `from` and `validator`.
// Specially it test that the VPower is partially popped.
func Test_Validator_DelPowerWithTxHash(t *testing.T) {
	_, pubTo := crypto.NewKeypairBytes()
	val := NewDelegatee(pubTo)
	from := rand.Bytes(20)

	var pows []int64
	var txHashes [][]byte
	expectedTotalPower := int64(0)

	for i := 0; i < 10000; i++ {
		pow := rand.Int63n(700_000_000)
		txhash := rand.Bytes(32)
		val.AddPowerWithTxHash(from, pow, int64(i+1), txhash)
		pows = append(pows, pow)
		txHashes = append(txHashes, txhash)

		expectedTotalPower += pow
	}

	require.Equal(t, expectedTotalPower, val.TotalPower())

	for len(txHashes) > 0 {
		rn := rand.Intn(len(txHashes))
		removed, _ := val.DelPowerWithTxHash(from, txHashes[rn])
		removedPower := removed.SumPower

		require.Equal(t, pows[rn], removedPower)
		require.Equal(t, expectedTotalPower-removedPower, val.TotalPower())

		pows = append(pows[:rn], pows[rn+1:]...)
		txHashes = append(txHashes[:rn], txHashes[rn+1:]...)
		expectedTotalPower -= removedPower
	}

	require.Equal(t, int64(0), val.TotalPower())
}

func Test_Validator_SelfPower(t *testing.T) {
	_, pubTo := crypto.NewKeypairBytes()
	val := NewDelegatee(pubTo)

	expectedSelf, expectedTotal := int64(0), int64(0)
	for i := 0; i < 10000; i++ {
		pow := bytes.RandInt64N(700_000_000)
		expectedTotal += pow

		from := types.RandAddress()
		if rand.Int()%2 == 0 {
			from, _ = crypto.PubBytes2Addr(pubTo)
			expectedSelf += pow
		}

		val.AddPowerWithTxHash(
			from,
			pow,
			bytes.RandInt64N(100_000_000), // random height
			bytes.RandBytes(32),           // random txhash
		)
	}

	require.Equal(t, expectedSelf, val.SelfPower())
	require.Equal(t, expectedTotal, val.TotalPower())
	require.Equal(t, val.MaturePower(), val.sumMaturePower)
	require.Equal(t, val.MaturePower()+val.RisingPower(), val.TotalPower())
}

// Test_Validator_MaturePower checks the MaturePower after executing Compute
// with static `from` and `validator`
func Test_Validator_MaturePower(t *testing.T) {
	_, pubTo := crypto.NewKeypairBytes()
	val := NewDelegatee(pubTo)
	from := rand.Bytes(20)

	var pows []int64
	lastHeight := int64(0)
	expectedTotalPower := int64(0)

	for i := 0; i < 10000; i++ {
		pow := rand.Int63n(700_000_000)
		val.AddPowerWithTxHash(from, pow, int64(i+1), rand.Bytes(32))
		pows = append(pows, pow)
		expectedTotalPower += pow

		lastHeight = int64(i + 1)
	}

	require.Equal(t, int64(0), val.MaturePower())
	require.Equal(t, expectedTotalPower, val.RisingPower())
	require.Equal(t, expectedTotalPower, val.TotalPower())

	inflationBlocks := lastHeight / 10
	for i := int64(0); i <= lastHeight+inflationBlocks; i += inflationBlocks {
		at := powerRipeningCycle + i

		start := time.Now()
		_ = val.Compute(at, powerRipeningCycle, types.ToFons(math.MaxUint64), 200)
		dur := time.Since(start)

		maturePower := int64(0)
		oh := at - powerRipeningCycle
		for j := 0; j < min(int(oh), len(pows)); j++ {
			maturePower += pows[j]
		}

		require.Equal(t, maturePower, val.MaturePower())
		require.Equal(t, expectedTotalPower-maturePower, val.RisingPower())
		require.Equal(t, expectedTotalPower, val.TotalPower())

		fmt.Println("idx", i, "mature", val.MaturePower(), "total", val.TotalPower(), "compute_time", dur)
	}

	require.Equal(t, expectedTotalPower, val.MaturePower())
	require.Equal(t, int64(0), val.RisingPower())
	require.Equal(t, expectedTotalPower, val.TotalPower())
}

// Test_Validator_Load is a test that uses random heights and ripening blocks.
func Test_Validator_Load(t *testing.T) {
	maxDgtCnt := 42
	vpowCnt := 10000
	dgteeProtos, vpowProtos, ledgerDgtees, ledgerVPows, lastHeight, xerr := initVPowerLedger(maxDgtCnt, vpowCnt)
	require.NoError(t, xerr)
	require.Equal(t, maxDgtCnt, len(dgteeProtos))
	require.Equal(t, maxDgtCnt*vpowCnt, len(vpowProtos))

	for i := 0; i < 1; i++ {
		r := twoWeeksSeconds + rand.Int63n(oneYearSeconds)
		h := lastHeight + rand.Int63n(r/2)

		dgtProtos, xerr := LoadAllDelegateeProtos(ledgerDgtees)
		require.NoError(t, xerr)

		dgtees, xerr := LoadAllVPowerProtos(ledgerVPows, dgtProtos, h, r)
		require.NoError(t, xerr)
		require.Equal(t, len(dgteeProtos), len(dgtees))

		total0, mature0, rising0 := int64(0), int64(0), int64(0)
		for _, dgt := range dgtees {

			found := false
			for _, dgtProto := range dgteeProtos {
				if bytes.Equal(dgtProto.PubKey, dgt.pubKey) {
					require.Equal(t, dgtProto.TotalPower, dgt.totalPower)
					found = true
					break
				}
			}
			require.True(t, found)

			dgtTotalPower := int64(0)
			for k, vpow := range dgt.mapPowers {
				// All `VPowerProto` in dgt.mapPowers must have `From` equal to their map key 'k'.
				require.EqualValues(t, k, types.Address(vpow.From).String())
				// All `VPowerProto` in dgt.mapPowers must have `To` as dgt.addr.
				require.EqualValues(t, dgt.addr, vpow.To)

				sumPower := int64(0)
				for _, c := range vpow.PowerChunks {
					sumPower += c.Power
					dgtTotalPower += c.Power
				}
				// the `vpow.SumPower` is equal to the sum of it's PowerChunks.Power
				require.EqualValues(t, sumPower, vpow.SumPower)
			}

			require.Equal(t, dgtTotalPower, dgt.TotalPower())

			mature0 += dgt.MaturePower()
			rising0 += dgt.RisingPower()
			total0 += dgt.TotalPower()
		}
		require.Equal(t, mature0+rising0, total0)
		//fmt.Println("total", total0, "mature", mature0, "rising", rising0, "lastHeight", lastHeight, "height", h, "(h - lastH)", h-lastHeight, "ripening", r)

		total1, mature1, rising1, notYet := expectedXPower(vpowProtos, h, r)
		require.Equal(t, mature1+rising1+notYet, total1)
		require.Equal(t, total1-notYet, total0)
		require.Equal(t, mature1, mature0)
		require.Equal(t, rising1, rising0)
	}

	require.NoError(t, ledgerDgtees.Close())
	require.NoError(t, ledgerVPows.Close())
}

// Test_ComputeWeight tests that the sum of Wi and the result of Delegatee.Compute is same to decimal 6 places.
func Test_Compute_vs_Wi(t *testing.T) {

	//
	// init []*Delegatee and []*testPowObj
	tObjTotalPower := int64(0)
	var tObjs []*testPowObj
	for i := 0; i < 500_000; i++ {
		pow := bytes.RandInt64N(1_000_000_000)
		tObjs = append(tObjs, &testPowObj{
			vpow: pow,
			vdur: 0,
		})
		tObjTotalPower += pow
	}
	totalSupply := types.ToFons(uint64(tObjTotalPower))
	initialSupply := totalSupply.Clone()
	maxSupply := new(uint256.Int).Mul(initialSupply, uint256.NewInt(2))

	dgtees := make([]*Delegatee, 42)
	for i, tobj := range tObjs {
		dgt := dgtees[i%len(dgtees)]
		if dgt == nil {
			_, pub := crypto.NewKeypairBytes()
			dgt = NewDelegatee(pub)
			dgtees[i%len(dgtees)] = dgt
		}
		dgt.AddPower(types.RandAddress(), tobj.vpow, 1) // genesis block 1
	}

	preW := decimal.Zero
	inflationCycle := oneYearSeconds / 12
	for height := int64(1); height < oneYearSeconds*10; /*10 years*/ height += inflationCycle {
		W0, W1 := decimal.Zero, decimal.Zero

		// W of all Delegatee at `height`
		start := time.Now()
		for _, val := range dgtees {
			W0 = W0.Add(val.Compute(height, powerRipeningCycle, totalSupply, 200))
		}
		dur0 := time.Since(start)

		// W of all vpObjs at `height`
		start = time.Now()
		for _, vpo := range tObjs {
			vpo.vdur = height - 1
			W1 = W1.Add(Wi(vpo.vpow, vpo.vdur, powerRipeningCycle, decimal.NewFromBigInt(totalSupply.ToBig(), 0), 200))
		}
		dur1 := time.Since(start)

		W0 = W0.Truncate(6)
		W1 = W1.Truncate(6)

		//require.True(t, W1.Sub(W0).Abs().LessThanOrEqual(decimal.RequireFromString("0.001")), fmt.Sprintf("height: %v, Delegatee: %v, testPowObj: %v", height, W0, W1))
		require.Equal(t, W1.String(), W0.String(), fmt.Sprintf("height: %v, Delegatee: %v, testPowObj: %v", height, W0, W1))
		added := Sd(height, inflationCycle, 1, initialSupply, maxSupply, "0.3", W0, preW)
		_ = totalSupply.Add(totalSupply, added)
		preW = W0

		fmt.Println("height", height, "ValidatorW", W0, "testObjW", W1, "issued", types.ToBTOZ(added), "total", types.ToBTOZ(totalSupply), "dur0/1", dur0, dur1)
	}

}

func Test_Compute_vs_ComputeEx(t *testing.T) {
	maxDgtCnt := 42
	vpowCnt := 10000
	dgteeProtos, vpowProtos, ledgerDgtees, ledgerVPows, lastHeight, xerr := initVPowerLedger(maxDgtCnt, vpowCnt)
	require.NoError(t, xerr)
	require.Equal(t, maxDgtCnt, len(dgteeProtos))
	require.Equal(t, maxDgtCnt*vpowCnt, len(vpowProtos))

	totalSupply := types.ToFons(math.MaxUint64)

	nOp := 10
	dur0, dur1 := time.Duration(0), time.Duration(0)
	for i := 0; i < nOp; i++ {
		wCompute, wComputeEx := decimal.Zero, decimal.Zero

		r := powerRipeningCycle
		h := lastHeight + powerRipeningCycle/2 //lastHeight + bytes.RandInt64N(powerRipeningCycle+1)

		dgtProtos, xerr := LoadAllDelegateeProtos(ledgerDgtees)
		require.NoError(t, xerr)

		dgtees, xerr := LoadAllVPowerProtos(ledgerVPows, dgtProtos, h, r)
		require.NoError(t, xerr)
		require.Equal(t, len(dgteeProtos), len(dgtees))

		//Compute
		start := time.Now()
		for _, d := range dgtees {
			wCompute = wCompute.Add(d.Compute(h, r, totalSupply, 200))
		}
		dur0 += time.Since(start)

		//ComputeEx
		start = time.Now()
		for _, d := range dgtees {
			wComputeEx = wComputeEx.Add(d.ComputeEx(h, r, totalSupply, 200))
		}
		dur1 += time.Since(start)

		require.Equal(t, wComputeEx.String(), wComputeEx.String())
	}
	dur0 /= time.Duration(nOp)
	dur1 /= time.Duration(nOp)
	fmt.Printf("Compute:%v, ComputeEx:%v, ComputeEx/Compute:%.2v%%\n", dur0, dur1, dur1*100/dur0)
}

func Benchmark_Load(b *testing.B) {
	maxDgtCnt := 42
	vpowCnt := 10000
	dgteeProtos, vpowProtos, ledgerDgtees, ledgerVPows, lastHeight, xerr := initVPowerLedger(maxDgtCnt, vpowCnt)
	require.NoError(b, xerr)
	require.Equal(b, maxDgtCnt, len(dgteeProtos))
	require.Equal(b, maxDgtCnt*vpowCnt, len(vpowProtos))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := twoWeeksSeconds + rand.Int63n(oneYearSeconds)
		h := lastHeight + rand.Int63n(r/2)

		dgtProtos, xerr := LoadAllDelegateeProtos(ledgerDgtees)
		require.NoError(b, xerr)

		dgtees, xerr := LoadAllVPowerProtos(ledgerVPows, dgtProtos, h, r)
		require.NoError(b, xerr)
		require.Equal(b, maxDgtCnt, len(dgtees))
	}
}

func Benchmark_Compute(b *testing.B) {
	maxDgtCnt := 42
	vpowCnt := 10000
	dgteeProtos, vpowProtos, ledgerDgtees, ledgerVPows, lastHeight, xerr := initVPowerLedger(maxDgtCnt, vpowCnt)
	require.NoError(b, xerr)
	require.Equal(b, maxDgtCnt, len(dgteeProtos))
	require.Equal(b, maxDgtCnt*vpowCnt, len(vpowProtos))

	//// worst case: about 733.409784 ms/op (733409784 ns/op)
	//r := powerRipeningCycle
	//h := lastHeight + 1

	//// about 396.396993 ms/op (396396993 ns/op)
	//r := powerRipeningCycle
	//h := lastHeight + powerRipeningCycle/2

	// best case: about 48.866198 ms/op (48866198 ns/op)
	r := powerRipeningCycle
	h := lastHeight + powerRipeningCycle + 1

	dgtProtos, xerr := LoadAllDelegateeProtos(ledgerDgtees)
	require.NoError(b, xerr)

	dgtees, xerr := LoadAllVPowerProtos(ledgerVPows, dgtProtos, h, r)
	require.NoError(b, xerr)
	require.Equal(b, len(dgteeProtos), len(dgtees))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, d := range dgtees {
			_ = d.Compute(h, r, types.ToFons(math.MaxUint64), 200)
		}
	}
}

func Benchmark_ComputeEx(b *testing.B) {
	maxDgtCnt := 42
	vpowCnt := 10000
	dgteeProtos, vpowProtos, ledgerDgtees, ledgerVPows, lastHeight, xerr := initVPowerLedger(maxDgtCnt, vpowCnt)
	require.NoError(b, xerr)
	require.Equal(b, maxDgtCnt, len(dgteeProtos))
	require.Equal(b, maxDgtCnt*vpowCnt, len(vpowProtos))

	//// worst case: about 733.409784 ms/op (733409784 ns/op)
	//r := powerRipeningCycle
	//h := lastHeight + 1

	//// about 396.396993 ms/op (396396993 ns/op)
	//r := powerRipeningCycle
	//h := lastHeight + powerRipeningCycle/2

	// best case: about 33.799914 ms/op (33799914 ns/op)
	r := powerRipeningCycle
	h := lastHeight + powerRipeningCycle + 1

	dgtProtos, xerr := LoadAllDelegateeProtos(ledgerDgtees)
	require.NoError(b, xerr)

	dgtees, xerr := LoadAllVPowerProtos(ledgerVPows, dgtProtos, h, r)
	require.NoError(b, xerr)
	require.Equal(b, len(dgteeProtos), len(dgtees))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, d := range dgtees {
			_ = d.ComputeEx(h, r, types.ToFons(math.MaxUint64), 200)
		}
	}
}

func initVPowerLedger(dgteeCnt, powCnt int) ([]*DelegateeProto, []*VPowerProto, v1.IStateLedger[*DelegateeProto], v1.IStateLedger[*VPowerProto], int64, error) {
	var dgtProtos []*DelegateeProto
	var vpowProtos []*VPowerProto
	var lastHeight int64

	dbpath := filepath.Join(os.TempDir(), "vpower_ledger_test")
	_ = os.RemoveAll(dbpath)

	ledgerDgtees, xerr := v1.NewStateLedger[*DelegateeProto]("dgtees", dbpath, 128, func() v1.ILedgerItem { return &DelegateeProto{} }, log.NewNopLogger())
	if xerr != nil {
		return nil, nil, nil, nil, 0, xerr
	}
	ledgerVPow, xerr := v1.NewStateLedger[*VPowerProto]("vpows", dbpath, 2048, func() v1.ILedgerItem { return &VPowerProto{} }, log.NewNopLogger())
	if xerr != nil {
		return nil, nil, nil, nil, 0, xerr
	}

	for i := 0; i < dgteeCnt; i++ {
		_, pubKey := crypto.NewKeypairBytes()
		dgtee := NewDelegatee(pubKey)
		for j := 0; j < powCnt; j++ {
			txhash, from := bytes.RandBytes(32), types.RandAddress()
			pow, height := bytes.RandInt64N(1_000_000)+4000, bytes.RandInt64N(powerRipeningCycle)+1
			lastHeight = max(lastHeight, height)

			vpow := dgtee.AddPowerWithTxHash(from, pow, height, txhash)
			if xerr := ledgerVPow.Set(vpow, true); xerr != nil {
				return nil, nil, nil, nil, 0, xerr
			}

			//fmt.Println("init>>> ", "txhash", bytes.HexBytes(vpow.TxHash), "power", vpow.Power, "height", vpow.Height)

			vpowProtos = append(vpowProtos, vpow)
		}

		dgtProto := dgtee.GetDelegateeProto()
		if xerr := ledgerDgtees.Set(dgtProto, true); xerr != nil {
			return nil, nil, nil, nil, 0, xerr
		}
		dgtProtos = append(dgtProtos, dgtProto)
	}

	_, _, xerr = ledgerVPow.Commit()
	if xerr != nil {
		return nil, nil, nil, nil, 0, xerr
	}
	_, _, xerr = ledgerDgtees.Commit()
	if xerr != nil {
		return nil, nil, nil, nil, 0, xerr
	}

	//if xerr := ledgerDgtees.Close(); xerr != nil {
	//	return nil, nil, nil, nil, 0, xerr
	//}
	//if xerr := ledgerVPow.Close(); xerr != nil {
	//	return nil, nil, nil, nil, 0, xerr
	//}
	//
	//// re-open
	//ledgerDgtees, xerr = v1.NewStateLedger[*DelegateeProto]("dgtees", dbpath, 128, func() v1.ILedgerItem { return &DelegateeProto{} }, log.NewNopLogger())
	//if xerr != nil {
	//	return nil, nil, nil, nil, 0, xerr
	//}
	//ledgerVPow, xerr = v1.NewStateLedger[*VPowerProto]("vpows", dbpath, 2048, func() v1.ILedgerItem { return &VPowerProto{} }, log.NewNopLogger())
	//if xerr != nil {
	//	return nil, nil, nil, nil, 0, xerr
	//}

	fmt.Println("Delegatee Count", len(dgtProtos), "VPowerProto Count", len(vpowProtos))
	return dgtProtos, vpowProtos, ledgerDgtees, ledgerVPow, lastHeight, nil
}

func expectedXPower(vpowArr []*VPowerProto, h, r int64) (int64, int64, int64, int64) {
	total, mature, rising, notYet := int64(0), int64(0), int64(0), int64(0)
	for _, vpow := range vpowArr {
		for _, c := range vpow.PowerChunks {
			dur := h - c.Height
			if dur >= r {
				mature += c.Power
			} else if dur >= 0 && dur < r {
				rising += c.Power
			} else {
				// because this is testing,
				// there may be VPower that has the height greater than the current height `h`.
				notYet += c.Power
			}
			total += c.Power
		}
	}
	return total, mature, rising, notYet
}
