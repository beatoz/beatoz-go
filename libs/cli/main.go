package main

import (
	"fmt"
	"github.com/beatoz/beatoz-go/libs/cli/cmds"
)

func main() {
	cmds.RootCmd.AddCommand(
		cmds.NewCmd_Account(),
		cmds.NewCmd_Transfer())

	//cmds.PrepareBaseCmd(cmds.RootCmd)
	//executor := cli.PrepareBaseCmd(cmds.RootCmd, "RG", filepath.Join(libs.GetHome(), ".rg"))
	//if err := executor.Execute(); err != nil {
	//	panic(err)
	//}

	if err := cmds.PrepareBaseCmd(cmds.RootCmd); err != nil {
		panic(err)
	}

	if err := cmds.RootCmd.Execute(); err != nil {
		fmt.Println("error:", err)
	}
}
