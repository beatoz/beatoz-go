package cmds

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

func NewCmd_RemoveAll() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"clear"},
		Short:   "remove all secrets",
		RunE:    removeAll,
	}

	addRemoveAllFlags(cmd)
	return cmd
}

func addRemoveAllFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&DataDir, "data", "d", DataDir, "-")
}

func removeAll(cmd *cobra.Command, args []string) error {
	if err := os.RemoveAll(DataDir); err != nil {
		return err
	}
	fmt.Println("Remove all secrets in", DataDir)
	return nil
}
