package test

import (
	"errors"
	"fmt"
	"github.com/beatoz/beatoz-go/cmd/commands"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/node"
	beatozweb3 "github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/containerd/continuity/fs"
	tmcfg "github.com/tendermint/tendermint/config"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmnode "github.com/tendermint/tendermint/node"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var (
	peers []*PeerMock
)

type PeerMock struct {
	PeerID  string
	ChainID string
	Config  *cfg.Config
	nd      *tmnode.Node
	RPCURL  string
	WSEnd   string

	Pass []byte
}

func NewPeerMock(chain, id string, p2pPort, rpcPort int, logLevel string) *PeerMock {
	config := cfg.DefaultConfig()
	config.LogLevel = logLevel
	config.P2P.AllowDuplicateIP = true
	config.P2P.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", p2pPort)
	config.RPC.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", rpcPort)
	config.Config.Moniker = fmt.Sprintf("peer-%v@%v", id, chain)
	config.SetRoot(filepath.Join(os.TempDir(), "beatoz_test_"+id))
	os.RemoveAll(config.RootDir) // reset root directory
	tmcfg.EnsureRoot(config.RootDir)

	if err := config.ValidateBasic(); err != nil {
		panic(fmt.Errorf("error in rootConfig file: %v", err))
	}

	return &PeerMock{
		PeerID:  id,
		ChainID: chain,
		Config:  config,
		RPCURL:  fmt.Sprintf("http://localhost:%d", rpcPort),
		WSEnd:   fmt.Sprintf("ws://localhost:%d/websocket", rpcPort),
		Pass:    []byte("1111"),
	}
}

func (peer *PeerMock) CopyGenesisFrom(source string) error {
	return fs.CopyFile(
		peer.Config.GenesisFile(),
		source)
}

func (peer *PeerMock) IDAddress() (string, error) {
	if peer.nd == nil {
		return "", errors.New("not created node")
	}
	ni := peer.nd.NodeInfo()
	na, _ := ni.NetAddress()
	return fmt.Sprintf("%s@127.0.0.1:%d", ni.ID(), na.Port), nil
}

func (peer *PeerMock) SetPersistentPeer(other *PeerMock) {
	peer.Config.P2P.PersistentPeers, _ = other.IDAddress()
}

func (peer *PeerMock) SetPass(pass []byte) {
	peer.Pass = pass
}

func (peer *PeerMock) Init() error {
	return commands.InitFilesWith(peer.ChainID, peer.Config, 1, peer.Pass, 500, peer.Pass)
}

func (peer *PeerMock) Start() error {
	logger := tmlog.NewNopLogger()

	logger = tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout))
	if peer.Config.LogFormat == "json" {
		logger = tmlog.NewTMJSONLogger(tmlog.NewSyncWriter(os.Stdout))
	}
	logger, err := tmflags.ParseLogLevel(peer.Config.LogLevel, logger, tmcfg.DefaultLogLevel)

	peer.Config.RPC.MaxSubscriptionClients = 501
	peer.Config.RPC.MaxSubscriptionsPerClient = 100
	peer.Config.RPC.SubscriptionBufferSize = 1000

	peer.nd, err = node.NewBeatozNode(peer.Config, peer.Pass, logger)
	if err != nil {
		return fmt.Errorf("failed to create beatoz: %w", err)
	}

	err = peer.nd.Start()
	if err != nil {
		return fmt.Errorf("failed to start beatoz: %w", err)
	}
	return nil
}

func (peer *PeerMock) Stop() {
	if peer.nd.IsRunning() {
		if err := peer.nd.ProxyApp().Stop(); err != nil {
			panic(fmt.Errorf("unable to stop the beatoz proxy app: %v", err))
		}
		if err := peer.nd.Stop(); err != nil {
			panic(fmt.Errorf("unable to stop the beatoz node: %v", err))
		}
	}
}

func (peer *PeerMock) WalletPath() string {
	return filepath.Join(peer.Config.RootDir, "walkeys")
}

func (peer *PeerMock) PrivValKeyPath() string {
	return peer.Config.PrivValidatorKeyFile()
}

func randPeer() *PeerMock {
	rand.Seed(time.Now().UnixNano())
	n := rand.Intn(len(peers))
	return peers[n]
}

func randBeatozWeb3() *beatozweb3.BeatozWeb3 {
	peer := randPeer()
	return beatozweb3.NewBeatozWeb3(beatozweb3.NewHttpProvider(peer.RPCURL))
}

func runPeers(n int) {
	for i := 0; i < n; i++ {
		ll := "*:error"
		//if i == 0 {
		//  // print debug logs only on the first peer.
		//	ll = "beatoz_AcctCtrler:debug,beatoz_EVMCtrler:debug,*:error"
		//}
		_peer := NewPeerMock("beatoz_test_chain", strconv.FormatInt(int64(i), 10), 46656+i, 36657+i, ll)
		if err := _peer.Init(); err != nil {
			panic(err)
		}

		if i > 0 {
			// use genesis file of peer[0]
			prevPeer := peers[i-1]
			if err := _peer.CopyGenesisFrom(prevPeer.Config.GenesisFile()); err != nil {
				panic(err)
			}
			_peer.SetPersistentPeer(prevPeer)
		}

		if err := _peer.Start(); err != nil {
			panic(err)
		}

		fmt.Printf("peer_%d: p2p(%s) root(%s)\n", i, _peer.Config.P2P.ListenAddress, _peer.Config.RootDir)
		peers = append(peers, _peer)

		time.Sleep(time.Second)
	}
}
