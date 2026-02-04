package commands

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/beatoz/beatoz-go/genesis"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	types2 "github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/stretchr/testify/require"
	tmcfg "github.com/tendermint/tendermint/config"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmtypes "github.com/tendermint/tendermint/types"
)

func Test_ChainId(t *testing.T) {
	cases := []struct {
		chainId string
		isErr   bool
	}{
		{"", false},
		{"localnet0", false},
		{"12aa", false},
		{"0X12aa", false},
		{"0x123Z", false},
		{"0x123", false},
		{"0x0123", true},
		{"1234", true},
	}
	params := &InitParams{
		ValCnt:          1,
		ValSecret:       bytes.RandBytes(12),
		HolderCnt:       1,
		HolderSecret:    bytes.RandBytes(12),
		BlockSizeLimit:  22020096,
		BlockGasLimit:   rand.Int63n(100_000_000),
		MaxTotalSupply:  1000,
		InitTotalSupply: 1000,
		InitVotingPower: 1000,
	}

	for _, c := range cases {
		params.ChainID = c.chainId
		err := params.Validate()
		if c.isErr {
			require.NoError(t, err, "chainId", params.ChainID)
		} else {
			require.Error(t, err, "chainId", params.ChainID)
		}
	}
}

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
			ChainID:         "0x01",
			ValCnt:          rand.Intn(100) + 1,
			ValSecret:       bytes.RandBytes(12),
			HolderCnt:       rand.Intn(100) + 1,
			HolderSecret:    bytes.RandBytes(12),
			BlockSizeLimit:  22020096,
			BlockGasLimit:   rand.Int63n(100_000_000),
			MaxTotalSupply:  rand.Int63n(1000) + 100,
			InitTotalSupply: rand.Int63n(1000) + 100,
			InitVotingPower: rand.Int63n(1000) + 100,
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
		actualInitSupply := types2.PowerToAmount(actualInitPower)
		for _, holder := range appState.AssetHolders {
			_ = actualInitSupply.Add(actualInitSupply, holder.Balance)
		}
		require.Equal(t, types2.ToGrans(params.InitTotalSupply).Dec(), actualInitSupply.Dec())

		//
		// GovParams
		govParams := appState.GovParams
		require.Equal(t, types2.ToGrans(params.MaxTotalSupply).Dec(), govParams.MaxTotalSupply().Dec())

		require.NoError(t, os.RemoveAll(config.RootDir))

		successCase++
	}

	require.NotEqual(t, 0, failCase, "no fail case")
	require.NotEqual(t, 0, successCase, "no success case")
}
