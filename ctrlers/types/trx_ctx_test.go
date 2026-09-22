package types_test

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/beatoz/beatoz-go/ctrlers/mocks/acct"
	"github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/merkle"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

var (
	chainId  = uint256.MustFromHex("0xabc")
	govMock  = gov.NewGovHandlerMock(ctrlertypes.DefaultGovParams())
	acctMock = acct.NewAcctHandlerMock(1000)
)

func init() {
	acctMock.Iterate(func(idx int, w *web3.Wallet) bool {
		w.GetAccount().SetBalance(uint256.NewInt(1_000_000_000))
		return true
	})
}

func Test_NewTrxContext(t *testing.T) {
	w0 := acctMock.RandWallet() //web3.NewWallet(nil)
	w1 := web3.NewWallet(nil)

	//
	// Small Gas
	tx := web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas()-1, govMock.GasPrice(), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr := newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGas.Error())

	//
	// 0 GasPrice
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), uint256.NewInt(0), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGasPrice.Error())

	//
	// negative GasPrice
	var b [32]byte
	b[0] = 0x80
	neg := uint256.NewInt(0).SetBytes32(b[:])
	require.Negative(t, neg.Sign())
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), neg, uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGasPrice.Error())

	//
	// too much GasPrice
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), uint256.NewInt(10_000_000_001), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidGasPrice.Error())

	//
	// Wrong Signature - no signature
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(0))
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidTrxSig.Error())

	//
	// Wrong Signature - other's signature
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(0))
	_, _, _ = w1.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidTrxSig.Error())

	//
	// Wrong Signature - wrong chainId
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(0))
	_, _, _ = w0.SignTrxRLP(tx, "tx_executor_test_chain_wrong")
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidTrxSig.Error())

	//
	// To nil address (not contract transaction)
	// todo: move this case to trx_test.go
	tx = web3.NewTrxTransfer(w0.Address(), nil, 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.ErrorContains(t, xerr, xerrors.ErrInvalidAddress.Error())

	//
	// To nil address (contract transaction)
	tx = web3.NewTrxContract(w0.Address(), nil, 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(0), bytes.RandBytes(32))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
	require.NotNil(t, txctx.Sender())
	require.Equal(t, txctx.Sender().Address, w0.Address())
	require.NotNil(t, txctx.Receiver())
	require.Equal(t, txctx.Receiver().Address, types.ZeroAddress())
	require.NotNil(t, txctx.Payer())
	require.Equal(t, txctx.Payer().Address, w0.Address())
	//
	// To Zero Address
	tx = web3.NewTrxTransfer(w0.Address(), types.ZeroAddress(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
	require.NotNil(t, txctx.Sender())
	require.Equal(t, txctx.Sender().Address, w0.Address())
	require.NotNil(t, txctx.Receiver())
	require.Equal(t, txctx.Receiver().Address, types.ZeroAddress())
	require.NotNil(t, txctx.Payer())
	require.Equal(t, txctx.Payer().Address, w0.Address())

	//
	// Success
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(1000))
	_, _, _ = w0.SignTrxRLP(tx, chainId.Hex())
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
	require.NotNil(t, txctx.Sender())
	require.Equal(t, txctx.Sender().Address, w0.Address())
	require.NotNil(t, txctx.Receiver())
	require.Equal(t, txctx.Receiver().Address, w1.Address())
	require.NotNil(t, txctx.Payer())
	require.EqualValues(t, txctx.Sender().Address, txctx.Payer().Address)

	//
	// Payer: not Sender
	payer := acctMock.RandWallet()
	tx = web3.NewTrxTransfer(w0.Address(), w1.Address(), 0, govMock.MinTrxGas(), govMock.GasPrice(), uint256.NewInt(1000))
	_, _, err := w0.SignTrxRLP(tx, chainId.Hex())
	require.NoError(t, err)
	_, _, err = payer.SignPayerTrxRLP(tx, chainId.Hex())
	require.NoError(t, err)
	txctx, xerr = newTrxCtx(tx, 1)
	require.NoError(t, xerr)
	require.NotNil(t, txctx.Payer())
	require.Equal(t, payer.Address(), txctx.Payer().Address)
}

func newTrxCtx(tx *ctrlertypes.Trx, height int64) (*ctrlertypes.TrxContext, xerrors.XError) {
	bctx := ctrlertypes.TempBlockContext(chainId.Hex(), height, time.Now(), govMock, acctMock, nil, nil, nil)
	bz, _ := tx.Encode()
	return ctrlertypes.NewTrxContext(bz, bctx, true)
}

//
// test code for `EventRoot()` and `EventRootEx()`
//

func Test_TrxContext_EventRoot(t *testing.T) {

	txctx := &ctrlertypes.TrxContext{}
	txctx.Events = append(txctx.Events, abcitypes.Event{
		Type: "tx",
		Attributes: []abcitypes.EventAttribute{
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXTYPE), Value: []byte("TestTransfer"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXSENDER), Value: []byte("123456"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXRECVER), Value: []byte("abcdef"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_AMOUNT), Value: []byte("1111"), Index: false},
		},
	})
	tree, root := ctrlertypes.EventRoot(txctx)
	require.NotNil(t, tree)
	require.NotNil(t, root)

	targetIdx := 1
	targetData := append([]byte("tx"), append([]byte(ctrlertypes.EVENT_ATTR_TXSENDER), []byte("123456")...)...)

	_, siblings, err := tree.Proof(targetIdx)
	require.NoError(t, err)

	err = merkle.VerifyProof(targetIdx, targetData, siblings, root)
	require.NoError(t, err)

	// wrong index
	err = merkle.VerifyProof(targetIdx+1, targetData, siblings, root)
	require.Error(t, err)

	// wrong data
	err = merkle.VerifyProof(targetIdx, []byte("wrong data"), siblings, root)
	require.Error(t, err)

	// other siblings
	_, otherSiblings, err := tree.Proof(targetIdx + 1)
	require.NoError(t, err)
	err = merkle.VerifyProof(targetIdx, targetData, otherSiblings, root)
	require.Error(t, err)
}

