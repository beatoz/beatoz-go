package test

import (
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	tmcfg "github.com/tendermint/tendermint/config"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	test_on_internal_node(m)
	//test_on_external_node(m)
}

func test_on_internal_node(m *testing.M) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {

		runPeers(3)

		wg.Done()
	}()
	wg.Wait()
	time.Sleep(time.Second)

	defaultRpcNode = peers[len(peers)-1]
	subWg, err := waitEvent("tm.event='NewBlock'", func(event *coretypes.ResultEvent, err error) bool {
		return true
	})
	if err != nil {
		panic(err)
	}
	subWg.Wait()

	prepareTest(peers) // peers[0] is active validator node

	exitCode := m.Run()

	for _, p := range peers {
		p.Stop()
		time.Sleep(time.Second)
		os.RemoveAll(p.Config.RootDir)
	}

	os.Exit(exitCode)
}

func test_on_external_node(m *testing.M) {
	//// node to be executed externally

	config := cfg.DefaultConfig()
	config.LogLevel = ""
	root, _ := filepath.Abs("../.tmp/test-localnet0")
	config.SetRoot(root) //config.SetRoot("/Users/kysee/beatoz_localnet_0")
	tmcfg.EnsureRoot(config.RootDir)
	if err := config.ValidateBasic(); err != nil {
		panic(fmt.Errorf("error in rootConfig file: %v", err))
	}

	peer := &PeerMock{
		ChainID: "test-localnet0",
		Config:  config,
		RPCURL:  "http://localhost:26657",
		WSEnd:   "ws://localhost:26657/websocket",
		Pass:    []byte("1111"),
	}
	peers = append(peers, peer)
	defaultRpcNode = peer

	prepareTest([]*PeerMock{peer})

	exitCode := m.Run()

	os.Exit(exitCode)
}
