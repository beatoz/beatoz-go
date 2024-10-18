package web3

import (
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/holiman/uint256"
	tmsecp256k1 "github.com/tendermint/tendermint/crypto/secp256k1"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"io"
	"sync"
)

type Wallet struct {
	wkey *crypto.WalletKey
	acct *ctrlertypes.Account

	mtx sync.RWMutex
}

func NewWallet(s []byte) *Wallet {
	prvKey := tmsecp256k1.GenPrivKey()
	wkey := crypto.NewWalletKeyWith(prvKey, s)
	return &Wallet{
		wkey: wkey,
		acct: ctrlertypes.NewAccount(wkey.Address),
	}
}

func ImportKey(prvKey, s []byte) *Wallet {
	wkey := crypto.NewWalletKeyWith(prvKey, s)
	return &Wallet{
		wkey: wkey,
		acct: ctrlertypes.NewAccount(wkey.Address),
	}
}

func OpenWallet(r io.Reader) (*Wallet, error) {
	wk, err := crypto.OpenWalletKey(r)
	if err != nil {
		return nil, err
	}

	ret := &Wallet{
		wkey: wk,
	}

	//if err := ret.syncAccount(); err != nil {
	//	return nil, err
	//}
	return ret, nil
}

func (w *Wallet) Save(wr io.Writer) error {
	w.mtx.RLock()
	defer w.mtx.RUnlock()

	_, err := w.wkey.Save(wr)
	return err
}

func (w *Wallet) Clone() *Wallet {
	w.mtx.RLock()
	defer w.mtx.RUnlock()

	return &Wallet{
		wkey: w.wkey,
		acct: w.acct.Clone(),
	}
}

func (w *Wallet) Address() types.Address {
	w.mtx.RLock()
	defer w.mtx.RUnlock()

	return w.wkey.Address
}

func (w *Wallet) GetAccount() *ctrlertypes.Account {
	w.mtx.RLock()
	defer w.mtx.RUnlock()

	return w.acct
}

func (w *Wallet) AddNonce() {
	w.mtx.Lock()
	defer w.mtx.Unlock()

	w.addNonce()
}

func (w *Wallet) addNonce() {
	w.acct.AddNonce()
}

func (w *Wallet) GetNonce() uint64 {
	w.mtx.RLock()
	defer w.mtx.RUnlock()

	return w.acct.GetNonce()
}

func (w *Wallet) GetBalance() *uint256.Int {
	w.mtx.RLock()
	defer w.mtx.RUnlock()

	if w.acct == nil {
		return uint256.NewInt(0)
	} else {
		return w.acct.GetBalance()
	}
}

func (w *Wallet) Lock() {
	w.mtx.Lock()
	defer w.mtx.Unlock()

	w.wkey.Lock()
}

func (w *Wallet) GetPubKey() bytes.HexBytes {
	w.mtx.RLock()
	defer w.mtx.RUnlock()

	return w.wkey.PubKey()
}

func (w *Wallet) Unlock(s []byte) error {
	w.mtx.Lock()
	defer w.mtx.Unlock()

	return w.wkey.Unlock(s)
}

func (w *Wallet) SignTrxProto(tx *ctrlertypes.Trx, chainId string) (bytes.HexBytes, bytes.HexBytes, error) {
	w.mtx.RLock()
	defer w.mtx.RUnlock()

	preimg, xerr := ctrlertypes.PreImageToSignTrxProto(tx, chainId)
	if xerr != nil {
		return nil, nil, xerr
	}

	sig, err := w.wkey.Sign(preimg)
	if err != nil {
		return nil, nil, err
	}

	tx.Sig = sig
	return sig, preimg, nil
}

func (w *Wallet) SignTrxRLP(tx *ctrlertypes.Trx, chainId string) (bytes.HexBytes, bytes.HexBytes, error) {
	w.mtx.RLock()
	defer w.mtx.RUnlock()

	preimg, xerr := ctrlertypes.PreImageToSignTrxRLP(tx, chainId)
	if xerr != nil {
		return nil, nil, xerr
	}

	sig, err := w.wkey.Sign(preimg)
	if err != nil {
		return nil, nil, err
	}

	tx.Sig = sig
	return sig, preimg, nil
}

