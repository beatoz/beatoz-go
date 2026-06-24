package v2

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteSetSetCopiesInput(t *testing.T) {
	ws := newWriteSet()

	key := []byte{0x01}
	val := []byte("value")
	ws.set(key, val)

	key[0] = 0xff
	val[0] = 'X'

	entry, ok := ws.get([]byte{0x01})
	require.True(t, ok)
	require.Equal(t, []byte{0x01}, entry.key)
	require.Equal(t, []byte("value"), entry.val)
	require.Equal(t, opSet, entry.op)
}

func TestWriteSetLastWriteWins(t *testing.T) {
	ws := newWriteSet()
	key := []byte{0x01}

	ws.set(key, []byte("first"))
	ws.del(key)

	entry, ok := ws.get(key)
	require.True(t, ok)
	require.Equal(t, opDel, entry.op)
	require.Nil(t, entry.val)

	ws.set(key, []byte("second"))
	entry, ok = ws.get(key)
	require.True(t, ok)
	require.Equal(t, opSet, entry.op)
	require.Equal(t, []byte("second"), entry.val)
}

func TestWriteSetOrderedEntries(t *testing.T) {
	ws := newWriteSet()
	ws.set([]byte{0x02}, []byte("2"))
	ws.set([]byte{0x01}, []byte("1"))
	ws.del([]byte{0x03})

	ascending := ws.orderedEntries(true)
	require.Equal(t, []byte{0x01}, ascending[0].key)
	require.Equal(t, []byte{0x02}, ascending[1].key)
	require.Equal(t, []byte{0x03}, ascending[2].key)

	descending := ws.orderedEntries(false)
	require.Equal(t, []byte{0x03}, descending[0].key)
	require.Equal(t, []byte{0x02}, descending[1].key)
	require.Equal(t, []byte{0x01}, descending[2].key)
}

func TestWriteSetKeysWithPrefix(t *testing.T) {
	ws := newWriteSet()
	ws.set([]byte{0x10, 0x02}, []byte("2"))
	ws.set([]byte{0x10, 0x01}, []byte("1"))
	ws.set([]byte{0x11, 0x01}, []byte("other"))

	keys := ws.keysWithPrefix([]byte{0x10})
	require.Equal(t, [][]byte{
		{0x10, 0x01},
		{0x10, 0x02},
	}, keys)
}
