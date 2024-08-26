package cmds

import (
	"errors"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/libs/web3"
	"github.com/holiman/uint256"
	"github.com/spf13/cobra"
)

func NewCmd_Transfer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "transfer BEATOZ coin",
		RunE:  transfer,
	}
	return cmd
}

func transfer(cmd *cobra.Command, args []string) error {
	bzweb3 := web3.NewBeatozWeb3(web3.NewHttpProvider(rootFlags.RPCUrl))

	// get sender's wallet key
	if rootFlags.From == "" {
		return errors.New("please set the wallet key file of sender")
	}
	w, err := web3.OpenWallet(libs.NewFileReader(rootFlags.From))
	if err != nil {
		return err
	}

	w.SyncAccount(bzweb3)
	w.Unlock([]byte("1111"))

	ret, err := w.TransferCommit(rootFlags.To, rootFlags.Gas, uint256.MustFromDecimal(rootFlags.GasPrice), uint256.MustFromDecimal(rootFlags.Amount), bzweb3)
	if err != nil {
		return err
	}
	if ret.CheckTx.Code != 0 {
		return errors.New(ret.CheckTx.Log)
	}
	if ret.DeliverTx.Code != 0 {
		return errors.New(ret.DeliverTx.Log)
	}

	return nil
}