func Test_TrxContext_EventRootEx(t *testing.T) {
	// empty events
	txctx := &ctrlertypes.TrxContext{}
	tree, root := ctrlertypes.EventRootEx(txctx)
	require.Nil(t, tree)
	require.Nil(t, root)

	// two events
	txctx.Events = append(txctx.Events, abcitypes.Event{
		Type: "tx",
		Attributes: []abcitypes.EventAttribute{
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXTYPE), Value: []byte("TestTransfer"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXSENDER), Value: []byte("sender1"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXRECVER), Value: []byte("recver1"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_AMOUNT), Value: []byte("1000"), Index: false},
		},
	})
	txctx.Events = append(txctx.Events, abcitypes.Event{
		Type: "tx",
		Attributes: []abcitypes.EventAttribute{
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXTYPE), Value: []byte("TestTransfer"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXSENDER), Value: []byte("sender2"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_TXRECVER), Value: []byte("recver2"), Index: true},
			{Key: []byte(ctrlertypes.EVENT_ATTR_AMOUNT), Value: []byte("2000"), Index: false},
		},
	})

	tree, root = ctrlertypes.EventRootEx(txctx)
	require.NotNil(t, tree)
	require.NotNil(t, root)

	// Verify proof for the first event root (index 0) in the top-level tree.
	// The top-level tree leaves are already hashed, so use preHashed=true for VerifyProof.
	leafHash, siblings, err := tree.Proof(0)
	require.NoError(t, err)
	err = merkle.VerifyProof(0, leafHash, siblings, root, true)
	require.NoError(t, err)

	// Verify proof for the second event root (index 1)
	leafHash, siblings, err = tree.Proof(1)
	require.NoError(t, err)
	err = merkle.VerifyProof(1, leafHash, siblings, root, true)
	require.NoError(t, err)

	// wrong data should fail
	err = merkle.VerifyProof(0, []byte("wrong"), siblings, root, true)
	require.Error(t, err)

	// Verify that per-event merkle root matches the leaf in the top-level tree.
	// Reconstruct the first event's merkle root manually.
	evt0 := txctx.Events[0]
	var leaves [][]byte
	for _, attr := range evt0.Attributes {
		leaves = append(leaves, attr.Value)
	}
	evt0Tree := merkle.NewMerkleTree(merkle.WithRawLeaves(leaves))
	evt0Root := evt0Tree.Root()

	leaf0, _, err := tree.Proof(0)
	require.NoError(t, err)
	require.Equal(t, evt0Root, leaf0)

	// Verify a specific attribute in the per-event merkle tree.
	// Events[0].Attributes[1] (TXSENDER, "sender1")
	targetIdx := 1
	targetData := []byte("sender1")
	_, evt0Siblings, err := evt0Tree.Proof(targetIdx)
	require.NoError(t, err)
	err = merkle.VerifyProof(targetIdx, targetData, evt0Siblings, evt0Root)
	require.NoError(t, err)

	// wrong attribute data should fail
	err = merkle.VerifyProof(targetIdx, []byte("wrong"), evt0Siblings, evt0Root)
	require.Error(t, err)

	// Events[1].Attributes[2] (TXRECVER, "recver2")
	evt1 := txctx.Events[1]
	var leaves1 [][]byte
	for _, attr := range evt1.Attributes {
		leaves1 = append(leaves1, attr.Value)
	}
	evt1Tree := merkle.NewMerkleTree(merkle.WithRawLeaves(leaves1))
	evt1Root := evt1Tree.Root()

	targetIdx = 2
	targetData = []byte("recver2")
	_, evt1Siblings, err := evt1Tree.Proof(targetIdx)
	require.NoError(t, err)
	err = merkle.VerifyProof(targetIdx, targetData, evt1Siblings, evt1Root)
	require.NoError(t, err)
}

