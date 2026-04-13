package types

import (
	"strings"

	"github.com/holiman/uint256"
)

const (
	Bud = "Bud"
)

var (
	devnetForkBlocks = map[string]int64{
		Bud: 0,
	}
	testnetForkBlocks = map[string]int64{
		Bud: 100_000,
	}

	mainnetForkBlocks = map[string]int64{
		Bud: 0,
	}

	chainForkBlocks = map[string]map[string]int64{
		"0xbea700": devnetForkBlocks,
		"0xbea701": testnetForkBlocks,
		"0xbea702": mainnetForkBlocks,
	}
)

func IsForkedBig(chainId *uint256.Int, height int64, forkName string) bool {
	cid := chainId.Hex()
	return IsForkedStr(cid, height, forkName)
}

func IsForkedStr(chainId string, height int64, forkName string) bool {
	h0 := chainForkBlocks[strings.ToLower(chainId)][forkName]
	return height >= h0
}
