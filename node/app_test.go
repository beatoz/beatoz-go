package node

import (
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"strings"
	"testing"
)

func Test_InitChain(t *testing.T) {
	// max total supply is less than initial total supply
	req := abcitypes.RequestInitChain{
		Validators: abcitypes.ValidatorUpdates{
			{Power: 1}, {Power: 1}, {Power: 1}, // 3000000000000000000
		},
		AppStateBytes: []byte(`{
"assetHolders": [
	{"address": "AAAAAA", "balance":"1000000000000000000"},
	{"address": "BBBBBB", "balance":"1000000000000000000"},
	{"address": "CCCCCC", "balance":"1000000000000000000"}
],
"govParams":{
	"maxTotalSupply":"5999999999999999999"
}}`)}

	_, _, err := checkRequestInitChain(req)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "error: initial supply"))

	// success case
	req = abcitypes.RequestInitChain{
		Validators: abcitypes.ValidatorUpdates{
			{Power: 1}, {Power: 1}, {Power: 1},
		},
		AppStateBytes: []byte(`{
"assetHolders": [
	{"address": "AAAAAA", "balance":"1000000000000000000"},
	{"address": "BBBBBB", "balance":"1000000000000000000"},
	{"address": "CCCCCC", "balance":"1000000000000000000"}
],
"govParams":{
	"maxTotalSupply":"6000000000000000000"
}}`)}

	_, genTotalSupply, err := checkRequestInitChain(req)
	require.NoError(t, err)
	require.Equal(t, "6000000000000000000", genTotalSupply.Dec())
}
