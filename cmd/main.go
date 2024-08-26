package main

import (
	"github.com/beatoz/beatoz-go/cmd/commands"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/node"
	"github.com/tendermint/tendermint/libs/cli"
	"path/filepath"
)

func main() {
	commands.RootCmd.AddCommand(
		commands.NewInitFilesCmd(),
		commands.ResetPrivValidatorCmd,
		commands.ResetAllCmd,
		commands.NewRunNodeCmd(node.NewBeatozNode),
		commands.ShowNodeIDCmd,
		commands.NewWalletKeyCmd(),
		commands.VersionCmd,
	)

	executor := cli.PrepareBaseCmd(commands.RootCmd, "BEATOZ", filepath.Join(libs.GetHome(), ".beatoz"))
	if err := executor.Execute(); err != nil {
		panic(err)
	}
}
