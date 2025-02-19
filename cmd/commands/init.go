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
	"os"
	"path/filepath"
	"strings"
)

var (
	beatozChainID = "mainnet"
	holderCnt     = 10
	privValCnt    = 1
	blockGasLimit = int64(50_000_000)

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
		holderCnt, // default value is 10
		"the number of holder's account files to be generated.\n"+
			"if you create a new genesis of your own blockchain, "+
			"you need to generate accounts of genesis holders and "+
			"these accounts will be saved at $BEATOZHOME/walkeys directory.\n"+
			"if `--chain_id` is `mainnet`, the holder's accounts is not generated.",
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
	cmd.Flags().Int64Var(
		&blockGasLimit,
		"block_gas_limit",
		blockGasLimit,
		"the maximum gas that can be used in one block.\n"+
			"this value is deterministically adjusted based on the gas usage in the blockchain network.\n"+
			"however, it cannot exceed the `maxBlockGas` of Governance Parameters.",
	)

}

func initFiles(cmd *cobra.Command, args []string) error {
	var s0, s1 []byte

	_secret := os.Getenv("BEATOZ_VALIDATOR_SECRET")
	if _secret == "" {
		s0 = libs.ReadCredential(fmt.Sprintf("Passphrase for %v: ", filepath.Base(rootConfig.PrivValidatorKeyFile())))
	} else {
		s0 = []byte(_secret)
	}

	_secret = os.Getenv("BEATOZ_HOLDER_SECRET")
	if _secret == "" {
		s1 = libs.ReadCredential("Passphrase for initial holder's accounts: ")
	} else {
		s1 = []byte(_secret)
	}

	defer func() {
		libs.ClearCredential(s0)
		libs.ClearCredential(s1)
	}()

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

		_keyFilePath := fmt.Sprintf("%s/%s%d%s", defaultValDirPath, strings.TrimSuffix(filepath.Base(privValKeyFile), filepath.Ext(privValKeyFile)), i, filepath.Ext(privValKeyFile))
		_keyStateFilePath := fmt.Sprintf("%s/%s%d%s", defaultValDirPath, strings.TrimSuffix(filepath.Base(privValStateFile), filepath.Ext(privValStateFile)), i, filepath.Ext(_keyFilePath))

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
		if i == 0 {
			// copy to `privValKeyFile` and `privValStateFile`
			if data, err := os.ReadFile(_keyFilePath); err != nil {
				return err
			} else if err = os.WriteFile(privValKeyFile, data, libs.DefaultSFilePerm); err != nil {
				return err
			}
			if data, err := os.ReadFile(_keyStateFilePath); err != nil {
				return err
			} else if err = os.WriteFile(privValStateFile, data, libs.DefaultSFilePerm); err != nil {
				return err
			}
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

			pow, err := types.AmountToPower(types.DefaultGovParams().MinValidatorStake())
			if err != nil {
				return err
			}
			var valset []tmtypes.GenesisValidator
			for _, pv := range pvs {
				pubKey, err := pv.GetPubKey()
				if err != nil {
					return fmt.Errorf("can't get pubkey: %w", err)
				}
				valset = append(valset, tmtypes.GenesisValidator{
					Address: pubKey.Address(),
					PubKey:  pubKey,
					Power:   pow,
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
			genDoc.ConsensusParams.Block.MaxGas = blockGasLimit
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
