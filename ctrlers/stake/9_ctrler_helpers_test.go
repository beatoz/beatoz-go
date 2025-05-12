package stake_test

import (
	"bytes"
	"errors"
	"github.com/beatoz/beatoz-go/ctrlers/mocks"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/tendermint/tendermint/abci/types"
	types3 "github.com/tendermint/tendermint/proto/tendermint/types"
	"math/rand"
	"time"
)

func randMakeStakingToSelfTrxContext(txHeight int64) (*ctrlertypes.TrxContext, error) {
	from := acctMock00.RandWallet()
	to := from

	power := govMock00.MinValidatorPower() + rand.Int63n(10000)
	if txCtx, err := makeStakingTrxContext(from, to, power, txHeight); err != nil {
		return nil, err
	} else {
		DelegateeWallets = append(DelegateeWallets, to)
		return txCtx, nil
	}

}

func randMakeStakingTrxContext(txHeight int64) (*ctrlertypes.TrxContext, error) {
	for {
		from, to := acctMock00.RandWallet(), DelegateeWallets[rand.Intn(len(DelegateeWallets))]
		if bytes.Compare(from.Address(), to.Address()) == 0 {
			continue
		}
		power := rand.Int63n(1000) + 10
		return makeStakingTrxContext(from, to, power, txHeight)
	}
}

func makeStakingTrxContext(from, to *web3.Wallet, power, txHeight int64) (*ctrlertypes.TrxContext, error) {
	amt := ctrlertypes.PowerToAmount(power)

	tx := web3.NewTrxStaking(from.Address(), to.Address(), dummyNonce, defGas, defGasPrice, amt)
	bz, err := tx.Encode()
	if err != nil {
		return nil, err
	}

	return &ctrlertypes.TrxContext{
		Exec:         true,
		Tx:           tx,
		TxHash:       crypto.DefaultHash(bz),
		SenderPubKey: from.GetPubKey(),
		Sender:       from.GetAccount(),
		Receiver:     to.GetAccount(),
		GasUsed:      0,
		BlockContext: ctrlertypes.NewBlockContext(
			types.RequestBeginBlock{
				Header: types3.Header{Height: txHeight},
			},
			govMock00, acctMock00, nil, nil, nil,
		),
	}, nil
	//_, _, err := from.SignTrxRLP(tx, "stake_test_chain")
	//if err != nil {
	//	return nil, err
	//}
	//return mocks.MakeTrxCtxWithTrx(tx, "", txHeight, time.Now(), true, govMock00, acctMock00, nil, nil, nil)
}

func findStakingTxCtx(txhash bytes2.HexBytes) *ctrlertypes.TrxContext {
	for _, tctx := range stakingTrxCtxs {
		if bytes.Compare(tctx.TxHash, txhash) == 0 {
			return tctx
		}
	}
	return nil
}

func randMakeUnstakingTrxContext(txHeight int64) (*ctrlertypes.TrxContext, error) {
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

	return makeUnstakingTrxContext(from, to, stakingTxCtx.TxHash, txHeight)
}

func makeUnstakingTrxContext(from, to *web3.Wallet, txhash bytes2.HexBytes, txHeight int64) (*ctrlertypes.TrxContext, error) {

	tx := web3.NewTrxUnstaking(from.Address(), to.Address(), dummyNonce, defGas, defGasPrice, txhash)
	//tzbz, _, err := from.SignTrxRLP(tx, "stake_test_chain")
	//if err != nil {
	//	return nil, err
	//}
	//
	//return &ctrlertypes.TrxContext{
	//	Exec:         true,
	//	Tx:           tx,
	//	TxHash:       crypto.DefaultHash(tzbz),
	//	Height:       txHeight,
	//	SenderPubKey: from.GetPubKey(),
	//	Sender:       from.GetAccount(),
	//	Receiver:     to.GetAccount(),
	//	GovParams:    govParams00,
	//}, nil

	//return &ctrlertypes.TrxContext{
	//	Exec:         true,
	//	Tx:           tx,
	//	TxHash:       crypto.DefaultHash(tzbz),
	//	SenderPubKey: from.GetPubKey(),
	//	Sender:       from.GetAccount(),
	//	Receiver:     to.GetAccount(),
	//	BlockContext: ctrlertypes.NewBlockContext(
	//		types.RequestBeginBlock{
	//			Header: types3.Header{Height: txHeight},
	//		},
	//		govMock00, nil, nil, nil, nil,
	//	),
	//}, nil

	_, _, err := from.SignTrxRLP(tx, "stake_test_chain")
	if err != nil {
		return nil, err
	}
	return mocks.MakeTrxCtxWithTrx(tx, "stake_test_chain", txHeight, time.Now(), true, govMock00, acctMock00, nil, nil, nil)
}
