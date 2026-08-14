package v2

import (
	"bytes"
	"testing"

	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types"
	"github.com/stretchr/testify/require"
)

func TestLedgerKey_V1(t *testing.T) {
	addressA := types.Address(bytes.Repeat([]byte{0x01}, types.AddrSize))
	addressB := types.Address(bytes.Repeat([]byte{0x02}, types.AddrSize))
	hash := bytes.Repeat([]byte{0x03}, 32)

	tests := []struct {
		name string
		v1   []byte
		v2   []byte
	}{
		{"account", v1.LedgerKeyAccount(addressA), LedgerKeyAccount(addressA)},
		{"gov params", v1.LedgerKeyGovParams(), LedgerKeyGovParams()},
		{"proposal", v1.LedgerKeyProposal(hash), LedgerKeyProposal(hash)},
		{"frozen proposal", v1.LedgerKeyFrozenProp(hash), LedgerKeyFrozenProp(hash)},
		{"delegatee", v1.LedgerKeyDelegatee(addressA), LedgerKeyDelegatee(addressA)},
		{"voting power", v1.LedgerKeyVPower(addressA, addressB), LedgerKeyVPower(addressA, addressB)},
		{"frozen voting power", v1.LedgerKeyFrozenVPower(123, addressA), LedgerKeyFrozenVPower(123, addressA)},
		{"missed block count", v1.LedgerKeyMissedBlockCount(addressA), LedgerKeyMissedBlockCount(addressA)},
		{"total supply", v1.LedgerKeyTotalSupply(), LedgerKeyTotalSupply()},
		{"reward", v1.LedgerKeyReward(addressA), LedgerKeyReward(addressA)},
	}

	for _, test := range tests {
		require.Equal(t, test.v1, test.v2, "case=%s", test.name)
	}
}

func TestWriteSet_Lookup_Miss(t *testing.T) {
	ws := newWriteSet()

	entry, ok := ws.lookup([]byte("missing"))

	require.False(t, ok)
	require.Nil(t, entry)
}

func TestWriteSet_Set(t *testing.T) {
	ws := newWriteSet()
	key := []byte("key")
	item := &mutableTestItem{value: []byte("value")}

	ws.set(key, item)
	key[0] = 'K'
	item.value[0] = 'V'

	entry, ok := ws.lookup([]byte("key"))
	require.True(t, ok)
	require.Equal(t, []byte("key"), entry.key)
	require.Same(t, item, entry.item)
	require.Equal(t, []byte("Value"), entry.item.(*mutableTestItem).value)
	require.Equal(t, opSet, entry.op)
}

func TestWriteSet_Del(t *testing.T) {
	ws := newWriteSet()

	ws.del([]byte("key"))

	entry, ok := ws.lookup([]byte("key"))
	require.True(t, ok)
	require.Equal(t, []byte("key"), entry.key)
	require.Nil(t, entry.item)
	require.Equal(t, opDel, entry.op)
}

func TestWriteSet_LastOp(t *testing.T) {
	ws := newWriteSet()
	ws.set([]byte("key"), &mutableTestItem{value: []byte("value")})
	ws.del([]byte("key"))

	entry, ok := ws.lookup([]byte("key"))
	require.True(t, ok, "case=set_delete")
	require.Equal(t, opDel, entry.op, "case=set_delete")

	ws = newWriteSet()
	ws.del([]byte("key"))
	item := &mutableTestItem{value: []byte("value")}
	ws.set([]byte("key"), item)

	entry, ok = ws.lookup([]byte("key"))
	require.True(t, ok, "case=delete_set")
	require.Equal(t, opSet, entry.op, "case=delete_set")
	require.Same(t, item, entry.item, "case=delete_set")
}

func TestWriteSet_Ordered(t *testing.T) {
	ws := newWriteSet()
	ws.set([]byte("c"), &mutableTestItem{value: []byte("3")})
	ws.set([]byte("a"), &mutableTestItem{value: []byte("1")})
	ws.del([]byte("b"))

	entries := ws.orderedEntries()

	require.Len(t, entries, 3)
	require.Equal(t, []byte("a"), entries[0].key)
	require.Equal(t, []byte("b"), entries[1].key)
	require.Equal(t, []byte("c"), entries[2].key)
}

func TestWriteSet_Ordered_Deterministic(t *testing.T) {
	first := newWriteSet()
	first.set([]byte("c"), &mutableTestItem{value: []byte("3")})
	first.set([]byte("a"), &mutableTestItem{value: []byte("1")})
	first.set([]byte("b"), &mutableTestItem{value: []byte("2")})

	second := newWriteSet()
	second.set([]byte("b"), &mutableTestItem{value: []byte("2")})
	second.set([]byte("c"), &mutableTestItem{value: []byte("3")})
	second.set([]byte("a"), &mutableTestItem{value: []byte("1")})

	require.Equal(t, first.orderedEntries(), second.orderedEntries())
}

func TestWriteSet_Prefix(t *testing.T) {
	ws := newWriteSet()
	ws.set([]byte("acct/c"), &mutableTestItem{value: []byte("3")})
	ws.set([]byte("gov/a"), &mutableTestItem{value: []byte("1")})
	ws.del([]byte("acct/a"))
	ws.set([]byte("acct/b"), &mutableTestItem{value: []byte("2")})

	keys := ws.keysWithPrefix([]byte("acct/"))

	require.ElementsMatch(t, [][]byte{
		[]byte("acct/a"),
		[]byte("acct/b"),
		[]byte("acct/c"),
	}, keys)
}
