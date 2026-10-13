package evm

import (
	"context"
	"testing"
	"time"

	cfg "github.com/beatoz/beatoz-go/cmd/config"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func Test_Query(t *testing.T) {
	config := cfg.DefaultConfig("1")
	config.App.QueryTimeout = 20 * time.Millisecond
	config.SetRoot(t.TempDir())

	acctHandler := &acctHandlerMock{}
	ctrler := NewEVMCtrler(config, acctHandler, tmlog.NewNopLogger())
	t.Cleanup(func() { require.NoError(t, ctrler.Close()) })

	bctx := ctrlertypes.NewBlockContext(
		abcitypes.RequestBeginBlock{
			Header: tmproto.Header{ChainID: config.ChainIdHex(), Height: 1, Time: time.Unix(1, 0)},
		},
		govMock, acctHandler, ctrler, nil, nil,
	)
	_, xerr := ctrler.BeginBlock(bctx)
	require.NoError(t, xerr)

	from := types.RandAddress()
	echoAddr := types.RandAddress()
	revertAddr := types.RandAddress()
	loopAddr := types.RandAddress()

	// Copy calldata to memory and return it.
	ctrler.stateDBWrapper.SetCode(echoAddr.Array20(), []byte{0x36, 0x60, 0x00, 0x60, 0x00, 0x37, 0x36, 0x60, 0x00, 0xf3})
	// REVERT with empty data.
	ctrler.stateDBWrapper.SetCode(revertAddr.Array20(), []byte{0x60, 0x00, 0x60, 0x00, 0xfd})
	// JUMPDEST; PUSH1 0; JUMP.
	ctrler.stateDBWrapper.SetCode(loopAddr.Array20(), []byte{0x5b, 0x60, 0x00, 0x56})
	_, height, xerr := ctrler.Commit()
	require.NoError(t, xerr)

	// Normal queries.
	input := []byte{0x01, 0x02, 0x03, 0x04}
	tests := []struct {
		name           string
		path           string
		to             types.Address
		input          []byte
		height         int64
		wantReturnData []byte
		wantVMError    string
	}{
		{
			name: "vm_call",
			path: "vm_call", to: echoAddr, input: input, height: height,
			wantReturnData: input,
		},
		{
			name: "vm_estimate_gas",
			path: "vm_estimate_gas", to: echoAddr, input: input, height: height,
		},
		{
			name: "latest_height",
			path: "vm_call", to: echoAddr, input: input,
			wantReturnData: input,
		},
		{
			name: "evm_error",
			path: "vm_call", to: revertAddr, height: height,
			wantVMError: "execution reverted",
		},
	}

	for _, test := range tests {
		value, xerr := ctrler.Query(abcitypes.RequestQuery{
			Path:   test.path,
			Data:   queryTestData(from, test.to, test.input),
			Height: test.height,
		})

		require.NoError(t, xerr, "case=%s", test.name)
		var result ctrlertypes.VMCallResult
		require.NoError(t, jsonx.Unmarshal(value, &result), "case=%s", test.name)
		require.Greater(t, result.UsedGas, int64(0), "case=%s", test.name)
		require.Equal(t, test.wantVMError, result.Err, "case=%s", test.name)
		require.Equal(t, test.wantReturnData, result.ReturnData, "case=%s", test.name)
	}

	// Timeout.
	type queryResult struct {
		value []byte
		xerr  xerrors.XError
	}
	resultCh := make(chan queryResult, 1)
	go func() {
		value, xerr := ctrler.Query(abcitypes.RequestQuery{
			Path:   "vm_call",
			Data:   queryTestData(from, loopAddr, nil),
			Height: height,
		})
		resultCh <- queryResult{value, xerr}
	}()

	select {
	case result := <-resultCh:
		require.Nil(t, result.value)
		require.Error(t, result.xerr)
		require.Equal(t, xerrors.ErrCodeQuery, result.xerr.Code())
		require.ErrorContains(t, result.xerr, context.DeadlineExceeded.Error())
	case <-time.After(2 * time.Second):
		t.Fatal("timeout query did not return")
	}
}

func queryTestData(from, to types.Address, input []byte) []byte {
	data := make([]byte, 0, types.AddrSize*2+len(input))
	data = append(data, from...)
	data = append(data, to...)
	return append(data, input...)
}
