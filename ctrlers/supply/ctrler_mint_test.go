package supply

import (
	"fmt"
	vpowmock "github.com/beatoz/beatoz-go/ctrlers/mocks/vpower"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ctrlers/vpower"
	btztypes "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func Test_Mint(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	initSupply := btztypes.PowerToAmount(350_000_000)
	ctrler, xerr := initLedger(initSupply)
	require.NoError(t, xerr)

	//
	// Use VPowerHandlerMock
	valsCnt := min(acctMock.WalletLen(), 10)
	valWals := make([]*web3.Wallet, valsCnt)
	for i := 0; i < valsCnt; i++ {
		valWals[i] = acctMock.GetWallet(i)
	}
	vpowMock := vpowmock.NewVPowerHandlerMock(valWals, len(valWals))
	totalSupply := initSupply.Clone()
	changeSupply := uint256.NewInt(0)
	fmt.Println("Test Mint using VPowerHandlerMock", "validator number", valsCnt, "total power", vpowMock.GetTotalPower())

	preRewards := make(map[string]*uint256.Int)
	for currHeight := govMock.InflationCycleBlocks(); /*int64(2)*/ currHeight < govMock.InflationCycleBlocks()*1000; currHeight += govMock.InflationCycleBlocks() {
		// expect x minting
		weightInfo, xerr := vpowMock.ComputeWeight(
			currHeight,
			govMock.InflationCycleBlocks(),
			govMock.RipeningBlocks(),
			govMock.BondingBlocksWeightPermil(),
			totalSupply)
		require.NoError(t, xerr)

		wa := weightInfo.SumWeight() //.Truncate(precision)
		//wa := vpower.FxNumWeightOfPowerChunks(vpowMock.PowerChunks, currHeight, govMock.RipeningBlocks(), govMock.BondingBlocksWeightPermil(), totalSupply)
		//wa = wa.Truncate(precision)

		sd := Sd(
			heightYears(currHeight, govMock.AssumedBlockInterval()),
			totalSupply, govMock.MaxTotalSupply(), govMock.InflationWeightPermil(), wa).Floor()
		expectedChange := uint256.MustFromBig(sd.BigInt())
		expectedTotalSupply := new(uint256.Int).Add(totalSupply, expectedChange)
		//fmt.Println("expected", "height", currHeight, "wa", wa.String(), "adjustedHeight", adjustedHeight, "max", govMock.MaxTotalSupply(), "lamda", govMock.InflationWeightPermil(), "total", expectedTotalSupply, "pre.total", totalSupply, "change", expectedChange)

		bctx := types.TempBlockContext("mint-test-chain", currHeight, time.Now(), govMock, acctMock, nil, nil, vpowMock)
		ctrler.requestMint(bctx)
		result, xerr := ctrler.waitMint(bctx)
		require.NoError(t, xerr)
		totalSupply = new(uint256.Int).Add(totalSupply, result.sumMintedAmt)
		changeSupply = result.sumMintedAmt.Clone()

		if changeSupply.Dec() == "0" {
			fmt.Println("changeSupply", changeSupply.Dec())
		}
		require.NotEqual(t, "0", changeSupply.Dec())
		require.NotEqual(t, expectedTotalSupply.Dec(), initSupply.Dec())
		changeDiff := absDiff(expectedChange, changeSupply)
		//require.Equal(t, expectedChange.Dec(), changeSupply.Dec())
		require.LessOrEqual(t, changeDiff.Uint64(), uint64(1), "height", currHeight, "changeDiff", changeDiff.Dec())
		supplyDiff := absDiff(expectedTotalSupply, totalSupply)
		//require.Equal(t, expectedTotalSupply.Dec(), totalSupply.Dec(), "height", currHeight)
		require.LessOrEqual(t, supplyDiff.Uint64(), uint64(1), "height", currHeight, "supplyDiff", supplyDiff.Dec())

		sumMint := uint256.NewInt(0)
		for _, mintRwd := range result.rewards {
			_ = sumMint.Add(sumMint, mintRwd.amt)

			//
			// check reward amount of beneficiary
			accumRwd, xerr := ctrler.readReward(mintRwd.addr)
			require.NoError(t, xerr)
			require.Equal(t, currHeight, accumRwd.Height())
			require.Equal(t, mintRwd.amt.Dec(), accumRwd.issued.Dec())

			_preAmt, ok := preRewards[mintRwd.addr.String()]
			if !ok {
				preRewards[mintRwd.addr.String()] = accumRwd.CumulatedAmount()
			} else {
				require.Equal(t, _preAmt.Add(_preAmt, mintRwd.amt).Dec(), accumRwd.CumulatedAmount().Dec())
				preRewards[mintRwd.addr.String()] = _preAmt
			}
		}

		delta := new(uint256.Int).Sub(expectedChange, sumMint)
		require.LessOrEqual(t, new(uint256.Int).Abs(delta).Uint64(), uint64(1), delta)

		//fmt.Println("---")
		//fmt.Println("height", currHeight, "totalSupply", totalSupply.Dec(), "changeSupply", changeSupply.Dec())
		//for _, rwd := range result.rewards {
		//	fmt.Println("height", currHeight, "beneficary", rwd.addr, "reward", rwd.amt.Dec())
		//}
	}

	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.RootDir))
}

