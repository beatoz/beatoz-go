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
	"strings"
)

//// InitFilesCmd initialises a fresh Tendermint Core instance.
//var InitFilesCmd = &cobra.Command{
//	Use:   "init",
//	Short: "Initialize a node",
//	RunE:  initFiles,
//}

var (
	beatozChainID           = "mainnet"
	holderCnt               = 10
	holderSecret            string
	privValCnt              = 1
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
		&holderCnt,
		"holders",
		holderCnt, // default value is 9
		"the number of holder's account files to be generated.\n"+
			"if you create a new genesis of your own blockchain, "+
			"you need to generate accounts of genesis holders and "+
			"these accounts will be saved at $BEATOZHOME/walkeys directory.\n"+
			"if `--chain_id` is `mainnet`, the holder's accounts is not generated.",
	)
	cmd.Flags().StringVar(
		&holderSecret,
		"holder_secret",
		"",
		"passphrase to encrypt and decrypt a private key in account files of initial holders.\n"+
			"these files are created in $BEATOZHOME/walkeys and have file names starting with 'wk'.",
	)
	cmd.Flags().IntVar(
		&privValCnt,
		"priv_validator_cnt",
		privValCnt, // default value is 1
		"the number of validators on BEATOZ network created.\n"+
			"if you create a new genesis of your own blockchain, "+
			"you need to generate validator's accounts and\n"+
			"the first validator's account file (wallet key file) is created as $BEATOZHOME/config/priv_validator_key.json.\n"+
			"if there is more than one validator, the rest will be created in the $BEATOZHOME/walkeys/vals directory.",
	)
	cmd.Flags().StringVar(
		&privValSecret,
		"priv_validator_secret",
		"",
		"passphrase to encrypt and decrypt $BEATOZHOME/config/priv_validator_key.json.",
	)
}

func initFiles(cmd *cobra.Command, args []string) error {
	var s0, s1 []byte
	if privValSecret != "" {
		s0 = []byte(privValSecret)
		privValSecret = ""
	} else {
		s0 = libs.ReadCredential(fmt.Sprintf("Passphrase for %v: ", filepath.Base(rootConfig.PrivValidatorKeyFile())))
	}
	if holderSecret != "" {
		s1 = []byte(holderSecret)
		holderSecret = ""
	} else {
		s1 = libs.ReadCredential("Passphrase for initial holder's accounts: ")
	}
	defer libs.ClearCredential(s0)

	return InitFilesWith(beatozChainID, rootConfig, privValCnt, s0, holderCnt, s1)
}

func InitFilesWith(chainID string, config *cfg.Config, vcnt int, vsecret []byte, hcnt int, hsecret []byte) error {
	// private validator
	privValKeyFile := config.PrivValidatorKeyFile()
	privValStateFile := config.PrivValidatorStateFile()

	defaultValDirPath := filepath.Join(config.RootDir, acrypto.DefaultValKeyDir)
	err := tmos.EnsureDir(defaultValDirPath, acrypto.DefaultWalletKeyDirPerm)
	if err != nil {
		return err
	}

	var pvs []*acrypto.SFilePV
	for i := 0; i < vcnt; i++ {
		var pv *acrypto.SFilePV

		_keyFilePath := privValKeyFile
		_keyStateFilePath := privValStateFile
		if i > 0 {
			_keyFilePath = fmt.Sprintf("%s/%s%d%s", defaultValDirPath, strings.TrimSuffix(filepath.Base(_keyFilePath), filepath.Ext(_keyFilePath)), i, filepath.Ext(_keyFilePath))
			_keyStateFilePath = fmt.Sprintf("%s/%s%d%s", defaultValDirPath, strings.TrimSuffix(filepath.Base(_keyStateFilePath), filepath.Ext(_keyStateFilePath)), i, filepath.Ext(_keyFilePath))
		}
		if tmos.FileExists(_keyFilePath) {
			pv = acrypto.LoadSFilePV(_keyFilePath, _keyStateFilePath, vsecret)
			logger.Info("Found private validator", "keyFile", _keyFilePath,
				"stateFile", _keyStateFilePath)
			//pv.SaveWith(secret) // encrypt with new driven key.
		} else {
			pv = acrypto.GenSFilePV(_keyFilePath, _keyStateFilePath)
			pv.SaveWith(vsecret)
			logger.Info("Generated private validator", "keyFile", _keyFilePath,
				"stateFile", _keyStateFilePath)
		}
		pvs = append(pvs, pv)
	}

	nodeKeyFile := config.NodeKeyFile()
	if tmos.FileExists(nodeKeyFile) {
		logger.Info("Found beatoz node key", "path", nodeKeyFile)
	} else {
		if _, err := p2p.LoadOrGenNodeKey(nodeKeyFile); err != nil {
			return err
		}
		logger.Info("Generated beatoz node key", "path", nodeKeyFile)
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
		} else { // anything (e.g. loclanet)
			defaultWalkeyDirPath := filepath.Join(config.RootDir, acrypto.DefaultWalletKeyDir)
			err := tmos.EnsureDir(defaultWalkeyDirPath, acrypto.DefaultWalletKeyDirPerm)
			if err != nil {
				return err
			}

			walkeys, err := acrypto.CreateWalletKeyFiles(hsecret, hcnt, defaultWalkeyDirPath)
			if err != nil {
				return err
			}
			logger.Info("Generated initial holder's wallet key files", "path", defaultWalkeyDirPath)

			//pvWalKey, err := acrypto.OpenWalletKey(libs.NewFileReader(privValKeyFile))
			//if err != nil {
			//	return err
			//}
			//_, err = pvWalKey.Save(
			//	libs.NewFileWriter(
			//		filepath.Join(defaultWalkeyDirPath, fmt.Sprintf("wk%X.json", pvWalKey.Address))))
			//if err != nil {
			//	return err
			//}

			var valset []tmtypes.GenesisValidator
			for _, pv := range pvs {
				pubKey, err := pv.GetPubKey()
				if err != nil {
					return fmt.Errorf("can't get pubkey: %w", err)
				}
				valset = append(valset, tmtypes.GenesisValidator{
					Address: pubKey.Address(),
					PubKey:  pubKey,
					Power:   types.AmountToPower(types.DefaultGovParams().MinValidatorStake()),
				})
			}

			holders := make([]*genesis.GenesisAssetHolder, len(walkeys))
			for i, wk := range walkeys {
				holders[i] = &genesis.GenesisAssetHolder{
					Address: wk.Address,
					Balance: uint256.MustFromDecimal("100000000000000000000000000"), // 100_000_000 * 1_000_000_000_000_000_000
				}
			}

			logger.Info("Generate GenesisAssetHolder")

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
