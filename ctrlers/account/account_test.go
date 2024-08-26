package account_test

import (
	"github.com/beatoz/beatoz-go/ctrlers/account"
	account2 "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAccountEncoding(t *testing.T) {
	prvKey, err := crypto.NewPrvKey()
	require.NoError(t, err)

	addr := crypto.Pub2Addr(&prvKey.PublicKey)
	acct0 := account2.NewAccount(addr)

	encoded, err := account.EncodeAccount(acct0)
	require.NoError(t, err)

	assetAcct1, err := account.DecodeAccount(encoded)
	require.NoError(t, err)

	require.Equal(t, acct0, assetAcct1)
}

func BenchmarkAccountEncoding(b *testing.B) {
	prvKey, err := crypto.NewPrvKey()
	require.NoError(b, err)

	addr := crypto.Pub2Addr(&prvKey.PublicKey)
	acct0 := account2.NewAccount(addr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = account.EncodeAccount(acct0)
		require.NoError(b, err)
	}
}

func BenchmarkAccountDecoding(b *testing.B) {
	prvKey, err := crypto.NewPrvKey()
	require.NoError(b, err)

	addr := crypto.Pub2Addr(&prvKey.PublicKey)
	acct0 := account2.NewAccount(addr)

	encoded, err := account.EncodeAccount(acct0)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = account.DecodeAccount(encoded)
		require.NoError(b, err)
	}
}

func BenchmarkAccountToProto(b *testing.B) {
	prvKey, err := crypto.NewPrvKey()
	require.NoError(b, err)

	addr := crypto.Pub2Addr(&prvKey.PublicKey)
	acct0 := account2.NewAccount(addr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = account.ToProto(acct0)
	}
}

func BenchmarkAccountFromProto(b *testing.B) {
	prvKey, err := crypto.NewPrvKey()
	require.NoError(b, err)

	addr := crypto.Pub2Addr(&prvKey.PublicKey)
	acct0 := account2.NewAccount(addr)
	pm := account.ToProto(acct0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = account.FromProto(pm)
	}
}