// Fixed roots preserve historical event hashing when replay crosses fork boundaries.
func Test_TrxContext_EventRoot_ForkBoundaries(t *testing.T) {
	const (
		legacyRoot = "93e2141e5046991b2f6267030cf1b1ec5ddc59abd0205f3a9e5ceaca936ee689"
		btip27Root = "424a8b97a36c130712bb6ffd75bb6b6d4abe2ff186ee25e1970a49daa8c6defb"
		btip48Root = "3bee3fbdc0a89cb01275036a78662045f38d0ecd45780321054320d8cbb98907"
	)
	tests := []struct {
		height int64
		root   string
	}{
		{194_849, legacyRoot},
		{194_850, btip27Root},
		{194_851, btip27Root},
		{499_999, btip27Root},
		{500_000, btip48Root},
		{500_001, btip48Root},
		// Revisit older heights to catch any sticky, process-wide fork selection.
		{499_999, btip27Root},
		{194_849, legacyRoot},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("height_%d", tt.height), func(t *testing.T) {
			txctx := &ctrlertypes.TrxContext{
				BlockContext: ctrlertypes.TempBlockContext("0xbea701", tt.height, time.Now(), govMock, acctMock, nil, nil, nil),
				Events: []abcitypes.Event{
					{Type: "tx", Attributes: []abcitypes.EventAttribute{
						{Key: []byte("from"), Value: []byte("A")},
						{Key: []byte("to"), Value: make([]byte, 32)},
						{Key: []byte("empty"), Value: nil},
					}},
					{Type: "empty"},
					{Type: "log", Attributes: []abcitypes.EventAttribute{
						{Key: []byte("value"), Value: []byte("B")},
					}},
				},
			}
			_, root := txctx.EventRoot()
			require.Equal(t, tt.root, fmt.Sprintf("%x", root))
		})
	}
}

