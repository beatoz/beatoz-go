package commands

import (
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/cli"
	tmos "github.com/tendermint/tendermint/libs/os"
)

var (
	defaultRoot = os.ExpandEnv("$HOME/.some/test/dir")
)

// clearConfig clears env vars, the given root dir, and resets viper.
func clearConfig(dir string) {
	if err := os.Unsetenv("BEATOZHOME"); err != nil {
		panic(err)
	}
	if err := os.Unsetenv("BEATOZ_HOME"); err != nil {
		panic(err)
	}

	if err := os.RemoveAll(dir); err != nil {
		panic(err)
	}
	viper.Reset()
	rootConfig = cfg.DefaultConfig()
}

// prepare new rootCmd
func testRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               RootCmd.Use,
		PersistentPreRunE: RootCmd.PersistentPreRunE,
		Run:               func(cmd *cobra.Command, args []string) {},
	}
	registerFlagsRootCmd(rootCmd)
	var l string
	rootCmd.PersistentFlags().String("log", l, "Log")
	return rootCmd
}

func testSetup(rootDir string, args []string, env map[string]string) error {
	clearConfig(defaultRoot)

	rootCmd := testRootCmd()
	cmd := cli.PrepareBaseCmd(rootCmd, "BEATOZ", defaultRoot)

	// run with the args and env
	args = append([]string{rootCmd.Use}, args...)
	return cli.RunWithArgs(cmd, args, env)
}

func TestRootHome(t *testing.T) {
	newRoot := filepath.Join(defaultRoot, "something-else")
	cases := []struct {
		args []string
		env  map[string]string
		root string
	}{
		{nil, nil, defaultRoot},
		{[]string{"--home", newRoot}, nil, newRoot},
		{nil, map[string]string{"BEATOZHOME": newRoot}, newRoot},
	}

	for i, tc := range cases {
		idxString := strconv.Itoa(i)

		err := testSetup(defaultRoot, tc.args, tc.env)
		require.Nil(t, err, idxString)

		assert.Equal(t, tc.root, rootConfig.RootDir, idxString)
		assert.Equal(t, tc.root, rootConfig.P2P.RootDir, idxString)
		assert.Equal(t, tc.root, rootConfig.Consensus.RootDir, idxString)
		assert.Equal(t, tc.root, rootConfig.Mempool.RootDir, idxString)
	}
}

func TestRootFlagsEnv(t *testing.T) {

	// defaults
	defaults := cfg.DefaultConfig()
	defaultLogLvl := defaults.LogLevel

	cases := []struct {
		args     []string
		env      map[string]string
		logLevel string
	}{
		{[]string{"--log", "debug"}, nil, defaultLogLvl},                      // wrong flag
		{[]string{"--log_level", "debug"}, nil, "debug"},                      // right flag
		{nil, map[string]string{"BEATOZ_LOW": "debug"}, defaultLogLvl},        // wrong env flag
		{nil, map[string]string{"_BEATOZ_LOG_LEVEL": "debug"}, defaultLogLvl}, // wrong env prefix
		{nil, map[string]string{"BEATOZ_LOG_LEVEL": "debug"}, "debug"},        // right env
	}

	for i, tc := range cases {
		idxString := strconv.Itoa(i)

		err := testSetup(defaultRoot, tc.args, tc.env)
		require.Nil(t, err, idxString)

		assert.Equal(t, tc.logLevel, rootConfig.LogLevel, idxString)
	}
}

func TestRootConfig(t *testing.T) {

	// write non-default config
	nonDefaultLogLvl := "abc:debug"
	cvals := map[string]string{
		"log_level": nonDefaultLogLvl,
	}

	cases := []struct {
		args []string
		env  map[string]string

		logLvl string
	}{
		{nil, nil, nonDefaultLogLvl},                                         // should load config
		{[]string{"--log_level=abc:info"}, nil, "abc:info"},                  // flag over rides
		{nil, map[string]string{"BEATOZ_LOG_LEVEL": "abc:info"}, "abc:info"}, // env over rides
	}

	for i, tc := range cases {
		idxString := strconv.Itoa(i)
		clearConfig(defaultRoot)

		// XXX: path must match cfg.defaultConfigPath
		configFilePath := filepath.Join(defaultRoot, "config")
		err := tmos.EnsureDir(configFilePath, 0700)
		require.Nil(t, err)

		// write the non-defaults to a different path
		err = WriteConfigVals(configFilePath, cvals)
		require.Nil(t, err)

		rootCmd := testRootCmd()
		cmd := cli.PrepareBaseCmd(rootCmd, "BEATOZ", defaultRoot)

		// run with the args and env
		tc.args = append([]string{rootCmd.Use}, tc.args...)
		err = cli.RunWithArgs(cmd, tc.args, tc.env)
		require.Nil(t, err, idxString)

		assert.Equal(t, tc.logLvl, rootConfig.LogLevel, idxString)
	}
}

// WriteConfigVals writes a toml file with the given values.
// It returns an error if writing was impossible.
func WriteConfigVals(dir string, vals map[string]string) error {
	data := ""
	for k, v := range vals {
		data += fmt.Sprintf("%s = \"%s\"\n", k, v)
	}
	cfile := filepath.Join(dir, "config.toml")
	return ioutil.WriteFile(cfile, []byte(data), 0600)
}
