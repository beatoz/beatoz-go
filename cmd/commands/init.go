package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/cmd/version"
	"github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/genesis"
	"github.com/beatoz/beatoz-go/libs"
	btztypes "github.com/beatoz/beatoz-go/types"
	acrypto "github.com/beatoz/beatoz-go/types/crypto"
	"github.com/spf13/cobra"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/p2p"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

var (
	initParams = DefaultInitParams()
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
		&initParams.ChainID,
		"chain_id",
		initParams.ChainID, // default name
		"the id of chain to generate (e.g. mainnet, testnet, devnet and others)")
	cmd.Flags().IntVar(
		&initParams.HolderCnt,
		"holders",
		initParams.HolderCnt, // default value is 10
		"the number of holder's account files to be generated.\n"+
			"if you create a new genesis of your own blockchain, "+
			"you need to generate accounts of genesis holders and "+
			"these accounts will be saved at $BEATOZHOME/walkeys directory.\n"+
			"if `--chain_id` is `mainnet`, the holder's accounts is not generated.",
	)
	cmd.Flags().IntVar(
		&initParams.ValCnt,
		"priv_validator_cnt",
		initParams.ValCnt, // default value is 1
		"the number of validators on BEATOZ network created.\n"+
			"if you create a new genesis of your own blockchain, "+
			"you need to generate validator's accounts and\n"+
			"the first validator's account file (wallet key file) is created as $BEATOZHOME/config/priv_validator_key.json.\n"+
			"if there is more than one validator, the rest will be created in the $BEATOZHOME/walkeys/vals directory.",
	)
	cmd.Flags().Int64Var(
		&initParams.BlockGasLimit,
		"block_gas_limit",
		initParams.BlockGasLimit,
		"the maximum gas that can be used in one block.\n"+
			"this value is deterministically adjusted based on the gas usage in the blockchain network.\n"+
			"however, it cannot exceed the `maxBlockGas` of Governance Parameters.",
	)
	cmd.Flags().StringVar(
		&initParams.AssumedBlockInterval,
		"assumed_block_interval",
		initParams.AssumedBlockInterval,
		"assumed time between blocks in seconds, used for estimating time from block count.\n"+
			"It is not based on actual block production timing.\n"+
			"Instead, it is used as a constant reference to estimate time from the number of blocks produced.")
	cmd.Flags().Int64Var(
		&initParams.MaxTotalSupply,
		"max_total_supply",
		initParams.MaxTotalSupply,
		"upper limit of total supply; "+
			"total supply will never exceed this value.",
	)
	cmd.Flags().Int64Var(
		&initParams.InitTotalSupply,
		"init_total_supply",
		initParams.InitTotalSupply,
		"initial total supply at genesis, shared equally by all holders; it includes the initial voting power",
	)
	cmd.Flags().Int64Var(
		&initParams.InitVotingPower,
		"init_voting_power",
		initParams.InitVotingPower,
		"initial voting power at genesis, shared equally by all validators",
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

	initParams.ValSecret = s0
	initParams.HolderSecret = s1

	return InitFilesWith(rootConfig, initParams)
}