// the following results are calculated by google spreadsheets
var expectedSupplys = []struct {
	height int64
	supply int64
}{
	{16329600, 350307915},
	{16934400, 350333828},
	{17539200, 350360997},
	{18144000, 350389445},
	{18748800, 350419194},
	{19353600, 350450267},
	{19958400, 350482686},
	{20563200, 350516473},
	{21168000, 350551651},
	{21772800, 350588241},
	{22377600, 350626267},
	{22982400, 350665750},
}

func Test_Sd(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))
	initSupply := btztypes.PowerToAmount(350_000_000)
	adjustedHeight := int64(1)
	ctrler, xerr := initLedger(initSupply)
	require.NoError(t, xerr)

	//
	// Use VPowerHandlerMock
	valsCnt := min(acctMock.WalletLen(), 21)
	valWals := make([]*web3.Wallet, valsCnt)
	for i := 0; i < valsCnt; i++ {
		valWals[i] = acctMock.GetWallet(i)
	}
	powerPerVal := int64(1_000_000)
	vpowMock := vpowmock.NewVPowerHandlerMockWithPower(valWals, len(valWals), 1_000_000)
	require.Equal(t, powerPerVal*int64(len(valWals)), vpowMock.GetTotalPower())
	totalSupply := initSupply.Clone()
	//preSupply := totalSupply.Clone()
	fmt.Println("Test Mint using VPowerHandlerMock", "validator number", valsCnt, "total power", vpowMock.GetTotalPower())

	for currHeight := int64(1); currHeight <= 22982400; currHeight++ {
		if currHeight%govMock.InflationCycleBlocks() != 0 {
			continue
		}

		//// Mint...
		//preSupply = totalSupply.Clone()

		weightInfo, xerr := vpowMock.ComputeWeight(
			currHeight,
			govMock.InflationCycleBlocks(),
			govMock.RipeningBlocks(),
			govMock.BondingBlocksWeightPermil(),
			totalSupply)
		require.NoError(t, xerr)

		wa := weightInfo.SumWeight()

		scaledH := heightYears(currHeight-adjustedHeight, govMock.AssumedBlockInterval())

		decSd := Sd(
			scaledH,
			totalSupply,
			govMock.MaxTotalSupply(),
			govMock.InflationWeightPermil(),
			wa).Floor()

		mintSupply := uint256.MustFromBig(decSd.BigInt())
		_ = totalSupply.Add(totalSupply, mintSupply)

		//fmt.Println("height", currHeight,
		//	"preSupply", btztypes.FormattedString(preSupply),
		//	"totalSupply", btztypes.FormattedString(totalSupply),
		//	"mintSupply", btztypes.FormattedString(mintSupply),
		//	"scaledH", scaledH, "wa", wa, "decSd", decSd)
		for _, expect := range expectedSupplys {
			if expect.height == currHeight {
				require.LessOrEqual(t, absDiff64(expect.supply, int64(btztypes.FromGrans(totalSupply))), int64(2))
			}
		}

	}
	require.NoError(t, ctrler.Close())
	require.NoError(t, os.RemoveAll(config.RootDir))
}
func Test_Annual_Supply_AdjustTo0(t *testing.T) {
	initSupply := uint256.MustFromDecimal("350000000000000000000000000")
	totalSupply := initSupply.Clone()
	adjustedHeight := int64(1)

	powChunks := []*vpower.PowerChunkProto{
		{Height: 1, Power: 21_000_000},
	}

	govMock.GetValues().InflationWeightPermil = 3 // 0.003
	fmt.Println("tau", govMock.BondingBlocksWeightPermil())
	fmt.Println("lamda", govMock.InflationWeightPermil())
	fmt.Println("inflation.cycle", govMock.InflationCycleBlocks())

	burned := false
	preSupply := totalSupply.Clone()
	currHeight := types.DaySeconds
	for {
		//burning
		if currHeight == types.YearSeconds*35 {
			// burn x %
			preSupply = totalSupply.Clone()
			remainRate := decimal.NewFromFloat(0.8)
			totalSupply = uint256.MustFromBig(decimal.NewFromBigInt(totalSupply.ToBig(), 0).Mul(remainRate).BigInt())
			//adjustedHeight = currHeight
			burned = true

			//fmt.Printf("year: %2d, height: %10v, preSupply: %s, totalSupply: %s, weight: %s, scaledH:%s, exp: %v, burned: -%s\n",
			//	currHeight/types.YearSeconds, currHeight,
			//	btztypes.FormattedString(preSupply),
			//	btztypes.FormattedString(totalSupply),
			//	"0", "0", "0",
			//	btztypes.FormattedString(new(uint256.Int).Sub(preSupply, totalSupply)))

		}
		// bonding/unbonding
		//if rand.Intn(7) == 0 {
		//	add := (rand.Intn(2) == 1)
		//	if add {
		//		pow := rand.Int63n(1_000_000) + 4_000
		//		powChunks = append(powChunks,
		//			&vpower.PowerChunkProto{
		//				Power:  pow,
		//				Height: currHeight,
		//			})
		//		//fmt.Printf("\tAdd voting power - height: %d, power: %d \n", currHeight, pow)
		//	} else {
		//		rdx := rand.Intn(len(powChunks))
		//		pc := powChunks[rdx]
		//		pow := rand.Int63n(pc.Power) + 1
		//		pc.Power -= pow
		//		if pc.Power == 0 {
		//			powChunks = append(powChunks[:rdx], powChunks[rdx+1:]...)
		//		}
		//		//fmt.Printf("\tSub voting power - height: %d, power: %d, change: %d\n", pc.Height, pc.Power, pow)
		//	}
		//}

		if currHeight%govMock.InflationCycleBlocks() != 0 {
			currHeight += types.DaySeconds
			continue
		}

		// Mint...
		preSupply = totalSupply.Clone()

		vw := vpower.FxNumWeightOfPowerChunks(
			powChunks, currHeight,
			govMock.RipeningBlocks(),
			govMock.BondingBlocksWeightPermil(),
			totalSupply)

		scaledH := heightYears(currHeight-adjustedHeight, govMock.AssumedBlockInterval())

		decSd := Sd(
			scaledH,
			totalSupply,
			govMock.MaxTotalSupply(),
			govMock.InflationWeightPermil(),
			vw).Floor()

		mintSupply := uint256.MustFromBig(decSd.BigInt())
		_ = totalSupply.Add(totalSupply, mintSupply)

		if !burned {
			//require.True(t, totalSupply.Gt(preSupply),
			//	fmt.Sprintf("height %d: %v >= %v, w=%v, scaledH:%v, adjust=%v, minted=%v",
			//		currHeight, preSupply, totalSupply, vw, scaledH, adjustedSupply, btztypes.FormattedString(mintSupply)))
			if totalSupply.Lt(preSupply) {
				t.Logf("totalSupply is dereased!!!! - height %d: %v >= %v, w=%v, scaledH:%v, exp:%v, minted=%v",
					currHeight,
					btztypes.FormattedString(preSupply),
					btztypes.FormattedString(totalSupply),
					vw, scaledH, vw.Mul(scaledH),
					btztypes.FormattedString(mintSupply))
			}
			burned = false
		}

		{
			// log annual total supply
			currHeightYear := currHeight / types.YearSeconds
			nextHeightYear := (currHeight + govMock.InflationCycleBlocks()) / types.YearSeconds
			if currHeightYear != nextHeightYear || currHeightYear >= 100 {

				fmt.Printf("year: %2d, height: %10v(%v), preSupply: %s, totalSupply: %s, weight: %s, scaledH:%s, exp: %v, minted: %s\n",
					currHeightYear, currHeight, adjustedHeight,
					btztypes.FormattedString(preSupply),
					btztypes.FormattedString(totalSupply),
					vw.StringN(7), scaledH.StringN(7), vw.Mul(scaledH).StringN(7),
					btztypes.FormattedString(mintSupply))
				if currHeightYear >= 100 {
					break
				}
			}
		}

		currHeight += types.DaySeconds
	}
}