func (w *Wallet) SendTxAsync(tx *ctrlertypes.Trx, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTx, error) {
	if _, _, err := w.SignTrxRLP(tx, bzweb3.ChainID()); err != nil {
		return nil, err
	} else {
		return bzweb3.SendTransactionAsync(tx)
	}
}

func (w *Wallet) SendTxSync(tx *ctrlertypes.Trx, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTx, error) {
	if _, _, err := w.SignTrxRLP(tx, bzweb3.ChainID()); err != nil {
		return nil, err
	} else {
		return bzweb3.SendTransactionSync(tx)
	}
}

func (w *Wallet) SendTxCommit(tx *ctrlertypes.Trx, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTxCommit, error) {
	if _, _, err := w.SignTrxRLP(tx, bzweb3.ChainID()); err != nil {
		return nil, err
	} else {
		return bzweb3.SendTransactionCommit(tx)
	}
}

func (w *Wallet) SetDocSync(name, url string, gas uint64, gasPrice *uint256.Int, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTx, error) {
	tx := NewTrxSetDoc(w.Address(), w.acct.GetNonce(), gas, gasPrice, name, url)
	if _, _, err := w.SignTrxRLP(tx, bzweb3.ChainID()); err != nil {
		return nil, err
	} else {
		return bzweb3.SendTransactionSync(tx)
	}
}

func (w *Wallet) SetDocCommit(name, url string, gas uint64, gasPrice *uint256.Int, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTxCommit, error) {
	tx := NewTrxSetDoc(w.Address(), w.acct.GetNonce(), gas, gasPrice, name, url)
	if _, _, err := w.SignTrxRLP(tx, bzweb3.ChainID()); err != nil {
		return nil, err
	} else {
		return bzweb3.SendTransactionCommit(tx)
	}
}

func (w *Wallet) TransferAsync(to types.Address, gas uint64, gasPrice, amt *uint256.Int, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTx, error) {
	tx := NewTrxTransfer(
		w.Address(), to,
		w.acct.GetNonce(),
		gas, gasPrice, amt,
	)
	return w.SendTxAsync(tx, bzweb3)
}

func (w *Wallet) TransferSync(to types.Address, gas uint64, gasPrice, amt *uint256.Int, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTx, error) {
	tx := NewTrxTransfer(
		w.Address(), to,
		w.acct.GetNonce(),
		gas, gasPrice, amt,
	)
	return w.SendTxSync(tx, bzweb3)
}

func (w *Wallet) TransferCommit(to types.Address, gas uint64, gasPrice, amt *uint256.Int, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTxCommit, error) {
	tx := NewTrxTransfer(
		w.Address(), to,
		w.acct.GetNonce(),
		gas, gasPrice, amt,
	)
	return w.SendTxCommit(tx, bzweb3)
}

func (w *Wallet) StakingAsync(to types.Address, gas uint64, gasPrice, amt *uint256.Int, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTx, error) {
	tx := NewTrxStaking(
		w.Address(), to,
		w.acct.GetNonce(),
		gas, gasPrice, amt,
	)
	return w.SendTxAsync(tx, bzweb3)
}

func (w *Wallet) StakingSync(to types.Address, gas uint64, gasPrice, amt *uint256.Int, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTx, error) {
	tx := NewTrxStaking(
		w.Address(), to,
		w.acct.GetNonce(),
		gas, gasPrice, amt,
	)
	return w.SendTxSync(tx, bzweb3)
}

func (w *Wallet) StakingCommit(to types.Address, gas uint64, gasPrice, amt *uint256.Int, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTxCommit, error) {
	tx := NewTrxStaking(
		w.Address(), to,
		w.acct.GetNonce(),
		gas, gasPrice, amt,
	)
	return w.SendTxCommit(tx, bzweb3)
}

func (w *Wallet) WithdrawAync(gas uint64, gasPrice, req *uint256.Int, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTx, error) {
	tx := NewTrxWithdraw(w.Address(), w.Address(), w.acct.GetNonce(), gas, gasPrice, req)
	return w.SendTxAsync(tx, bzweb3)
}

