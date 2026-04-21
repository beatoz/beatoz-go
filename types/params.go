package types

type ForkBlocks struct {
	BTIP27Block int64
	BTIP35Block int64
}

var (
	devnetForkBlocks = ForkBlocks{
		// all forks are enabled
	}
	testnetForkBlocks = ForkBlocks{
		BTIP27Block: 100_000,
		BTIP35Block: 100_000,
	}

	mainnetForkBlocks = ForkBlocks{
		// all forks are enabled
	}

	chainForkBlocks = map[string]ForkBlocks{
		"0xbea700": devnetForkBlocks,
		"0xbea701": testnetForkBlocks,
		"0xbea702": mainnetForkBlocks,
	}
)

func IsBTIP27(chainId string, height int64) bool {
	h0 := int64(0)
	if forkBlocks, ok := chainForkBlocks[chainId]; ok {
		h0 = forkBlocks.BTIP27Block
	}
	// If there is no forkBlocks then `h0` is 0; BTIP27 is enabled by default.
	return isBlockForked(h0, height)
}

func IsBTIP35(chainId string, height int64) bool {
	h0 := int64(0)
	if forkBlocks, ok := chainForkBlocks[chainId]; ok {
		h0 = forkBlocks.BTIP35Block
	}
	// If there is no forkBlocks then `h0` is 0; BTIP27 is enabled by default.
	return isBlockForked(h0, height)
}

func isBlockForked(h0, head int64) bool {
	// If `h0` is `0`, any `head` is forked.
	return h0 <= head
}
