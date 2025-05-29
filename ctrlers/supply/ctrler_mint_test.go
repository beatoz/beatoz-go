package supply

import (
	"fmt"
	vpowmock "github.com/beatoz/beatoz-go/ctrlers/mocks/vpower"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/ctrlers/vpower"
	"github.com/beatoz/beatoz-go/libs/fxnum"
	types2 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/robaho/fixed"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func Test_Mint(t *testing.T) {
	require.NoError(t, os.RemoveAll(config.RootDir))

	initSupply := types.PowerToAmount(350_000_000)
	adjustedSupply := initSupply.Clone()
	adjustedHeight := int64(1)
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

	////
	////Use VPowerCtrler
	//fmt.Println("Test using VPowerCtrler")
	//vpowMock, xerr := vpower.NewVPowerCtrler(config, int(govMock.MaxValidatorCnt()), log.NewNopLogger())
	//require.NoError(t, xerr)
	//
	//wal := acctMock.RandWallet()
	//dgtee := vpower.NewDelegatee(wal.GetPubKey())
	//
	//vpow := vpower.NewVPower(dgtee.Address(), dgtee.Address()) // self power
	//xerr = vpowMock.BondPowerChunk(dgtee, vpow, 70_000_000, 1, bytes.RandBytes(32), true)
	//require.NoError(t, xerr)
	//
	//height0 := govMock.InflationCycleBlocks()
	//bctx := types.TempBlockContext("mint-test-chain", height0, time.Now(), govMock, acctMock, nil, nil, vpowMock)
	//
	//// before vpowCtrler.EndBlock. (vpowCtrler.lastValidators is nil)
	//// expect 0 minting
	//ctrler.requestMint(bctx)
	//result, xerr := ctrler.waitMint(bctx)
	//require.NoError(t, xerr)
	//supplyHeight := result.newSupply.Height
	//totalSupply := new(uint256.Int).SetBytes(result.newSupply.XSupply)
	//changeSupply := new(uint256.Int).SetBytes(result.newSupply.XChange)
	//
	//require.Equal(t, height0, supplyHeight)
	//require.Equal(t, initSupply.String(), totalSupply.String())
	//require.Equal(t, "0", changeSupply.String())
	//
	//_, xerr = vpowMock.EndBlock(bctx)
	//require.NoError(t, xerr)
	//// End of Use VPowerCtrler
	////

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

		si := Si(currHeight, int64(govMock.AssumedBlockInterval()), adjustedHeight, adjustedSupply, govMock.MaxTotalSupply(), govMock.InflationWeightPermil(), wa).Floor()
		expectedTotalSupply := uint256.MustFromBig(si.BigInt())
		expectedChange := new(uint256.Int).Sub(expectedTotalSupply, totalSupply)
		//fmt.Println("expected", "height", currHeight, "wa", wa.String(), "adjustedSupply", adjustedSupply, "adjustedHeight", 1, "max", govMock.MaxTotalSupply(), "lamda", govMock.InflationWeightPermil(), "total", expectedTotalSupply, "pre.total", totalSupply, "change", expectedChange)

		bctx := types.TempBlockContext("mint-test-chain", currHeight, time.Now(), govMock, acctMock, nil, nil, vpowMock)
		ctrler.requestMint(bctx)
		result, xerr := ctrler.waitMint(bctx)
		require.NoError(t, xerr)
		totalSupply = new(uint256.Int).Add(totalSupply, result.sumMintedAmt)
		changeSupply = result.sumMintedAmt.Clone()

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

type testData struct {
	atHeight            int64
	weight              string
	adjustedHeight      int64
	adjustedSupply      string
	expectedTotalSupply string
}

var sampleData = []testData{
	{604800, "0.0182625", 1, "350000000000000000000000000", "350031213593743817990116967"},
	//{7862400, "0.1498358", 3775312, "90090972755328500000000000", "93099438518382400000000000"},
	//{15120000, "0.3155568", 3775312, "93099438518382400000000000", "110391615576530000000000000"},
	//{22377600, "0.5051176", 3775312, "110391615576530000000000000", "153471552699752000000000000"},
	//{29635200, "0.6561728", 9908459, "78849958256933700000000000", "140494611359761000000000000"},
	//{36892800, "0.7469144", 9908459, "140494611359761000000000000", "224527706888588000000000000"},
	//{44150400, "0.7984154", 9908459, "224527706888588000000000000", "318712632693887000000000000"},
}

// 7e26 - ((7e26-35e25)/((1.29)^( 0.0182625*((604800-1)/31536000))))
// 350031213_5937438179901169673800766242360022212667536399889505 from wolframalpha
// 350031213_593743817990116967.3800766 (chatgpt, precision7, fixed, final round) <- 350031213593743817990116967.38007662423600222126675363998895050575599794509825908
// 350031217_215424384144934271.8629498 (chatgpt, precision7, fixed, round)
// 350031217_215424384144934271 (fixed)
// 350031213_511584579350750774 (decimal)
func Test_Si(t *testing.T) {
	maxSupply := uint256.MustFromDecimal("700000000000000000000000000")
	//initSupply := types.PowerToAmount(350_000_000)
	lambda := int32(290)
	//ripening := types.YearSeconds

	fmt.Println("DivisionPrecision", decimal.DivisionPrecision)
	decimal.DivisionPrecision = 7
	for _, data := range sampleData {
		atHeight := data.atHeight
		weight := fxnum.FxNum{
			Fixed: fixed.NewS(data.weight),
		}
		adjustedHeight := data.adjustedHeight
		adjustedSupply := uint256.MustFromDecimal(data.adjustedSupply)
		expected := uint256.MustFromDecimal(data.expectedTotalSupply)

		fxTotal := Si(atHeight, 1, adjustedHeight, adjustedSupply, maxSupply, lambda, weight).Floor()
		u256Total := uint256.MustFromBig(fxTotal.BigInt())

		decW, err := decimal.NewFromString(data.weight)
		require.NoError(t, err)
		decTotal := decimalSi(atHeight, 1, adjustedHeight, adjustedSupply, maxSupply, lambda, decW).Floor()
		//require.Equal(t, expectedTotalSupply.Dec(), total.String())
		fmt.Println("---\ndiff", absDiff(u256Total, expected).Dec(), "expected", expected.String())
		fmt.Println("diff", absDiff(u256Total, uint256.MustFromBig(decTotal.BigInt())), "actual", u256Total.String(), "decimal", decTotal.String())
	}
}

func Test_Annual_Si(t *testing.T) {
	adjustedHeight := int64(1)
	adjustedSupply := uint256.MustFromDecimal("350000000000000000000000000")
	totalSupply := adjustedSupply.Clone()

	powChunks := []*vpower.PowerChunkProto{
		{Height: 1, Power: 42_000_000},
	}

	govMock.GetValues().InflationWeightPermil = 900
	fmt.Println("tau", govMock.BondingBlocksWeightPermil())
	fmt.Println("lamda", govMock.InflationWeightPermil())

	heightYears := int64(0)
	preSupply := totalSupply.Clone()
	for h := govMock.InflationCycleBlocks(); h < types.YearSeconds*40; h += govMock.InflationCycleBlocks() {
		burned := false
		if h == govMock.InflationCycleBlocks()*110 {
			// burn x %
			burnRate := decimal.NewFromFloat(0.9)
			adjustedSupply = uint256.MustFromBig(decimal.NewFromBigInt(totalSupply.ToBig(), 0).Mul(burnRate).BigInt())
			adjustedHeight = h - 100 // burned before 100 blocks
			totalSupply = adjustedSupply.Clone()
			b, f := types2.FromFons(adjustedSupply)
			fmt.Printf("Burn - adjustedHeight: %d, adjustedSupply: %v.%v \n", adjustedHeight, b, f)
			burned = true
		}

		vw := vpower.FxNumWeightOfPowerChunks(
			powChunks, h,
			govMock.RipeningBlocks(),
			govMock.BondingBlocksWeightPermil(),
			adjustedSupply) // test using adjustedSupply not totalSupply
		decTotalSupply := Si(h,
			1, adjustedHeight, adjustedSupply,
			govMock.MaxTotalSupply(),
			govMock.InflationWeightPermil(),
			vw).Floor()
		totalSupply = uint256.MustFromBig(decTotalSupply.BigInt())
		if !burned {
			require.True(t, totalSupply.Gt(preSupply), fmt.Sprintf("height %d: %v <= %v", h, totalSupply, preSupply))
		}

		hY := H(h-adjustedHeight, 1)
		heightYears = hY.Int()
		b, f := types2.FromFons(preSupply)
		b0, f0 := types2.FromFons(totalSupply)
		diff := new(uint256.Int).Sub(totalSupply, preSupply)
		sign := "-"
		if diff.Sign() < 0 {
			sign = "-"
			_ = diff.Abs(diff)
		} else if diff.Sign() > 0 {
			sign = "+"
		}
		b1, f1 := types2.FromFons(diff)
		fmt.Printf("year: %2d, height: %10v, preSupply: %v.%018v, totalSupply: %v.%018v, weight: %s, diff(%s): %v.%018v\n",
			heightYears, h, b, f, b0, f0, vw.StringN(7), sign, b1, f1)

		preSupply = totalSupply.Clone()
	}
}