// Fixed roots were calculated independently with SHA-256, not production helpers.
func Test_TrxContext_EventRootBTIP48(t *testing.T) {
	a, b, c := []byte("A"), []byte("B"), []byte("C")
	d, e, f := []byte("D"), []byte("E"), []byte("F")
	g, h, i := []byte("G"), []byte("H"), []byte("I")
	// One event, one nil Value: Attributes: []EventAttribute{{Value: nil}}
	// has root H(0x00 || H(0x00)). One event, zero attributes: Attributes: nil
	// has root H(0x00 || H(nil)). No events returns a nil tree and root.
	tests := []struct {
		name   string
		values [][][]byte // events -> attributes -> value bytes
		root   string
	}{
		{"multiple_events_multiple_attributes/no_padding", [][][]byte{{a, b}, {d, e}}, "a3454c32f187389c5efdf710621f5bb27169f3dac94eddcfd0197b11d34b6da6"},
		{"multiple_events_multiple_attributes/attribute_padding_only", [][][]byte{{a, b, c}, {d, e, f}}, "190c01291807f47a0893ff23f118a74fff7c2c758d5f7edcfab836c9488e780a"},
		{"multiple_events_multiple_attributes/event_padding_only", [][][]byte{{a, b}, {d, e}, {g, h}}, "7fe2cafc270e369804ad77047de4a80b3bab431f7f304fbbfece78af4a9417ae"},
		{"multiple_events_multiple_attributes/event_and_attribute_padding", [][][]byte{{a, b, c}, {d, e, f}, {g, h, i}}, "5c9fc044af2df0f5ed60a0c478ebaf3d592252746b9589cfd2332f6af02bb55e"},
		{"multiple_events_single_attribute/no_padding", [][][]byte{{a}, {b}}, "eadec9dee35e7c04322fbd985533fca8b238f0791387ace8909f35b1ecc6bb7b"},
		{"multiple_events_single_attribute/event_padding", [][][]byte{{a}, {b}, {c}}, "d01ca942f32f34749e8111ccb7460b07a3d62c6c0ed9f06b8480b98115fb6a20"},
		{"single_event_multiple_attributes/no_padding", [][][]byte{{a, b}}, "11a4e9096291a34828282aa31f2f0ef379cdd70ee570a35a41fe3f6a325173c2"},
		{"single_event_multiple_attributes/attribute_padding", [][][]byte{{a, b, c}}, "3cd09d9b9a88bbe7190d6b007c887ac7bb2d482b6e69f26a4135346b061bc319"},
		{"single_event_single_attribute/normal_value", [][][]byte{{a}}, "c0603962fb8fc20a20da41d0e63d7402aecacba29a2157ac4e3a27d188c25dc7"},
		{"single_event_single_attribute/nil_value", [][][]byte{{nil}}, "d9de27625445003d8a9739a851e3ff8d41c0683630b4d63a88327a6aaa37c409"},
		{"single_event_single_attribute/empty_value", [][][]byte{{{}}}, "d9de27625445003d8a9739a851e3ff8d41c0683630b4d63a88327a6aaa37c409"},
		{"empty_attributes/nil_attributes", [][][]byte{nil}, "4e59bf27372b1304bc0b137d1be9d566ad58b154b6a6b5778af7f414b1d4b84c"},
		{"empty_attributes/empty_attributes", [][][]byte{{}}, "4e59bf27372b1304bc0b137d1be9d566ad58b154b6a6b5778af7f414b1d4b84c"},
		{"empty_events/nil_events", nil, ""},
		{"empty_events/empty_events", [][][]byte{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var events []abcitypes.Event
			if tt.values != nil {
				events = make([]abcitypes.Event, len(tt.values))
			}
			for i, attributes := range tt.values {
				events[i].Type = fmt.Sprintf("event_%d", i)
				if attributes != nil {
					events[i].Attributes = make([]abcitypes.EventAttribute, len(attributes))
				}
				for j, value := range attributes {
					events[i].Attributes[j] = abcitypes.EventAttribute{
						Key: []byte(fmt.Sprintf("key_%d", j)), Value: value,
					}
				}
			}
			txctx := &ctrlertypes.TrxContext{
				BlockContext: ctrlertypes.TempBlockContext("0xbea701", 500_000, time.Now(), govMock, acctMock, nil, nil, nil),
				Events:       events,
			}
			tree, root := txctx.EventRoot()
			if len(tt.values) == 0 {
				require.Nil(t, tree)
				require.Nil(t, root)
				return
			}
			require.NotNil(t, tree)
			require.Len(t, root, sha256.Size)
			require.Equal(t, tt.root, fmt.Sprintf("%x", root))
		})
	}
}

func newBenchTrxContext(eventCount, attrCount int) *ctrlertypes.TrxContext {
	txctx := &ctrlertypes.TrxContext{}
	for i := 0; i < eventCount; i++ {
		var attrs []abcitypes.EventAttribute
		for j := 0; j < attrCount; j++ {
			attrs = append(attrs, abcitypes.EventAttribute{
				Key:   []byte(fmt.Sprintf("key_%d_%d", i, j)),
				Value: []byte(fmt.Sprintf("value_%d_%d", i, j)),
				Index: true,
			})
		}
		txctx.Events = append(txctx.Events, abcitypes.Event{
			Type:       fmt.Sprintf("event_%d", i),
			Attributes: attrs,
		})
	}
	return txctx
}

func Benchmark_EventRoot(b *testing.B) {
	for _, bc := range []struct {
		name       string
		eventCount int
		attrCount  int
	}{
		{"10events_4attrs", 10, 4},
		{"100events_4attrs", 100, 4},
		{"100events_10attrs", 100, 10},
	} {
		txctx := newBenchTrxContext(bc.eventCount, bc.attrCount)
		b.Run(bc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ctrlertypes.EventRoot(txctx)
			}
		})
	}
}

func Benchmark_EventRootEx(b *testing.B) {
	for _, bc := range []struct {
		name       string
		eventCount int
		attrCount  int
	}{
		{"10events_4attrs", 10, 4},
		{"100events_4attrs", 100, 4},
		{"100events_10attrs", 100, 10},
	} {
		txctx := newBenchTrxContext(bc.eventCount, bc.attrCount)
		b.Run(bc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ctrlertypes.EventRootEx(txctx)
			}
		})
	}
}
