package commands

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/spf13/cobra"
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

func parseWalletKeyFile(path string, hideSecret ...bool) (*crypto.WalletKey, error) {
	_hideSecret := false
	if len(hideSecret) > 0 {
		_hideSecret = hideSecret[0]
	}

	if wk, err := crypto.OpenWalletKey(libs.NewFileReader(path)); err != nil {
		return nil, err
	} else if !_hideSecret {
		s := libs.ReadCredential(fmt.Sprintf("Passphrase for %v: ", filepath.Base(path)))
		defer libs.ClearCredential(s)

		if err := wk.Unlock(s); err != nil {
			return nil, err
		}
		return wk, nil
	} else {
		return wk, nil
	}
}

func showWalletKeyFile(path string, hideSecret ...bool) error {
	var addr, prvKey, pubKey string

	if wk, err := parseWalletKeyFile(path, hideSecret...); err != nil {
		return err
	} else {
		defer wk.Lock()

		addr = wk.Address.String()
		if !wk.IsLock() {
			pubKey = hex.EncodeToString(wk.PubKey())
			prvKey = hex.EncodeToString(wk.PrvKey())
		}
	}

	fmt.Println("wallet file :", path)
	fmt.Println("address     :", addr)
	if pubKey != "" {
		fmt.Println("public key  :", pubKey)
	}
	if prvKey != "" {
		fmt.Println("private key :", prvKey)
	}
	return nil
}

func _showWalletKeyFile(path string, hideSecret ...bool) error {
	var addr, prvKey, pubKey string

	_hideSecret := false
	if len(hideSecret) > 0 {
		_hideSecret = hideSecret[0]
	}
	if wk, err := crypto.OpenWalletKey(libs.NewFileReader(path)); err != nil {
		return err
	} else if _hideSecret {
		addr = wk.Address.String()
		pubKey = hex.EncodeToString(wk.PubKey())
	} else {
		s := libs.ReadCredential(fmt.Sprintf("Passphrase for %v: ", filepath.Base(path)))
		defer libs.ClearCredential(s)

		if err := wk.Unlock(s); err != nil {
			return err
		}
		defer wk.Lock()

		addr = wk.Address.String()
		pubKey = hex.EncodeToString(wk.PubKey())
		prvKey = hex.EncodeToString(wk.PrvKey())
	}

	fmt.Println("wallet file :", path)
	fmt.Println("address     :", addr)
	if !_hideSecret {
		fmt.Println("public key  :", pubKey)
		fmt.Println("private key :", prvKey)
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
