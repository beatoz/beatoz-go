package cmds

import (
	"github.com/beatoz/beatoz-go/libs"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

const (
	dirPerm = 0o700
)

var (
	pass    = ""
	DataDir string
)

func NewCmd_Base() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "sfeeder",
		Short:        "Secret feeder for BEATOZ validator nodes",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
			if pass == "" {
				pass = string(libs.ReadCredential("Enter passphrase of sfeeder: "))
			}
			if DataDir == "" {
				if h, err := os.UserHomeDir(); err != nil {
					return err
				} else {
					DataDir = filepath.Join(h, ".sfeeder")
				}
			}

			return nil
		},
	}
	cmd.AddCommand(
		NewCmd_SetSecret(),
		NewCmd_GetSecret(),
		NewCmd_Start(),
		NewCmd_RemoveAll())

	if err := prepareBaseCmd(cmd); err != nil {
		panic(err)
	}
	return cmd
}

// PrepareBaseCmd is meant for tendermint and other servers
func prepareBaseCmd(cmd *cobra.Command) error {
	cmd.PersistentFlags().StringVarP(&pass, "pass", "p", "", "-")
	cmd.PersistentFlags().StringVarP(&DataDir, "data", "d", DataDir, "-")
	return nil
}
