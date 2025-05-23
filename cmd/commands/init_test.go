package commands

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/genesis"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	types2 "github.com/beatoz/beatoz-go/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	tmcfg "github.com/tendermint/tendermint/config"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmtypes "github.com/tendermint/tendermint/types"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func Test_InitialAmounts(t *testing.T) {

	logger = tmlog.NewNopLogger()
	config := rootConfig
	config.RootDir = filepath.Join(os.TempDir(), "init-cmd-test")

	failCase := 0
	successCase := 0

	for op := 0; op < 100; op++ {
		require.NoError(t, os.RemoveAll(config.RootDir))
		tmcfg.EnsureRoot(config.RootDir)

		// gloval variables
		beatozChainID = "init-test-chain-id"
		holderCnt = rand.Intn(100) + 1
		privValCnt = rand.Intn(100) + 1
		assumedBlockInterval = fmt.Sprintf("%ds", rand.Int31n(3600)+1)
		inflationCycleBlocks = rand.Int63n(types.WeekSeconds*8) + 1
		maxTotalSupply = rand.Int63n(1000) + 100
		initTotalSupply = rand.Int63n(1000) + 100
		initVotingPower = rand.Int63n(1000) + 100

		vsecrets := make([][]byte, privValCnt)
		for i, _ := range vsecrets {
			vsecrets[i] = []byte{0x1}
		}
		hsecrets := make([][]byte, holderCnt)
		for i, _ := range hsecrets {
			hsecrets[i] = []byte{0x1}
		}

		err := InitFilesWith(beatozChainID, config, privValCnt, nil, holderCnt, nil)
		if initTotalSupply > maxTotalSupply || initVotingPower > initTotalSupply {
			require.Error(t, err)
			failCase++
			continue
		}

		require.NoError(t, err)

		// check values in genesis.json
		// Calculate SHA-256 hash of the genesis file.
		f, err := os.Open(config.GenesisFile())
		require.NoError(t, err)
		defer f.Close()

		fi, err := f.Stat()
		require.NoError(t, err)

		jz := make([]byte, fi.Size())
		_, err = f.Read(jz)
		require.NoError(t, err)

		genDoc := &tmtypes.GenesisDoc{}
		require.NoError(t, tmjson.Unmarshal(jz, genDoc))

		//
		// chain id
		require.Equal(t, beatozChainID, genDoc.ChainID)

		appState := genesis.GenesisAppState{}
		require.NoError(t, jsonx.Unmarshal(genDoc.AppState, &appState))

		//
		// voting power
		actualInitPower := int64(0)
		for _, val := range genDoc.Validators {
			actualInitPower += val.Power
		}
		require.Equal(t, initVotingPower, actualInitPower)

		//
		// initial supply
		actualInitSupply := uint256.NewInt(0)
		for _, holder := range appState.AssetHolders {
			_ = actualInitSupply.Add(actualInitSupply, holder.Balance)
		}
		require.Equal(t, types2.ToFons(uint64(initTotalSupply)).Dec(), actualInitSupply.Dec())

		//
		// GovParams
		govParams := appState.GovParams
		bintv, err := time.ParseDuration(assumedBlockInterval)
		require.NoError(t, err)
		require.Equal(t, int32(bintv.Seconds()), govParams.AssumedBlockInterval(), assumedBlockInterval)
		require.Equal(t, inflationCycleBlocks, govParams.InflationCycleBlocks())
		require.Equal(t, types2.ToFons(uint64(maxTotalSupply)).Dec(), govParams.MaxTotalSupply().Dec())

		require.NoError(t, os.RemoveAll(config.RootDir))

		successCase++
	}

	require.NotEqual(t, 0, failCase, "no fail case")
	require.NotEqual(t, 0, successCase, "no success case")
}
