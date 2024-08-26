package commands

import (
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/genesis"
	"github.com/beatoz/beatoz-go/libs"
	acrypto "github.com/beatoz/beatoz-go/types/crypto"
	"github.com/holiman/uint256"
	"github.com/spf13/cobra"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/p2p"
	tmtypes "github.com/tendermint/tendermint/types"
	"path/filepath"
)

//// InitFilesCmd initialises a fresh Tendermint Core instance.
//var InitFilesCmd = &cobra.Command{
//	Use:   "init",
//	Short: "Initialize a node",
//	RunE:  initFiles,
//}

var (
	beatozChainID           = "mainnet"
	walkeyCnt               = 9
	privValSecret           string
	privValSecretFeederAddr string
)

// NewRunNodeCmd returns the command that allows the CLI to start a node.
// It can be used with a custom PrivValidator and in-process ABCI application.
func NewInitFilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a beatoz",
		RunE:  initFiles,
	}
	AddInitFlags(cmd)
	return cmd
}

func AddInitFlags(cmd *cobra.Command) {
	// bind flags
	cmd.Flags().StringVar(
		&beatozChainID,
		"chain_id",
		beatozChainID, // default name
		"the id of chain to generate (e.g. mainnet, testnet, devnet and others)")
	cmd.Flags().IntVar(
		&walkeyCnt,
		"holders",
		walkeyCnt, // default value is 9
		"the number of holder's accounts to generate. "+
			"if you create a new genesis of your own blockchain, "+
			"you need to generate accounts of genesis holders and "+
			"these accounts will be saved at $BEATOZHOME/walkeys",
	)
	cmd.Flags().StringVar(
		&privValSecret,
		"priv_validator_secret",
		"",
		"passphrase to encrypt and decrypt a private key in priv_validator_key.json",
	)
}

func initFiles(cmd *cobra.Command, args []string) error {
	var s []byte
	if privValSecret != "" {
		s = []byte(privValSecret)
		privValSecret = ""
	} else {
		s = libs.ReadCredential(fmt.Sprintf("Passphrase for %v: ", filepath.Base(rootConfig.PrivValidatorKeyFile())))
	}
	defer libs.ClearCredential(s)

	return InitFilesWith(beatozChainID, rootConfig, walkeyCnt, s)
}

func InitFilesWith(chainID string, config *cfg.Config, wkcnt int, secret []byte) error {
	// private validator
	privValKeyFile := config.PrivValidatorKeyFile()
	privValStateFile := config.PrivValidatorStateFile()
	var pv *acrypto.SFilePV
	if tmos.FileExists(privValKeyFile) {
		pv = acrypto.LoadSFilePV(privValKeyFile, privValStateFile, secret)
		logger.Info("Found private validator", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
		//pv.SaveWith(secret) // encrypt with new driven key.
	} else {
		pv = acrypto.GenSFilePV(privValKeyFile, privValStateFile)
		pv.SaveWith(secret)
		logger.Info("Generated private validator", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	}

	nodeKeyFile := config.NodeKeyFile()
	if tmos.FileExists(nodeKeyFile) {
		logger.Info("Found beatoz key", "path", nodeKeyFile)
	} else {
		if _, err := p2p.LoadOrGenNodeKey(nodeKeyFile); err != nil {
			return err
		}
		logger.Info("Generated beatoz key", "path", nodeKeyFile)
	}

	// genesis file
	genFile := config.GenesisFile()
	if tmos.FileExists(genFile) {
		logger.Info("Found genesis file", "path", genFile)
	} else {
		var err error
		var genDoc *tmtypes.GenesisDoc
		if chainID == "mainnet" {
			if genDoc, err = genesis.MainnetGenesisDoc(chainID); err != nil {
				return err
			}
		} else if chainID == "testnet" {
			if genDoc, err = genesis.TestnetGenesisDoc(chainID); err != nil {
				return err
			}
		} else if chainID == "devnet" {
			if genDoc, err = genesis.DevnetGenesisDoc(chainID); err != nil {
				return err
			}
		} else { // anything (e.g. loclanet)
			defaultWalkeyDirPath := filepath.Join(config.RootDir, acrypto.DefaultWalletKeyDir)
			err := tmos.EnsureDir(defaultWalkeyDirPath, acrypto.DefaultWalletKeyDirPerm)
			if err != nil {
				return err
			}

			walkeys, err := acrypto.CreateWalletKeyFiles(secret, wkcnt, defaultWalkeyDirPath)
			if err != nil {
				return err
			}

			pvWalKey, err := acrypto.OpenWalletKey(libs.NewFileReader(privValKeyFile))
			if err != nil {
				return err
			}
			_, err = pvWalKey.Save(
				libs.NewFileWriter(
					filepath.Join(defaultWalkeyDirPath, fmt.Sprintf("wk%X.json", pvWalKey.Address))))
			if err != nil {
				return err
			}

			pubKey, err := pv.GetPubKey()
			if err != nil {
				return fmt.Errorf("can't get pubkey: %w", err)
			}
			valset := []tmtypes.GenesisValidator{{
				Address: pubKey.Address(),
				PubKey:  pubKey,
				Power:   types.AmountToPower(types.DefaultGovParams().MinValidatorStake()),
			}}

			// validator is not included to initial holders
			//walkeys = append(walkeys, pvWalKey)

			holders := make([]*genesis.GenesisAssetHolder, len(walkeys))
			for i, wk := range walkeys {
				if err := wk.Unlock(secret); err != nil {
					return err
				}
				holders[i] = &genesis.GenesisAssetHolder{
					Address: wk.Address,
					Balance: uint256.MustFromDecimal("100000000000000000000000000"), // 100_000_000 * 1_000_000_000_000_000_000
				}
			}
			defer func() {
				for _, wk := range walkeys {
					wk.Lock()
				}
			}()

			genDoc, err = genesis.NewGenesisDoc(chainID, valset, holders, types.DefaultGovParams())
			if err != nil {
				return err
			}

		}
		if err := genDoc.SaveAs(genFile); err != nil {
			return err
		}
		logger.Info("Generated genesis file", "path", genFile)
	}

	return nil
}