func InitFilesWith(
	config *cfg.Config,
	params *InitParams,
	callbacks ...func(*tmtypes.GenesisDoc),
) error {
	if err := params.Validate(); err != nil {
		return err
	}
	// private validator
	privValKeyFile := config.PrivValidatorKeyFile()
	privValStateFile := config.PrivValidatorStateFile()

	defaultValDirPath := filepath.Join(config.RootDir, acrypto.DefaultValKeyDir)
	err := tmos.EnsureDir(defaultValDirPath, acrypto.DefaultWalletKeyDirPerm)
	if err != nil {
		return err
	}

	var pvs []*acrypto.SFilePV
	for i := 0; i < params.ValCnt; i++ {
		var pv *acrypto.SFilePV

		_keyFilePath := fmt.Sprintf("%s/%s%d%s", defaultValDirPath, strings.TrimSuffix(filepath.Base(privValKeyFile), filepath.Ext(privValKeyFile)), i, filepath.Ext(privValKeyFile))
		_keyStateFilePath := fmt.Sprintf("%s/%s%d%s", defaultValDirPath, strings.TrimSuffix(filepath.Base(privValStateFile), filepath.Ext(privValStateFile)), i, filepath.Ext(_keyFilePath))

		if tmos.FileExists(_keyFilePath) {
			pv = acrypto.LoadSFilePV(_keyFilePath, _keyStateFilePath, params.ValSecret)
			logger.Info("Found private validator", "keyFile", _keyFilePath,
				"stateFile", _keyStateFilePath)
		} else {
			pv = acrypto.GenSFilePV(_keyFilePath, _keyStateFilePath)
			pv.SaveWith(params.ValSecret)
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
		if params.ChainID == "mainnet" {
			if genDoc, err = genesis.MainnetGenesisDoc(params.ChainID); err != nil {
				return err
			}
		} else if params.ChainID == "testnet" {
			if genDoc, err = genesis.TestnetGenesisDoc(params.ChainID); err != nil {
				return err
			}
		} else { // anything (e.g. loclanet)
			//
			// Initialize consensus parameters
			consensusParams := &tmproto.ConsensusParams{
				Block:    tmtypes.DefaultBlockParams(),
				Evidence: tmtypes.DefaultEvidenceParams(),
				Validator: tmproto.ValidatorParams{
					PubKeyTypes: []string{tmtypes.ABCIPubKeyTypeSecp256k1},
				},
				Version: tmproto.VersionParams{
					AppVersion: version.Major(),
				},
			}
			consensusParams.Block.MaxGas = params.BlockGasLimit

			defaultWalkeyDirPath := filepath.Join(config.RootDir, acrypto.DefaultWalletKeyDir)
			if err = tmos.EnsureDir(defaultWalkeyDirPath, acrypto.DefaultWalletKeyDirPerm); err != nil {
				return err
			}
			//
			// Initialize validators at genesis
			pow := params.InitVotingPower / int64(len(pvs))
			rpow := params.InitVotingPower % int64(len(pvs))
			var valset []tmtypes.GenesisValidator
			for i, pv := range pvs {
				if i == len(pvs)-1 {
					pow += rpow
				}
				pubKey, err := pv.GetPubKey()
				if err != nil {
					return fmt.Errorf("can't get pubkey: %w", err)
				}
				valset = append(valset, tmtypes.GenesisValidator{
					Address: pubKey.Address(),
					PubKey:  pubKey,
					Power:   pow,
				})
				logger.Info("GenesisValidator", "address", pubKey.Address(), "power", pow)
			}

			//
			// Initialize asset holders at genesis
			walkeys, err := acrypto.CreateWalletKeyFiles(params.HolderSecret, params.HolderCnt, defaultWalkeyDirPath)
			if err != nil {
				return err
			}
			logger.Info("Generated initial holder's wallet key files", "path", defaultWalkeyDirPath)

			realInitSupply := params.InitTotalSupply - params.InitVotingPower
			amt := realInitSupply / int64(len(walkeys))
			ramt := realInitSupply % int64(len(walkeys))
			holders := make([]*genesis.GenesisAssetHolder, len(walkeys))
			for i, wk := range walkeys {
				if i == len(walkeys)-1 {
					amt += ramt
				}
				holders[i] = &genesis.GenesisAssetHolder{
					Address: wk.Address,
					Balance: btztypes.ToGrans(amt), // amt * 10^18
				}
			}
			logger.Debug("GenesisAssetHolder", "holders count", len(holders))

			//
			// Create Governance Parameters
			blockInterval, err := time.ParseDuration(params.AssumedBlockInterval)
			if err != nil {
				return err
			}
			govParams := types.NewGovParams(int(blockInterval.Seconds()))
			govParams.GetValues().XMaxTotalSupply = btztypes.ToGrans(params.MaxTotalSupply).Bytes()

			appState := &genesis.GenesisAppState{
				AssetHolders: holders,
				GovParams:    govParams,
			}

			//
			// Create genesis
			genDoc, err = genesis.NewGenesisDoc(
				params.ChainID,
				consensusParams,
				valset,
				appState,
			)
			if err != nil {
				return err
			}
		}
		for _, cb := range callbacks {
			cb(genDoc)
		}

		if err := genDoc.SaveAs(genFile); err != nil {
			return err
		}
		logger.Info("Generated genesis file", "path", genFile)
	}

	return nil
}

type InitParams struct {
	ChainID              string
	ValCnt               int
	ValSecret            []byte
	HolderCnt            int
	HolderSecret         []byte
	BlockGasLimit        int64
	AssumedBlockInterval string
	MaxTotalSupply       int64
	InitTotalSupply      int64
	InitVotingPower      int64
}

func DefaultInitParams() *InitParams {
	return &InitParams{
		ChainID:              "0x0001",
		ValCnt:               1,
		HolderCnt:            10,
		BlockGasLimit:        int64(36_000_000),
		AssumedBlockInterval: "10s",
		MaxTotalSupply:       int64(700_000_000),
		InitTotalSupply:      int64(350_000_000),
		InitVotingPower:      int64(35_000_000),
	}
}

func (params *InitParams) Validate() error {
	if !btztypes.IsHexByteString(params.ChainID) &&
		!btztypes.IsNumericString(params.ChainID) {
		return fmt.Errorf("invalid chain_id: %s", params.ChainID)
	}

	if params.InitTotalSupply > params.MaxTotalSupply {
		return fmt.Errorf("init_total_supply (%d) cannot exceed max_total_supply (%d)", params.InitTotalSupply, params.MaxTotalSupply)
	}
	if params.InitVotingPower > params.InitTotalSupply {
		return fmt.Errorf("init_voting_power (%d) cannot exceed init_total_supply (%d)", params.InitVotingPower, params.InitTotalSupply)
	}
	return nil
}
