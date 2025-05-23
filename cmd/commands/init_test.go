package commands

import (
	"fmt"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/genesis"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	types2 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
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
		params := &InitParams{
			ChainID:              "init-test-chain-id",
			ValCnt:               rand.Intn(100) + 1,
			ValSecret:            bytes.RandBytes(12),
			HolderCnt:            rand.Intn(100) + 1,
			HolderSecret:         bytes.RandBytes(12),
			BlockGasLimit:        rand.Int63n(36_000_000),
			AssumedBlockInterval: fmt.Sprintf("%ds", rand.Int31n(3600)+1),
			InflationCycleBlocks: rand.Int63n(types.WeekSeconds*8) + 1,
			MaxTotalSupply:       rand.Int63n(1000) + 100,
			InitTotalSupply:      rand.Int63n(1000) + 100,
			InitVotingPower:      rand.Int63n(1000) + 100,
		}

		err := InitFilesWith(config, params)
		if params.InitTotalSupply > params.MaxTotalSupply || params.InitVotingPower > params.InitTotalSupply {
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
		require.Equal(t, params.ChainID, genDoc.ChainID)

		appState := genesis.GenesisAppState{}
		require.NoError(t, jsonx.Unmarshal(genDoc.AppState, &appState))

		//
		// voting power
		actualInitPower := int64(0)
		for _, val := range genDoc.Validators {
			actualInitPower += val.Power
		}
		require.Equal(t, params.InitVotingPower, actualInitPower)

		//
		// initial supply
		actualInitSupply := uint256.NewInt(uint64(actualInitPower))
		for _, holder := range appState.AssetHolders {
			_ = actualInitSupply.Add(actualInitSupply, holder.Balance)
		}
		require.Equal(t, types2.ToFons(uint64(params.InitTotalSupply)).Dec(), actualInitSupply.Dec())

		//
		// GovParams
		govParams := appState.GovParams
		bintv, err := time.ParseDuration(params.AssumedBlockInterval)
		require.NoError(t, err)
		require.Equal(t, int32(bintv.Seconds()), govParams.AssumedBlockInterval(), params.AssumedBlockInterval)
		require.Equal(t, params.InflationCycleBlocks, govParams.InflationCycleBlocks())
		require.Equal(t, types2.ToFons(uint64(params.MaxTotalSupply)).Dec(), govParams.MaxTotalSupply().Dec())

		require.NoError(t, os.RemoveAll(config.RootDir))

		successCase++
	}

	require.NotEqual(t, 0, failCase, "no fail case")
	require.NotEqual(t, 0, successCase, "no success case")
}
