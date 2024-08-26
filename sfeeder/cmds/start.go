package cmds

import (
	"github.com/beatoz/beatoz-go/sfeeder/server"
	"github.com/spf13/cobra"
)

var (
	listenAddr = "127.0.0.1:9900"
)

func NewCmd_Start() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start",
		Aliases: []string{"run"},
		Short:   "start sfeeder server",
		RunE:    start,
	}
	addStartFlags(cmd)
	return cmd
}

func addStartFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&listenAddr, "laddr", "l", listenAddr, "-")
}

func start(cmd *cobra.Command, args []string) error {
	if err := ensureDir(DataDir, dirPerm); err != nil {
		return err
	}
	server.Start(listenAddr, DataDir, func() []byte { return []byte(pass) })
	return nil
}
