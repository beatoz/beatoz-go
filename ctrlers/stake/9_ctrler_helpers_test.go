package stake_test

import (
	"bytes"
	"errors"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/web3"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"math/rand"
)

func randMakeStakingToSelfTrxContext() (*ctrlertypes.TrxContext, error) {
	from := acctMock00.RandWallet()
	to := from

	power := ctrlertypes.AmountToPower(govParams00.MinValidatorStake()) + rand.Int63n(10000)

	if txCtx, err := makeStakingTrxContext(from, to, power); err != nil {
		return nil, err
	} else {
		DelegateeWallets = append(DelegateeWallets, to)
		return txCtx, nil
	}

}

func randMakeStakingTrxContext() (*ctrlertypes.TrxContext, error) {
	for {
		from, to := acctMock00.RandWallet(), DelegateeWallets[rand.Intn(len(DelegateeWallets))]
		if bytes.Compare(from.Address(), to.Address()) == 0 {
			continue
		}
		power := rand.Int63n(1000) + 10
		return makeStakingTrxContext(from, to, power)
	}
}

func makeStakingTrxContext(from, to *web3.Wallet, power int64) (*ctrlertypes.TrxContext, error) {
	amt := ctrlertypes.PowerToAmount(power)

	tx := web3.NewTrxStaking(from.Address(), to.Address(), dummyNonce, dummyGas, dummyGasPrice, amt)
	bz, err := tx.Encode()
	if err != nil {
		return nil, err
	}

	return &ctrlertypes.TrxContext{
		Exec:         true,
		Tx:           tx,
		TxHash:       crypto.DefaultHash(bz),
		Height:       lastHeight,
		SenderPubKey: from.GetPubKey(),
		Sender:       from.GetAccount(),
		Receiver:     to.GetAccount(),
		GasUsed:      0,
		GovHandler:   govParams00,
		AcctHandler:  acctMock00,
	}, nil
}

func findStakingTxCtx(txhash bytes2.HexBytes) *ctrlertypes.TrxContext {
	for _, tctx := range stakingTrxCtxs {
		if bytes.Compare(tctx.TxHash, txhash) == 0 {
			return tctx
		}
	}
	return nil
}

func randMakeUnstakingTrxContext() (*ctrlertypes.TrxContext, error) {
	rn := rand.Intn(len(stakingTrxCtxs))
	stakingTxCtx := stakingTrxCtxs[rn]

	from := acctMock00.FindWallet(stakingTxCtx.Tx.From)
	if from == nil {
		return nil, errors.New("not found test account for " + stakingTxCtx.Tx.From.String())
	}
	to := acctMock00.FindWallet(stakingTxCtx.Tx.To)
	if to == nil {
		return nil, errors.New("not found test account for " + stakingTxCtx.Tx.To.String())
	}

	return makeUnstakingTrxContext(from, to, stakingTxCtx.TxHash)
}

func makeUnstakingTrxContext(from, to *web3.Wallet, txhash bytes2.HexBytes) (*ctrlertypes.TrxContext, error) {

	tx := web3.NewTrxUnstaking(from.Address(), to.Address(), dummyNonce, dummyGas, dummyGasPrice, txhash)
	tzbz, _, err := from.SignTrxRLP(tx, "stake_test_chain")
	if err != nil {
		return nil, err
	}

	return &ctrlertypes.TrxContext{
		Exec:         true,
		Tx:           tx,
		TxHash:       crypto.DefaultHash(tzbz),
		Height:       lastHeight,
		SenderPubKey: from.GetPubKey(),
		Sender:       from.GetAccount(),
		Receiver:     to.GetAccount(),
		GovHandler:   govParams00,
	}, nil
}
