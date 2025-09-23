package commands

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/node"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/sfeeder/client"
	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/libs/log"
	tmp2p "github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

var (
	genesisHash             []byte
	privValSecretFeederAddr string
)

// AddNodeFlags exposes some common configuration options on the command-line
// These are exposed for convenience of commands embedding a node
func AddNodeFlags(cmd *cobra.Command) {
	// bind flags
	cmd.Flags().String("moniker", rootConfig.Moniker, "beatoz name")

	// priv val flags
	cmd.Flags().String(
		"priv_validator_laddr",
		rootConfig.PrivValidatorListenAddr,
		"socket address to listen on for connections from external priv_validator process")

	cmd.Flags().StringVar(
		&privValSecretFeederAddr,
		"priv_validator_secret_feeder",
		"",
		"socket address to listen on for connections from external priv_validator process")

	// node flags
	cmd.Flags().Bool("fast_sync", rootConfig.FastSyncMode, "fast blockchain syncing")
	cmd.Flags().BytesHexVar(
		&genesisHash,
		"genesis_hash",
		[]byte{},
		"optional SHA-256 hash of the genesis file")
	cmd.Flags().Int64("consensus.double_sign_check_height", rootConfig.Consensus.DoubleSignCheckHeight,
		"how many blocks to look back to check existence of the beatoz's "+
			"consensus votes before joining consensus")

	// abci flags
	cmd.Flags().String(
		"proxy_app",
		rootConfig.ProxyApp,
		"proxy app address, or one of: 'kvstore',"+
			" 'persistent_kvstore',"+
			" 'counter',"+
			" 'counter_serial' or 'noop' for local testing.")
	cmd.Flags().String("abci", rootConfig.ABCI, "specify abci transport (socket | grpc)")

	// provider flags
	cmd.Flags().String("rpc.laddr", rootConfig.RPC.ListenAddress, "RPC listen address. Port required")
	cmd.Flags().StringSlice("rpc.cors_allowed_origins", rootConfig.RPC.CORSAllowedOrigins, "")
	cmd.Flags().String(
		"rpc.grpc_laddr",
		rootConfig.RPC.GRPCListenAddress,
		"GRPC listen address (BroadcastTx only). Port required")
	cmd.Flags().Bool("rpc.unsafe", rootConfig.RPC.Unsafe, "enabled unsafe provider methods")
	cmd.Flags().String("rpc.pprof_laddr", rootConfig.RPC.PprofListenAddress, "pprof listen address (https://golang.org/pkg/net/http/pprof)")
	cmd.Flags().Int("rpc.max_subscription_clients", rootConfig.RPC.MaxSubscriptionClients, "Maximum number of unique clientIDs that can /subscribe")
	// p2p flags
	cmd.Flags().String(
		"p2p.laddr",
		rootConfig.P2P.ListenAddress,
		"beatoz listen address. (0.0.0.0:0 means any interface, any port)")
	cmd.Flags().String("p2p.seeds", rootConfig.P2P.Seeds, "comma-delimited ID@host:port seed nodes")
	cmd.Flags().String("p2p.persistent_peers", rootConfig.P2P.PersistentPeers, "comma-delimited ID@host:port persistent peers")
	cmd.Flags().String("p2p.unconditional_peer_ids",
		rootConfig.P2P.UnconditionalPeerIDs, "comma-delimited IDs of unconditional peers")
	cmd.Flags().Bool("p2p.upnp", rootConfig.P2P.UPNP, "enable/disable UPNP port forwarding")
	cmd.Flags().Bool("p2p.pex", rootConfig.P2P.PexReactor, "enable/disable Peer-Exchange")
	cmd.Flags().Bool("p2p.seed_mode", rootConfig.P2P.SeedMode, "enable/disable seed mode")
	cmd.Flags().String("p2p.private_peer_ids", rootConfig.P2P.PrivatePeerIDs, "comma-delimited private peer IDs")

	// consensus flags
	cmd.Flags().Bool(
		"consensus.create_empty_blocks",
		rootConfig.Consensus.CreateEmptyBlocks,
		"set this to false to only produce blocks when there are txs or when the AppHash changes")
	cmd.Flags().String(
		"consensus.create_empty_blocks_interval",
		rootConfig.Consensus.CreateEmptyBlocksInterval.String(),
		"the possible interval between empty blocks")

	// db flags
	cmd.Flags().String(
		"db_backend",
		rootConfig.DBBackend,
		"database backend: goleveldb | cleveldb | boltdb | rocksdb | badgerdb")
	cmd.Flags().String(
		"db_dir",
		rootConfig.DBPath,
		"database directory")
}