func Test_Annual_Supply_AdjustToN(t *testing.T) {
	initSupply := uint256.MustFromDecimal("350000000000000000000000000")
	totalSupply := initSupply.Clone()
	adjustedHeight := int64(1)

	powChunks := []*vpower.PowerChunkProto{
		{Height: 1, Power: 21_000_000},
	}

	govMock.GetValues().InflationWeightPermil = 3 // 0.003
	fmt.Println("tau", govMock.BondingBlocksWeightPermil())
	fmt.Println("lamda", govMock.InflationWeightPermil())
	fmt.Println("inflation.cycle", govMock.InflationCycleBlocks())

	burned := false
	preSupply := totalSupply.Clone()
	currHeight := types.DaySeconds
	for {
		//burning
		if currHeight == types.YearSeconds*35 {
			// burn x %
			remainRate := decimal.NewFromFloat(0.8)
			remainSupply := uint256.MustFromBig(decimal.NewFromBigInt(totalSupply.ToBig(), 0).Mul(remainRate).BigInt())
			//preLastSupply := uint256.MustFromBig(decimal.NewFromBigInt(preSupply.ToBig(), 0).Mul(remainRate).BigInt())

			//estimatedHeight := adjustHeight(
			//	remainSupply,
			//	preLastSupply,
			//	govMock.MaxTotalSupply(),
			//	powChunks[0].Power,
			//	govMock.InflationWeightPermil(),
			//	govMock.AssumedBlockInterval(),
			//)
			estimatedHeight := decimal.NewFromInt(currHeight).Mul(remainRate).IntPart()
			adjustedHeight = currHeight - estimatedHeight
			//fmt.Println("currHeight", currHeight, "estimatedHeight", estimatedHeight, "adjustedHeight", adjustedHeight)

			preSupply = totalSupply.Clone()
			totalSupply = remainSupply.Clone()
			burned = true

			//fmt.Printf("year: %2d, height: %10v, preSupply: %s, totalSupply: %s, weight: %s, scaledH:%s, exp: %v, burned: -%s\n",
			//	currHeight/types.YearSeconds, currHeight,
			//	btztypes.FormattedString(preSupply),
			//	btztypes.FormattedString(totalSupply),
			//	"0", "0", "0",
			//	btztypes.FormattedString(new(uint256.Int).Sub(preSupply, totalSupply)))
		}
		// bonding/unbonding
		//if rand.Intn(7) == 0 {
		//	add := (rand.Intn(2) == 1)
		//	if add {
		//		pow := rand.Int63n(1_000_000) + 4_000
		//		powChunks = append(powChunks,
		//			&vpower.PowerChunkProto{
		//				Power:  pow,
		//				Height: h,
		//			})
		//		//fmt.Printf("\tAdd voting power - height: %d, power: %d \n", h, pow)
		//	} else {
		//		rdx := rand.Intn(len(powChunks))
		//		pc := powChunks[rdx]
		//		pow := rand.Int63n(pc.Power) + 1
		//		pc.Power -= pow
		//		if pc.Power == 0 {
		//			powChunks = append(powChunks[:rdx], powChunks[rdx+1:]...)
		//		}
		//		//fmt.Printf("\tSub voting power - height: %d, power: %d, change: %d\n", pc.Height, pc.Power, pow)
		//	}
		//}

		if currHeight%govMock.InflationCycleBlocks() != 0 {
			currHeight += types.DaySeconds
			continue
		}

		// Mint...

		preSupply = totalSupply.Clone()

		vw := vpower.FxNumWeightOfPowerChunks(
			powChunks, currHeight,
			govMock.RipeningBlocks(),
			govMock.BondingBlocksWeightPermil(),
			totalSupply)

		scaledH := heightYears(currHeight-adjustedHeight, govMock.AssumedBlockInterval())

		decSd := Sd(
			scaledH,
			totalSupply,
			govMock.MaxTotalSupply(),
			govMock.InflationWeightPermil(),
			vw).Floor()

		mintSupply := uint256.MustFromBig(decSd.BigInt())
		_ = totalSupply.Add(totalSupply, mintSupply)

		if !burned {
			//require.True(t, totalSupply.Gt(preSupply),
			//	fmt.Sprintf("height %d: %v >= %v, w=%v, scaledH:%v, adjust=%v, minted=%v",
			//		h, preSupply, totalSupply, vw, scaledH, adjustedSupply, btztypes.FormattedString(mintSupply)))
			if totalSupply.Lt(preSupply) {
				t.Logf("totalSupply is dereased!!!! - height %d: %v >= %v, w=%v, scaledH:%v, exp:%v, minted=%v",
					currHeight,
					btztypes.FormattedString(preSupply),
					btztypes.FormattedString(totalSupply),
					vw, scaledH, vw.Mul(scaledH),
					btztypes.FormattedString(mintSupply))
			}
			burned = false
		}

		{
			// log annual total supply
			currHeightYear := currHeight / types.YearSeconds
			nextHeightYear := (currHeight + govMock.InflationCycleBlocks()) / types.YearSeconds
			if currHeightYear != nextHeightYear || currHeightYear >= 100 {

				fmt.Printf("year: %2d, height: %10v/%v, preSupply: %s, totalSupply: %s, weight: %s, scaledH:%s, exp: %v, minted: %s\n",
					currHeightYear, currHeight, adjustedHeight,
					btztypes.FormattedString(preSupply),
					btztypes.FormattedString(totalSupply),
					vw.StringN(7), scaledH.StringN(7), vw.Mul(scaledH).StringN(7),
					btztypes.FormattedString(mintSupply))

				if currHeightYear >= 100 {
					break
				}
			}
		}

		currHeight += types.DaySeconds
	}
}

