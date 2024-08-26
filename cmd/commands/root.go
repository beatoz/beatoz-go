package commands

import (
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	tmcfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/cli"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

var (
	rootConfig = cfg.DefaultConfig()
	logger     = tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout))
)

func init() {
	registerFlagsRootCmd(RootCmd)
}

func registerFlagsRootCmd(cmd *cobra.Command) {
	cmd.PersistentFlags().String("log_level", rootConfig.LogLevel, "log level")
}

// ParseConfig retrieves the default environment configuration,
// sets up the Tendermint root and ensures that the root exists
func ParseConfig() (*cfg.Config, error) {
	conf := tmcfg.DefaultConfig()
	err := viper.Unmarshal(conf)
	if err != nil {
		return nil, err
	}
	conf.SetRoot(conf.RootDir)
	tmcfg.EnsureRoot(conf.RootDir)
	if err := conf.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("error in rootConfig file: %v", err)
	}
	return &cfg.Config{conf, ""}, nil
}

// RootCmd is the root command for Tendermint core.
var RootCmd = &cobra.Command{
	Use:   "beatoz",
	Short: "BFT state machine replication for applications in any programming languages",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if cmd.Name() == VersionCmd.Name() {
			return nil
		}

		rootConfig, err = ParseConfig()
		if err != nil {
			return err
		}

		if rootConfig.LogFormat == tmcfg.LogFormatJSON {
			logger = tmlog.NewTMJSONLogger(tmlog.NewSyncWriter(os.Stdout))
		}

		logger, err = tmflags.ParseLogLevel(rootConfig.LogLevel, logger, tmcfg.DefaultLogLevel)
		if err != nil {
			return err
		}

		if viper.GetBool(cli.TraceFlag) {
			logger = tmlog.NewTracingLogger(logger)
		}

		logger = logger.With("module", "main")
		return nil
	},
}

// deprecateSnakeCase is a util function for 0.34.1. Should be removed in 0.35
func deprecateSnakeCase(cmd *cobra.Command, args []string) {
	if strings.Contains(cmd.CalledAs(), "_") {
		fmt.Println("Deprecated: snake_case commands will be replaced by hyphen-case commands in the next major release")
	}
}
