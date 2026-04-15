package types

type ForkBlocks struct {
	Bud int64
}

var (
	devnetForkBlocks = ForkBlocks{
		Bud: 0,
	}
	testnetForkBlocks = ForkBlocks{
		Bud: 100_000,
	}

	mainnetForkBlocks = ForkBlocks{
		Bud: 0,
	}

	chainForkBlocks = map[string]ForkBlocks{
		"0xbea700": devnetForkBlocks,
		"0xbea701": testnetForkBlocks,
		"0xbea702": mainnetForkBlocks,
	}
)

func IsBud(chainId string, height int64) bool {
	forkBlocks, ok := chainForkBlocks[chainId]
	if !ok {
		return false
	}
	return isBlockForked(forkBlocks.Bud, height)
}

func isBlockForked(h0, head int64) bool {

	return head >= h0
}