//func Test_Annual_Supply_Sd2(t *testing.T) {
//	initSupply := uint256.MustFromDecimal("350000000000000000000000000")
//	totalSupply := initSupply.Clone()
//	adjustedSupply := initSupply.Clone()
//	adjustedHeight := int64(1)
//
//	powChunks := []*vpower.PowerChunkProto{
//		{Height: 1, Power: 21_000_000},
//	}
//
//	govMock.GetValues().InflationWeightPermil = 2999
//	fmt.Println("tau", govMock.BondingBlocksWeightPermil())
//	fmt.Println("lamda", govMock.InflationWeightPermil())
//	fmt.Println("inflation.cycle", govMock.InflationCycleBlocks())
//
//	burned := false
//	preSupply := totalSupply.Clone()
//	currHeight := types.DaySeconds
//	for {
//		//burning
//		if currHeight == types.YearSeconds*14 {
//			// burn x %
//			remainRate := decimal.NewFromFloat(0.8)
//			remainSupply := uint256.MustFromBig(decimal.NewFromBigInt(totalSupply.ToBig(), 0).Mul(remainRate).BigInt())
//			adjustedHeight = currHeight
//			adjustedSupply = remainSupply.Clone()
//
//			preSupply = totalSupply.Clone()
//			totalSupply = remainSupply.Clone()
//			burned = true
//
//			//fmt.Printf("year: %2d, height: %10v(%v), adjustedSupply: %s, totalSupply: %s, weight: %s, scaledH:%s, exp: %v, burned: -%s\n",
//			//	currHeight/types.YearSeconds, currHeight, adjustedHeight,
//			//	btztypes.FormattedString(adjustedSupply),
//			//	btztypes.FormattedString(totalSupply),
//			//	"0", "0", "0",
//			//	btztypes.FormattedString(new(uint256.Int).Sub(preSupply, totalSupply)))
//		}
//		////bonding / unbonding
//		//if rand.Intn(7) == 0 {
//		//	add := (rand.Intn(7) == 0)
//		//	if add {
//		//		pow := rand.Int63n(1_000_000) + 4_000
//		//		powChunks = append(powChunks,
//		//			&vpower.PowerChunkProto{
//		//				Power:  pow,
//		//				Height: currHeight,
//		//			})
//		//		//fmt.Printf("\tAdd voting power - height: %d, power: %d \n", h, pow)
//		//	} else {
//		//		rdx := rand.Intn(len(powChunks))
//		//		pc := powChunks[rdx]
//		//		pow := rand.Int63n(pc.Power) + 1
//		//		pc.Power -= pow
//		//		if pc.Power == 0 {
//		//			powChunks = append(powChunks[:rdx], powChunks[rdx+1:]...)
//		//		}
//		//		//fmt.Printf("\tSub voting power - height: %d, power: %d, change: %d\n", pc.Height, pc.Power, pow)
//		//	}
//		//}
//
//		if currHeight%govMock.InflationCycleBlocks() != 0 {
//			currHeight += types.DaySeconds
//			continue
//		}
//
//		// Mint...
//
//		preSupply = totalSupply.Clone()
//
//		vw := vpower.FxNumWeightOfPowerChunks(
//			powChunks, currHeight,
//			govMock.RipeningBlocks(),
//			govMock.BondingBlocksWeightPermil(),
//			totalSupply)
//
//		scaledH := heightYears(currHeight-adjustedHeight, govMock.AssumedBlockInterval())
//
//		decSd := Sd2(
//			scaledH,
//			adjustedSupply,
//			govMock.MaxTotalSupply(),
//			govMock.InflationCycleBlocks(),
//			govMock.AssumedBlockInterval(),
//			govMock.InflationWeightPermil(),
//			vw).Floor()
//
//		mintSupply := uint256.MustFromBig(decSd.BigInt())
//		_ = totalSupply.Add(totalSupply, mintSupply)
//
//		if !burned {
//			//require.True(t, totalSupply.Gt(preSupply),
//			//	fmt.Sprintf("height %d: %v >= %v, w=%v, scaledH:%v, adjust=%v, minted=%v",
//			//		h, preSupply, totalSupply, vw, scaledH, adjustedSupply, btztypes.FormattedString(mintSupply)))
//			if totalSupply.Lt(preSupply) {
//				t.Logf("totalSupply is dereased!!!! - height %d: %v >= %v, w=%v, scaledH:%v, exp:%v, minted=%v",
//					currHeight,
//					btztypes.FormattedString(preSupply),
//					btztypes.FormattedString(totalSupply),
//					vw, scaledH, vw.Mul(scaledH),
//					btztypes.FormattedString(mintSupply))
//			}
//			burned = false
//		}
//
//		{
//			// log annual total supply
//			currHeightYear := currHeight / types.YearSeconds
//			nextHeightYear := (currHeight + govMock.InflationCycleBlocks()) / types.YearSeconds
//			m, _ := btztypes.FromGransRem(totalSupply)
//			if currHeightYear != nextHeightYear || m >= 693_000_000 {
//
//				fmt.Printf("year: %2d, height: %10v(%v), adjustedSupply: %s, totalSupply: %s, weight: %s, scaledH:%s, exp: %v, minted: %s\n",
//					currHeightYear, currHeight, adjustedHeight,
//					btztypes.FormattedString(adjustedSupply),
//					btztypes.FormattedString(totalSupply),
//					vw.StringN(7), scaledH.StringN(7), vw.Mul(scaledH).StringN(7),
//					btztypes.FormattedString(mintSupply))
//
//				if m >= 693_000_000 { // 99% of 700_000_000 (max)
//					fmt.Printf("totalSupply(%s) is reached max supply in %d years (height: %v)\n", btztypes.FormattedString(totalSupply), currHeightYear, currHeight)
//					break
//				}
//			}
//		}
//
//		currHeight += types.DaySeconds
//	}
//}

