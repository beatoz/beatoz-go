package rpc

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/consensus"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
)

func TestEthSyncingEncoding(t *testing.T) {
	for _, tt := range []struct {
		name string
		info coretypes.SyncInfo
		want string
	}{
		{"idle", coretypes.SyncInfo{}, `false`},
		{"caught up", coretypes.SyncInfo{EarliestBlockHeight: 1, LatestBlockHeight: 20}, `false`},
		{"starting", coretypes.SyncInfo{CatchingUp: true}, `{"startingBlock":"0x0","currentBlock":"0x0","highestBlock":"0x0"}`},
		{"catching up", coretypes.SyncInfo{CatchingUp: true, EarliestBlockHeight: 16, LatestBlockHeight: 42}, `{"startingBlock":"0x10","currentBlock":"0x2a","highestBlock":"0x2a"}`},
		{"large height", coretypes.SyncInfo{CatchingUp: true, EarliestBlockHeight: 1, LatestBlockHeight: math.MaxInt64}, `{"startingBlock":"0x1","currentBlock":"0x7fffffffffffffff","highestBlock":"0x7fffffffffffffff"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatEthSyncing(tt.info)
			require.NoError(t, err)
			encoded, err := json.Marshal(&result)
			require.NoError(t, err)
			require.JSONEq(t, tt.want, string(encoded))

			handler, fixture := ethTestRPCHandler(t)
			fixture.baseHeight = tt.info.EarliestBlockHeight
			fixture.blockHeight = tt.info.LatestBlockHeight
			fixture.blocks = make(map[int64]*coretypes.ResultBlock)
			for _, height := range []int64{fixture.baseHeight, fixture.blockHeight} {
				if height > 0 {
					fixture.blocks[height] = ethTestBlock(height)
				}
			}
			fixture.env.ConsensusReactor = consensus.NewReactor(&consensus.State{}, tt.info.CatchingUp)
			response := ethServerRequest(t, handler, "eth_syncing", `[]`)
			require.Nil(t, response.Error)
			require.JSONEq(t, tt.want, string(response.Result))
		})
	}
}

func TestFormatEthSyncingInvalidHeights(t *testing.T) {
	for _, info := range []coretypes.SyncInfo{
		{CatchingUp: true, EarliestBlockHeight: -1},
		{CatchingUp: true, LatestBlockHeight: -1},
	} {
		_, err := formatEthSyncing(info)
		require.Error(t, err)
	}
}
