package commands

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/beatoz/beatoz-go/cmd/commands/web3"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/holiman/uint256"
	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/libs/json"
	tmtypes "github.com/tendermint/tendermint/types"
)

var (
	rpcUrl       string
	showPriv     bool
	bondAmt      string
	unbondTxHash string

	wkLocal   *crypto.WalletKey
	bzweb3    *web3.BeatozWeb3
	signer    ctrlertypes.ISigner
	govParams *ctrlertypes.GovParams
)

func withValidatorSetup(fn func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		prvKeyFile := rootConfig.PrivValidatorKeyFile()
		wk, err := parseWalletKeyFile(prvKeyFile, !showPriv)
		if err != nil {
			return err
		}
		wkLocal = wk

		bzweb3 = web3.NewBeatozWeb3(web3.NewHttpProvider(rpcUrl))

		signer = ctrlertypes.NewSignerV1(bzweb3.ChainIDInt())

		gp, err := bzweb3.QueryGovParams()
		if err != nil {
			return err
		}
		govParams = gp

		return fn(cmd, args)
	}
}

func NewValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validator",
		Short: "Validator management command",
	}

	cmd.PersistentFlags().StringVar(
		&rpcUrl,
		"rpcurl",
		"http://localhost:26657",
		"BEATOZ RPC URL")

	cmd.AddCommand(
		newShowCmd(),
		newBondCmd(),
		newUnbondCmd(),
	)

	return cmd
}

func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show validator key information",
		RunE:  withValidatorSetup(handleShowCmd),
	}

	cmd.Flags().BoolVar(
		&showPriv,
		"priv",
		false,
		"Show private key",
	)

	return cmd
}

func newBondCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bond",
		Short: "Bond amount as voting power",
		RunE:  withValidatorSetup(handleBondCmd),
	}
	cmd.Flags().StringVar(
		&bondAmt,
		"amount",
		"",
		"Amount to bond as voting power (decimal number, converted to uint256)")
	_ = cmd.MarkFlagRequired("amount")
	return cmd
}

func newUnbondCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unbond",
		Short: "Unbond voting power back to balance",
		RunE:  withValidatorSetup(handleUnbondCmd),
	}
	cmd.Flags().StringVar(
		&unbondTxHash,
		"txhash",
		"",
		"Transaction hash of the staking transaction to unbond")
	_ = cmd.MarkFlagRequired("txhash")
	return cmd
}

func handleShowCmd(cmd *cobra.Command, args []string) error {
	fmt.Printf("Local validator : %v\n", wkLocal.Address)
	retValidators, err := bzweb3.QueryValidators(0, 1, 100)
	if err != nil {
		return err
	}
	for _, val := range retValidators.Validators {
		if bytes.Equal(wkLocal.Address, val.Address) {
			stakes, err := bzweb3.QueryStakes(wkLocal.Address)
			if err != nil {
				return err
			}
			rwd, err := bzweb3.QueryReward(wkLocal.Address, 0)
			if err != nil {
				return err
			}

			result := struct {
				Addresss    tmtypes.Address        `json:"address"`
				VotingPower int64                  `json:"voting_power"`
				Stakes      []*web3.RespQueryStake `json:"stakes"`
				Reward      *web3.RespQueryReward  `json:"reward"`
			}{
				Addresss:    val.Address,
				VotingPower: val.VotingPower,
				Stakes:      stakes,
				Reward:      rwd,
			}

			jz, err := json.MarshalIndent(&result, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(jz))
			return nil
		}
	}

	return fmt.Errorf("not found validator(%v) in the network (chainId:%v).\n", wkLocal.Address, bzweb3.ChainID())
}

func handleBondCmd(cmd *cobra.Command, args []string) error {
	amt := new(uint256.Int)
	if err := amt.SetFromDecimal(bondAmt); err != nil {
		return err
	}

	localAcct, err := bzweb3.QueryAccount(wkLocal.Address)
	if err != nil {
		return err
	}
	if amt.Cmp(localAcct.Balance) > 0 {
		return fmt.Errorf("insufficient balance")
	}

	if wkLocal.IsLock() {
		s := libs.ReadCredential(fmt.Sprintf("Passphrase for %v: ", wkLocal.Address))
		defer libs.ClearCredential(s)

		if err := wkLocal.Unlock(s); err != nil {
			return err
		}
		defer wkLocal.Lock()
	}
	tx := web3.NewTrxStaking(
		wkLocal.Address,
		wkLocal.Address,
		localAcct.GetNonce(),
		govParams.MinTrxGas(),
		govParams.GasPrice(),
		amt,
	)

	sig, err := signer.SignSender(tx, wkLocal.PrvKey())
	if err != nil {
		return err
	}
	tx.Sig = sig

	retCommit, err := bzweb3.SendTransactionCommit(tx)
	if err != nil {
		return err
	}
	if retCommit.CheckTx.Code != 0 {
		return fmt.Errorf("check tx failed: %v", retCommit.CheckTx.Log)
	}
	if retCommit.DeliverTx.Code != 0 {
		return fmt.Errorf("deliver tx failed: %v", retCommit.DeliverTx.Log)
	}
	fmt.Printf("tx hash: %v\n", retCommit.Hash)

	return nil
}

func handleUnbondCmd(cmd *cobra.Command, args []string) error {
	txHash, err := hex.DecodeString(strings.TrimPrefix(unbondTxHash, "0x"))
	if err != nil {
		return fmt.Errorf("invalid txhash: %w", err)
	}

	localAcct, err := bzweb3.QueryAccount(wkLocal.Address)
	if err != nil {
		return err
	}

	if wkLocal.IsLock() {
		s := libs.ReadCredential(fmt.Sprintf("Passphrase for %v: ", wkLocal.Address))
		defer libs.ClearCredential(s)

		if err := wkLocal.Unlock(s); err != nil {
			return err
		}
		defer wkLocal.Lock()
	}

	tx := web3.NewTrxUnstaking(
		wkLocal.Address,
		wkLocal.Address,
		localAcct.GetNonce(),
		govParams.MinTrxGas(),
		govParams.GasPrice(),
		txHash,
	)

	sig, err := signer.SignSender(tx, wkLocal.PrvKey())
	if err != nil {
		return err
	}
	tx.Sig = sig

	retCommit, err := bzweb3.SendTransactionCommit(tx)
	if err != nil {
		return err
	}
	if retCommit.CheckTx.Code != 0 {
		return fmt.Errorf("check tx failed: %v", retCommit.CheckTx.Log)
	}
	if retCommit.DeliverTx.Code != 0 {
		return fmt.Errorf("deliver tx failed: %v", retCommit.DeliverTx.Log)
	}
	fmt.Printf("tx hash: %v\n", retCommit.Hash)

	return nil
}