func Benchmark_AdjustHeight(b *testing.B) {

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		smax := btztypes.ToGrans(700_000_000)
		si := bytes.RandU256IntN(smax)
		lastSi := bytes.RandU256IntN(si)
		vp := bytes.RandInt64N(btztypes.FromGrans(lastSi))
		b.StartTimer()

		_ = adjustHeight(si, lastSi, smax, vp, govMock.InflationWeightPermil(), govMock.AssumedBlockInterval())
	}
}

func absDiff(x, y *uint256.Int) *uint256.Int {
	result := new(uint256.Int)
	switch x.Cmp(y) {
	case -1: // x < y
		result.Sub(y, x)
	case 0: // x == y
		result.SetUint64(0)
	case 1: // x > y
		result.Sub(x, y)
	}
	return result
}

func absDiff64(x, y int64) int64 {
	if x > y {
		return x - y
	}
	return y - x
}

// return_height = {(si * YearSeconds) / (vpAmt * blockIntv)} * {ln((smax - preSi)/(smax-si)) / ln(1+lambda)}
func adjustHeight(si, preSi, smax *uint256.Int, vp int64, lambda, blockIntv int32) int64 {
	dLambdaAddOne := decimal.New(int64(lambda), -3)
	dLambdaAddOne = dLambdaAddOne.Add(decimal.NewFromInt(1))
	dsi := decimal.NewFromBigInt(si.ToBig(), 0)
	d0 := decimal.NewFromInt(types.YearSeconds).Mul(dsi)
	d0 = d0.Div(decimal.New(vp, 18).Mul(decimal.NewFromInt(int64(blockIntv))))

	var err error
	dlastSi := decimal.NewFromBigInt(preSi.ToBig(), 0)
	dlog := decimal.NewFromBigInt(smax.ToBig(), 0).Sub(dlastSi)
	dlog = dlog.Div(decimal.NewFromBigInt(smax.ToBig(), 0).Sub(dsi))
	dlog, err = dlog.Ln(int32(decimal.DivisionPrecision))
	if err != nil {
		panic(err)
	}
	dLambdaAddOne, err = dLambdaAddOne.Ln(int32(decimal.DivisionPrecision))
	if err != nil {
		panic(err)
	}
	dlog = dlog.Div(dLambdaAddOne)

	h := d0.Mul(dlog)
	return h.IntPart()
}
