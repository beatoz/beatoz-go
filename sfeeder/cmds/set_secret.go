package cmds

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/sfeeder/common"
	"github.com/spf13/cobra"
	"os"
)

var (
	hexAddrs []string
)

func NewCmd_SetSecret() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "set",
		Aliases: []string{"new", "add"},
		Short:   "add secret",
		RunE:    setSecret,
	}
	addSetSecretFlags(cmd)
	return cmd
}

func addSetSecretFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringSliceVarP(&hexAddrs, "addrs", "a", hexAddrs, "-")
}

func setSecret(cmd *cobra.Command, args []string) error {
	defer func() {
		libs.ClearCredential([]byte(pass))
	}()

	if err := ensureDir(DataDir, dirPerm); err != nil {
		return err
	}

	for _, hxaddr := range hexAddrs {
		addr, err := hex.DecodeString(hxaddr)
		if err != nil {
			return err
		}
		if _, err := common.WriteSecret(addr, nil, []byte(pass), DataDir); err != nil {
			return err
		}
	}

	for {
		hxaddr, err := readFromShell("Enter address: ")
		addr, err := hex.DecodeString(string(hxaddr))
		if err != nil {
			return err
		}
		if _, err := common.WriteSecret(addr, nil, []byte(pass), DataDir); err != nil {
			return err
		}
	}
}

func ensureDir(dir string, mode os.FileMode) error {
	err := os.MkdirAll(dir, mode)
	if err != nil {
		return fmt.Errorf("could not create directory %q: %w", dir, err)
	}
	return nil
}

func readFromShell(prompt string) ([]byte, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("ERROR:", err.Error())
		return nil, err
	}
	return bytes.TrimSpace([]byte(text)), nil
}
