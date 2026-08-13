package v2

import (
	"testing"

	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
)

func TestCacheContext_New(t *testing.T) {
	ctx := newCacheContext()

	require.NotNil(t, ctx)
	require.Empty(t, ctx.orderedEntries())
}

func TestCacheContext_Read(t *testing.T) {
	{
		ctx := newCacheContext()

		value, found, xerr := ctx.read([]byte("key"))

		require.NoError(t, xerr, "case=miss")
		require.False(t, found, "case=miss")
		require.Nil(t, value, "case=miss")
	}

	{
		ctx := newCacheContext()
		item := &mutableTestItem{value: []byte("value")}
		ctx.set([]byte("key"), item)

		cached, found, xerr := ctx.read([]byte("key"))

		require.NoError(t, xerr, "case=set")
		require.True(t, found, "case=set")
		require.Same(t, item, cached, "case=set")
	}

	{
		ctx := newCacheContext()
		ctx.del([]byte("key"))

		value, found, xerr := ctx.read([]byte("key"))

		require.ErrorIs(t, xerr, xerrors.ErrNotFoundResult, "case=delete")
		require.True(t, found, "case=delete")
		require.Nil(t, value, "case=delete")
	}

	{
		var ctx *cacheContext

		value, found, xerr := ctx.read([]byte("key"))

		require.NoError(t, xerr, "case=nil")
		require.False(t, found, "case=nil")
		require.Nil(t, value, "case=nil")
	}
}
