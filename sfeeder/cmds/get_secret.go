package cmds

import (
	"fmt"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/sfeeder/common"
	"github.com/spf13/cobra"
)

var targetAddr []byte

func NewCmd_GetSecret() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Aliases: []string{"read", "show"},
		Short:   "get secret",
		RunE:    getSecret,
	}
	addGetSecretFlags(cmd)
	return cmd
}

func addGetSecretFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BytesHexVarP(&targetAddr, "addr", "a", targetAddr, "-")
}

func getSecret(cmd *cobra.Command, args []string) error {
	defer func() {
		libs.ClearCredential([]byte(pass))
	}()

	secret, err := common.ReadSecret(targetAddr, []byte(pass), DataDir)
	defer libs.ClearCredential(secret)

	if err != nil {
		return err
	}

	l := len(secret)
	_secret := make([]byte, l)
	copy(_secret, secret)
	if l <= 8 {
		n := libs.MIN(2, l)
		_secret = append(_secret[:n], []byte("..")...)
	} else if l <= 16 {
		s0 := _secret[:2]
		s1 := _secret[l-2:]
		_secret = []byte(fmt.Sprintf("%s..%s", s0, s1))
	} else {
		s0 := _secret[:4]
		s1 := _secret[l-4:]
		_secret = []byte(fmt.Sprintf("%s..%s", s0, s1))
	}

	fmt.Println("Secret:", string(_secret))
	return nil
}
