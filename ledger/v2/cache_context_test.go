package v2

import (
	"testing"

	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
)

type memoryCacheParent struct {
	values map[string][]byte
}

func newMemoryCacheParent() *memoryCacheParent {
	return &memoryCacheParent{
		values: make(map[string][]byte),
	}
}

func (parent *memoryCacheParent) getRaw(key []byte) ([]byte, bool, xerrors.XError) {
	val, ok := parent.values[string(key)]
	if !ok {
		return nil, false, nil
	}
	return cloneBytes(val), true, nil
}

func (parent *memoryCacheParent) setRaw(key, value []byte) xerrors.XError {
	parent.values[string(key)] = cloneBytes(value)
	return nil
}

func (parent *memoryCacheParent) deleteRaw(key []byte) xerrors.XError {
	delete(parent.values, string(key))
	return nil
}

func TestCacheContextReadThroughParent(t *testing.T) {
	parent := newMemoryCacheParent()
	require.NoError(t, parent.setRaw([]byte{0x01}, []byte("parent")))
	ctx := newCacheContext(parent)

	val, ok, xerr := ctx.Get([]byte{0x01})
	require.NoError(t, xerr)
	require.True(t, ok)
	require.Equal(t, []byte("parent"), val)
}

func TestCacheContextSetAndDeleteStayLocalUntilWrite(t *testing.T) {
	parent := newMemoryCacheParent()
	ctx := newCacheContext(parent)

	require.NoError(t, ctx.Set([]byte{0x01}, []byte("local")))
	val, ok, xerr := ctx.Get([]byte{0x01})
	require.NoError(t, xerr)
	require.True(t, ok)
	require.Equal(t, []byte("local"), val)

	_, ok, xerr = parent.getRaw([]byte{0x01})
	require.NoError(t, xerr)
	require.False(t, ok)

	require.NoError(t, ctx.Delete([]byte{0x01}))
	_, ok, xerr = ctx.Get([]byte{0x01})
	require.NoError(t, xerr)
	require.False(t, ok)
}

func TestCacheContextWriteFlushesDirtyEntriesToParent(t *testing.T) {
	parent := newMemoryCacheParent()
	require.NoError(t, parent.setRaw([]byte{0x01}, []byte("old")))
	ctx := newCacheContext(parent)

	require.NoError(t, ctx.Set([]byte{0x01}, []byte("new")))
	require.NoError(t, ctx.Set([]byte{0x02}, []byte("created")))
	require.NoError(t, ctx.Delete([]byte{0x03}))
	require.NoError(t, ctx.Write())

	val, ok, xerr := parent.getRaw([]byte{0x01})
	require.NoError(t, xerr)
	require.True(t, ok)
	require.Equal(t, []byte("new"), val)

	val, ok, xerr = parent.getRaw([]byte{0x02})
	require.NoError(t, xerr)
	require.True(t, ok)
	require.Equal(t, []byte("created"), val)
	require.Empty(t, ctx.dirtyEntries(true))
}

func TestCacheContextCacheWrapWritePromotesToParentCache(t *testing.T) {
	root := newMemoryCacheParent()
	blockCache := newCacheContext(root)
	txCache := blockCache.CacheWrap()

	require.NoError(t, txCache.Set([]byte{0x01}, []byte("tx")))

	_, ok, xerr := blockCache.Get([]byte{0x01})
	require.NoError(t, xerr)
	require.False(t, ok)

	require.NoError(t, txCache.Write())

	val, ok, xerr := blockCache.Get([]byte{0x01})
	require.NoError(t, xerr)
	require.True(t, ok)
	require.Equal(t, []byte("tx"), val)

	_, ok, xerr = root.getRaw([]byte{0x01})
	require.NoError(t, xerr)
	require.False(t, ok)
}

func TestCacheContextDiscardedChildDoesNotAffectParent(t *testing.T) {
	root := newMemoryCacheParent()
	blockCache := newCacheContext(root)
	txCache := blockCache.CacheWrap()

	require.NoError(t, txCache.Set([]byte{0x01}, []byte("tx")))
	txCache = nil

	_, ok, xerr := blockCache.Get([]byte{0x01})
	require.NoError(t, xerr)
	require.False(t, ok)
	require.Nil(t, txCache)
}

func TestCacheContextLastWriteWins(t *testing.T) {
	parent := newMemoryCacheParent()
	ctx := newCacheContext(parent)

	require.NoError(t, ctx.Set([]byte{0x01}, []byte("first")))
	require.NoError(t, ctx.Delete([]byte{0x01}))
	require.NoError(t, ctx.Set([]byte{0x01}, []byte("second")))
	require.NoError(t, ctx.Write())

	val, ok, xerr := parent.getRaw([]byte{0x01})
	require.NoError(t, xerr)
	require.True(t, ok)
	require.Equal(t, []byte("second"), val)
}

func TestCacheContextDirtyEntriesAreDeterministic(t *testing.T) {
	ctx := newCacheContext(newMemoryCacheParent())

	require.NoError(t, ctx.Set([]byte{0x02}, []byte("2")))
	require.NoError(t, ctx.Set([]byte{0x01}, []byte("1")))
	require.NoError(t, ctx.Delete([]byte{0x03}))

	ascending := ctx.dirtyEntries(true)
	require.Equal(t, []byte{0x01}, ascending[0].key)
	require.Equal(t, []byte{0x02}, ascending[1].key)
	require.Equal(t, []byte{0x03}, ascending[2].key)

	descending := ctx.dirtyEntries(false)
	require.Equal(t, []byte{0x03}, descending[0].key)
	require.Equal(t, []byte{0x02}, descending[1].key)
	require.Equal(t, []byte{0x01}, descending[2].key)
}

func TestCacheContextKeysWithPrefix(t *testing.T) {
	ctx := newCacheContext(newMemoryCacheParent())
	require.NoError(t, ctx.Set([]byte{0x10, 0x02}, []byte("2")))
	require.NoError(t, ctx.Set([]byte{0x10, 0x01}, []byte("1")))
	require.NoError(t, ctx.Set([]byte{0x11, 0x01}, []byte("other")))

	keys := ctx.keysWithPrefix([]byte{0x10})
	require.Equal(t, [][]byte{
		{0x10, 0x01},
		{0x10, 0x02},
	}, keys)
}

func TestCacheContextNilParent(t *testing.T) {
	ctx := newCacheContext(nil)

	require.NotPanics(t, func() {
		_, _, xerr := ctx.Get([]byte{0x01})
		require.Error(t, xerr)
	})
	require.NoError(t, ctx.Set([]byte{0x01}, []byte("local")))

	val, ok, xerr := ctx.Get([]byte{0x01})
	require.NoError(t, xerr)
	require.True(t, ok)
	require.Equal(t, []byte("local"), val)

	require.NotPanics(t, func() {
		require.Error(t, ctx.Write())
	})
}
