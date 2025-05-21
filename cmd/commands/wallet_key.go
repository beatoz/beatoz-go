package commands

import (
	"fmt"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
)

var (
	changePass bool
)

func AddWalletKeyCmdFlag(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(
		&changePass,
		"change-passphrase",
		"c",
		false,
		"Change passphrase of a wallet key file")

}

func NewWalletKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "wallet-key",
		Aliases: []string{"wallet_key"},
		Short:   "Wallet key file management",
		RunE:    handleWalletKey,
		PreRun:  deprecateSnakeCase,
	}

	AddWalletKeyCmdFlag(cmd)

	return cmd
}

func handleWalletKey(cmd *cobra.Command, args []string) error {
	for _, arg := range args {
		if strings.HasPrefix(arg, "~") {
			if home, err := os.UserHomeDir(); err != nil {
				return err
			} else {
				arg = strings.Replace(arg, "~", home, 1)
			}

		}
		fileInfo, err := os.Stat(arg)
		if err != nil {
			return err
		}

		if changePass {
			if err := resetPassphrase(arg); err != nil {
				return err
			}
		} else if fileInfo.IsDir() {
			if err := showWalletKeyDir(arg); err != nil {
				return err
			}
		} else {
			if err := showWalletKeyFile(arg); err != nil {
				return err
			}
		}
	}
	return nil
}

func showWalletKeyDir(path string) error {
	err := filepath.WalkDir(path, func(entry string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			fmt.Println("it is directory", entry)
		} else if err := showWalletKeyFile(entry); err != nil {
			return err
		}
		fmt.Println("---")
		fmt.Println(" ")
		return nil
	})
	return err
}

func showWalletKeyFile(path string) error {

	if wk, err := crypto.OpenWalletKey(libs.NewFileReader(path)); err != nil {
		return err
	} else {
		s := libs.ReadCredential(fmt.Sprintf("Passphrase for %v: ", filepath.Base(path)))
		defer libs.ClearCredential(s)

		if err := wk.Unlock(s); err != nil {
			return err
		}
		defer wk.Lock()

		tmp := &struct {
			*crypto.WalletKey `json:"walletKey"`
			PrvKey            bytes.HexBytes `json."prvKey"`
			PubKey            bytes.HexBytes `json."pubKey"`
		}{
			WalletKey: wk,
			PrvKey:    wk.PrvKey(),
			PubKey:    wk.PubKey(),
		}
		if bz, err := jsonx.MarshalIndent(tmp, "", " "); err != nil {
			return err
		} else {
			fmt.Println(string(bz))
		}
	}
	return nil

}

func resetPassphrase(path string) error {
	wk, err := crypto.OpenWalletKey(libs.NewFileReader(path))
	if err != nil {
		return err
	}

	pass0 := libs.ReadCredential(fmt.Sprintf("Current Passphrase for %v: ", filepath.Base(path)))
	defer bytes.ClearBytes(pass0)
	if err := wk.Unlock(pass0); err != nil {
		return err
	}
	defer wk.Lock()

	pass1 := libs.ReadCredential(fmt.Sprintf("New Passphrase for %v: ", filepath.Base(path)))
	defer bytes.ClearBytes(pass1)
	wk.LockWith(pass1)

	if _, err := wk.Save(libs.NewFileWriter(path)); err != nil {
		return err
	}

	return nil
}
