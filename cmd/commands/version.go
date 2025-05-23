package commands

import (
	"fmt"
	"github.com/beatoz/beatoz-go/cmd/version"
	"github.com/spf13/cobra"
)

// VersionCmd ...
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version info",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.String())
	},
}
