package types

import (
	"errors"

	"github.com/beatoz/beatoz-go/types"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/merkle"
	"github.com/beatoz/beatoz-go/types/xerrors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

type TrxContext struct {
	*BlockContext

	Tx     *Trx
	TxIdx  int
	TxHash bytes2.HexBytes
	Exec   bool

	SenderPubKey []byte
	Sender       *Account
	Receiver     *Account
	Payer        *Account
	GasUsed      int64
	RetData      []byte
	Events       []abcitypes.Event

	ValidateResult interface{}
	Callback       func(*TrxContext, xerrors.XError)
}

type NewTrxContextCb func(*TrxContext) xerrors.XError

func NewTrxContext(txbz []byte, bctx *BlockContext, exec bool) (*TrxContext, xerrors.XError) {
	tx := &Trx{}
	if xerr := tx.Decode(txbz); xerr != nil {
		return nil, xerr
	}
	if xerr := tx.Validate(); xerr != nil {
		return nil, xerr
	}

	txctx := &TrxContext{
		BlockContext: bctx,
		Tx:           tx,
		TxIdx:        bctx.TxsCnt(),
		TxHash:       tmtypes.Tx(txbz).Hash(),
		Exec:         exec,
		GasUsed:      0,
	}

	//
	// validation gas.
	if tx.Gas < txctx.BlockContext.GovHandler.MinTrxGas() {
		return nil, xerrors.ErrInvalidGas.Wrapf("the tx has too small gas (min: %v)", txctx.GovHandler.MinTrxGas())
	}

	if tx.GasPrice.Cmp(txctx.BlockContext.GovHandler.GasPrice()) != 0 {
		return nil, xerrors.ErrInvalidGasPrice
	}

	//
	// verify signature.
	_, pubKeyBytes, xerr := VerifyTrxRLP(tx)
	if xerr != nil {
		return nil, xerr
	}
	txctx.SenderPubKey = pubKeyBytes

	//
	// verify payer's signature.
	var payerAddr types.Address
	if tx.PayerSig != nil {
		payerAddr, _, xerr = VerifyPayerTrxRLP(tx)
		if xerr != nil {
			return nil, xerr.Wrap(errors.New("payer signature is invalid"))
		}
	}

	//
	//

	txctx.Sender = txctx.BlockContext.AcctHandler.FindAccount(tx.From, txctx.Exec)
	if txctx.Sender == nil {
		return nil, xerrors.ErrNotFoundAccount.Wrapf("sender address: %v", tx.From)
	}

	// RG-91: Find the account object with the destination address 0x0.
	toAddr := txctx.Tx.To
	if toAddr == nil {
		// `toAddr` may be `nil` when the tx type is `TRX_CONTRACT`.
		toAddr = types.ZeroAddress()
	}

	txctx.Receiver = txctx.BlockContext.AcctHandler.FindOrNewAccount(toAddr, txctx.Exec)
	if txctx.Receiver == nil {
		return nil, xerrors.ErrNotFoundAccount.Wrapf("receiver address: %v", toAddr)
	}

	if payerAddr != nil {
		txctx.Payer = txctx.BlockContext.AcctHandler.FindAccount(payerAddr, txctx.Exec)
		if txctx.Payer == nil {
			return nil, xerrors.ErrNotFoundAccount.Wrapf("payer address: %v", payerAddr)
		}
	} else {
		txctx.Payer = txctx.Sender
	}

	return txctx, nil
}

func (ctx *TrxContext) IsHandledByEVM() bool {
	b := ctx.Tx.GetType() == TRX_CONTRACT || (ctx.Tx.GetType() == TRX_TRANSFER && ctx.Receiver.Code != nil)
	return b
}

func (ctx *TrxContext) EventRoot() (*merkle.MerkleTree, []byte) {
	if types.IsBud(ctx.ChainID(), ctx.Height()) {
		return ctx.eventRootEx()
	}
	return ctx.eventRoot()
}

// eventRoot returns the merkle tree instance and root hash of the event log.
// The merkle tree's leaf are constructed as follows:
//
//	[event type + attribute key + attribute value]
func (ctx *TrxContext) eventRoot() (*merkle.MerkleTree, []byte) {
	if len(ctx.Events) == 0 {
		return nil, nil
	}

	var leaves [][]byte
	for _, evt := range ctx.Events {
		ety := evt.Type
		for _, attr := range evt.Attributes {
			leaves = append(leaves, append(append([]byte(ety), attr.Key...), attr.Value...))
		}
	}

	tree := merkle.NewMerkleTree(merkle.WithRawLeaves(leaves))
	return tree, tree.Root()
}

// eventRootEx returns the merkle tree instance and root hash of the event log
// using a two-level merkle tree construction.
//
// Unlike [eventRoot], which flattens all event attributes into a single merkle tree,
// eventRootEx builds the tree in two stages:
//
//  1. For each event, a merkle tree is constructed from its attributes, where each
//     leaf is formed as [event type + attribute key + attribute value]. The root hash
//     of this per-event tree is used as a summary of that event.
//
//  2. The per-event root hashes (already hashed) are then used as leaves of a second
//     merkle tree. The root of this top-level tree is the final merkle root.
//
// This two-level approach preserves event boundaries, allowing efficient proof
// generation and verification at the individual event level.
func (ctx *TrxContext) eventRootEx() (*merkle.MerkleTree, []byte) {
	if len(ctx.Events) == 0 {
		return nil, nil
	}

	// 1. Each Event -> merkle tree -> root
	roots := make([][]byte, len(ctx.Events))
	for i, evt := range ctx.Events {
		leaves := make([][]byte, len(evt.Attributes))
		ety := evt.Type
		for j, attr := range evt.Attributes {
			leaves[j] = append(append([]byte(ety), attr.Key...), attr.Value...)
		}
		t := merkle.NewMerkleTree(merkle.WithRawLeaves(leaves))
		roots[i] = t.Root()
	}

	// 2. Roots -> merkle tree -> final root (roots are already hashed)
	tree := merkle.NewMerkleTree(merkle.WithHashedLeaves(roots))
	return tree, tree.Root()
}

// EventRoot is used only for testing to avoid cyclic import.
func EventRoot(ctx *TrxContext) (*merkle.MerkleTree, []byte) {
	return ctx.eventRoot()
}

// EventRootEx is used only for testing to avoid cyclic import.
func EventRootEx(ctx *TrxContext) (*merkle.MerkleTree, []byte) {
	return ctx.eventRootEx()
}