// NewRunNodeCmd returns the command that allows the CLI to start a node.
// It can be used with a custom PrivValidator and in-process ABCI application.
func NewRunNodeCmd(nodeProvider node.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start",
		Aliases: []string{"run"},
		Short:   "Run the beatoz",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkGenesisHash(rootConfig); err != nil {
				return err
			}

			// get chainId from genesis file
			data, err := os.ReadFile(rootConfig.GenesisFile())
			if err != nil {
				return fmt.Errorf("can't open genesis file: %w", err)
			}
			genDoc, err := types.GenesisDocFromJSON(data)
			if err != nil {
				return fmt.Errorf("can't parse genesis file: %w", err)
			}
			rootConfig.SetChainId(genDoc.ChainID)
			logger.Info("BEATOZ Blockchain", "ChainId", rootConfig.ChainId())

			var s []byte
			if privValSecretFeederAddr != "" {
				wkf, err := crypto.OpenWalletKey(libs.NewFileReader(rootConfig.PrivValidatorKeyFile()))
				if err != nil {
					return err
				}
				nodeKey, err := tmp2p.LoadNodeKey(rootConfig.NodeKeyFile())

				if err != nil {
					return err
				}
				s, err = client.ReadRemoteCredential(string(nodeKey.ID()), privValSecretFeederAddr, wkf.Address)
				if err != nil {
					return err
				}
			} else {
				_secret := os.Getenv("BEATOZ_VALIDATOR_SECRET")
				if _secret == "" {
					s = libs.ReadCredential(fmt.Sprintf("Passphrase for %v: ", filepath.Base(rootConfig.PrivValidatorKeyFile())))
				} else {
					s = []byte(_secret)
				}
				defer libs.ClearCredential(s)
			}

			n, err := nodeProvider(rootConfig, s, logger)

			libs.ClearCredential(s)

			if err != nil {
				return fmt.Errorf("failed to create beatoz: %w", err)
			}

			if err := n.Start(); err != nil {
				return fmt.Errorf("failed to start beatoz: %w", err)
			}

			logger.Info("Started beatoz", "nodeInfo", n.Switch().NodeInfo())

			// Stop upon receiving SIGTERM or CTRL-C.
			trapSignal(logger, func() {
				if n.IsRunning() {
					if err := n.ProxyApp().Stop(); err != nil {
						logger.Error("unable to stop the beatoz proxy app", "error", err)
					}
					if err := n.Stop(); err != nil {
						logger.Error("unable to stop the beatoz node", "error", err)
					}
				}
			})

			// Run forever.
			select {}
		},
	}

	AddNodeFlags(cmd)
	return cmd
}

func checkGenesisHash(config *cfg.Config) error {
	if len(genesisHash) == 0 || config.Genesis == "" {
		return nil
	}

	// Calculate SHA-256 hash of the genesis file.
	f, err := os.Open(config.GenesisFile())
	if err != nil {
		return fmt.Errorf("can't open genesis file: %w", err)
	}
	defer f.Close()
	h := crypto.DefaultHasher()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("error when hashing genesis file: %w", err)
	}
	actualHash := h.Sum(nil)

	// Compare with the flag.
	if !bytes.Equal(genesisHash, actualHash) {
		return fmt.Errorf(
			"--genesis_hash=%X does not match %s hash: %X",
			genesisHash, config.GenesisFile(), actualHash)
	}

	return nil
}

// trapSignal() comes from tmos.TrapSignal
func trapSignal(logger log.Logger, cb func()) {
	var signals = []os.Signal{
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		//syscall.SIGQUIT,
		//syscall.SIGABRT,
		syscall.SIGKILL,
		syscall.SIGTERM,
		//syscall.SIGSTOP,
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)
	go func() {
		for sig := range c {
			logger.Info("signal trapped", "msg", log.NewLazySprintf("captured %v, exiting...", sig.String()))
			if cb != nil {
				cb()
			}
			os.Exit(0)
		}
	}()
}