func (w *Wallet) WithdrawSync(gas uint64, gasPrice, req *uint256.Int, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTx, error) {
	tx := NewTrxWithdraw(w.Address(), w.Address(), w.acct.GetNonce(), gas, gasPrice, req)
	return w.SendTxSync(tx, bzweb3)
}

func (w *Wallet) WithdrawCommit(gas uint64, gasPrice, req *uint256.Int, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTxCommit, error) {
	tx := NewTrxWithdraw(w.Address(), w.Address(), w.acct.GetNonce(), gas, gasPrice, req)
	return w.SendTxCommit(tx, bzweb3)
}

func (w *Wallet) ProposalSync(gas uint64, gasPrice *uint256.Int, msg string, start, period, applyingHeight int64, optType int32, options []byte, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTx, error) {
	tx := NewTrxProposal(
		w.Address(),
		types.ZeroAddress(),
		w.acct.GetNonce(),
		gas, gasPrice, msg, start, period, applyingHeight, optType, options,
	)
	if _, _, err := w.SignTrxRLP(tx, bzweb3.ChainID()); err != nil {
		return nil, err
	} else {
		return bzweb3.SendTransactionSync(tx)
	}
}

func (w *Wallet) ProposalCommit(gas uint64, gasPrice *uint256.Int, msg string, start, period, applyingHeight int64, optType int32, options []byte, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTxCommit, error) {
	tx := NewTrxProposal(
		w.Address(),
		types.ZeroAddress(),
		w.acct.GetNonce(),
		gas, gasPrice, msg, start, period, applyingHeight, optType, options,
	)
	if _, _, err := w.SignTrxRLP(tx, bzweb3.ChainID()); err != nil {
		return nil, err
	} else {
		return bzweb3.SendTransactionCommit(tx)
	}
}

func (w *Wallet) VotingSync(gas uint64, gasPrice *uint256.Int, txHash bytes.HexBytes, choice int32, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTx, error) {
	tx := NewTrxVoting(
		w.Address(),
		types.ZeroAddress(),
		w.acct.GetNonce(),
		gas, gasPrice, txHash, choice,
	)
	if _, _, err := w.SignTrxRLP(tx, bzweb3.ChainID()); err != nil {
		return nil, err
	} else {
		return bzweb3.SendTransactionSync(tx)
	}
}

func (w *Wallet) VotingCommit(gas uint64, gasPrice *uint256.Int, txHash bytes.HexBytes, choice int32, bzweb3 *BeatozWeb3) (*coretypes.ResultBroadcastTxCommit, error) {
	tx := NewTrxVoting(
		w.Address(),
		types.ZeroAddress(),
		w.acct.GetNonce(),
		gas, gasPrice, txHash, choice,
	)
	if _, _, err := w.SignTrxRLP(tx, bzweb3.ChainID()); err != nil {
		return nil, err
	} else {
		return bzweb3.SendTransactionCommit(tx)
	}
}

func (w *Wallet) syncAccount(bzweb3 *BeatozWeb3) error {
	if acct, err := bzweb3.GetAccount(w.wkey.Address); err != nil {
		return err
	} else {
		w.acct = acct
	}
	return nil
}

func (w *Wallet) SyncAccount(bzweb3 *BeatozWeb3) error {
	w.mtx.Lock()
	defer w.mtx.Unlock()

	return w.syncAccount(bzweb3)
}

func (w *Wallet) syncNonce(bzweb3 *BeatozWeb3) error {
	return w.syncAccount(bzweb3)
}

func (w *Wallet) SyncNonce(bzweb3 *BeatozWeb3) error {
	w.mtx.Lock()
	defer w.mtx.Unlock()

	return w.syncNonce(bzweb3)
}

func (w *Wallet) syncBalance(bzweb3 *BeatozWeb3) error {
	return w.syncAccount(bzweb3)
}

func (w *Wallet) SyncBalance(bzweb3 *BeatozWeb3) error {
	w.mtx.Lock()
	defer w.mtx.Unlock()

	return w.syncBalance(bzweb3)
}
