package crypto_test

import (
	"bytes"
	cryptorand "crypto/rand"
	"github.com/beatoz/beatoz-go/libs"
	bytes2 "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"os"
	"path/filepath"
	"testing"
)

var TESTDIR = filepath.Join(libs.GetHome(), "walkey_test")

func TestSFilePV(t *testing.T) {
	os.MkdirAll(TESTDIR, 0700)
	defer os.RemoveAll(TESTDIR)

	privKeyFilePath := filepath.Join(TESTDIR, "test_key.json")
	privStateFilePath := filepath.Join(TESTDIR, "test_state.json")

	pass := []byte("abcdef")

	sfilePV := crypto.LoadOrGenSFilePV(privKeyFilePath, privStateFilePath, pass)
	require.NotNil(t, sfilePV)
	prvKey := sfilePV.Key.PrivKey
	require.NotNil(t, prvKey)

	sfilePV2 := crypto.LoadOrGenSFilePV(privKeyFilePath, privStateFilePath, pass)
	require.NotNil(t, sfilePV2)

	msg := make([]byte, 1024)
	cryptorand.Read(msg)
	sig, err := sfilePV.Key.PrivKey.Sign(msg)
	require.NoError(t, err)
	require.NotNil(t, sig)

	sig2, err := sfilePV.Key.PrivKey.Sign(msg)
	require.NoError(t, err)
	require.NotNil(t, sig2)

	require.True(t, sfilePV.Key.PubKey.VerifySignature(msg, sig2))
	require.True(t, sfilePV2.Key.PubKey.VerifySignature(msg, sig))

	// wrong msg and sig.
	// false expected
	msg2 := make([]byte, 1024)
	cryptorand.Read(msg2)
	require.False(t, sfilePV.Key.PubKey.VerifySignature(msg2, sig))
	require.False(t, sfilePV2.Key.PubKey.VerifySignature(msg2, sig))
}

func TestLock(t *testing.T) {
	os.MkdirAll(TESTDIR, 0700)
	defer os.RemoveAll(TESTDIR)

	pass := []byte("abcdef")

	w := crypto.NewWalletKey(pass)
	require.Nil(t, w.PrvKey())
	require.NotNil(t, w.PubKey())

	err := w.Unlock([]byte("wrong password"))
	require.Error(t, err)
	require.Nil(t, w.PrvKey())
	require.NotNil(t, w.PubKey())

	err = w.Unlock(nil)
	require.Error(t, err)
	require.Nil(t, w.PrvKey())
	require.NotNil(t, w.PubKey())

	err = w.Unlock(pass)
	require.NoError(t, err)
	require.NotNil(t, w.PrvKey())
	require.NotNil(t, w.PubKey())

	prvKey := w.PrvKey()
	prvKeyClone := w.PrvKeyClone()

	require.Equal(t, prvKey, prvKeyClone)
	require.Equal(t, w.PubKey(), secp256k1.PrivKey(prvKey).PubKey().Bytes())
	require.Equal(t, w.PubKey(), secp256k1.PrivKey(prvKeyClone).PubKey().Bytes())

	w.Lock()
	require.Nil(t, w.PrvKey())
	require.NotNil(t, w.PubKey())
	cleared := bytes.Repeat([]byte{0x00}, len(prvKey))
	require.Equal(t, prvKey, cleared)
	require.NotEqual(t, prvKeyClone, cleared)
}

func TestOpenSave(t *testing.T) {
	os.MkdirAll(TESTDIR, 0700)
	defer os.RemoveAll(TESTDIR)

	pass := []byte("abcdef")
	path := filepath.Join(TESTDIR, "test_key.json")

	w1 := crypto.NewWalletKey(pass)
	_, err := w1.Save(libs.NewFileWriter(path))
	require.NoError(t, err)

	w2, err := crypto.OpenWalletKey(libs.NewFileReader(path))
	require.NoError(t, err)

	err = w1.Unlock(pass)
	require.NoError(t, err)
	err = w2.Unlock(pass)
	require.NoError(t, err)

	require.Equal(t, w1.PrvKey(), w2.PrvKey())
	require.Equal(t, w1.PubKey(), w2.PubKey())
	require.Equal(t, w1.Address, w2.Address)
}

//func TestAddrFromPub(t *testing.T) {
//	home, err := os.UserHomeDir()
//	require.NoError(t, err)
//	err = filepath.Walk(filepath.Join(home, ".beatoz/walkeys"), func(path string, info os.FileInfo, err error) error {
//		require.NoError(t, err)
//		if info.IsDir() == false {
//			wk, err := crypto.OpenWalletKey(libs.NewFileReader(path))
//			require.NoError(t, err)
//			require.NoError(t, wk.Unlock([]byte("1111")))
//			addr, err := crypto.PubBytes2Addr(wk.PubKey())
//			//t.Logf("prvKey: %x\n", wk.PrvKey())
//			//t.Logf("prvKey: %x\n", wk.PubKey())
//			//t.Logf("prvKey: %x\n", wk.Address)
//			//t.Log("--------------------------")
//			require.NoError(t, err)
//			require.Equal(t, wk.Address, addr)
//		}
//		return nil
//	})
//	require.NoError(t, err)
//}

func TestSig2Addr(t *testing.T) {
	os.MkdirAll(TESTDIR, 0700)
	defer os.RemoveAll(TESTDIR)

	pass := []byte("abcdef")

	w := crypto.NewWalletKey(pass)
	err := w.Unlock(pass)
	require.NoError(t, err)
	require.NotNil(t, w.PrvKey())
	require.NotNil(t, w.PubKey())

	msg := bytes2.RandBytes(1024)
	sig, err := w.Sign(msg)
	require.NoError(t, err)

	addr, _, err := crypto.Sig2Addr(msg, sig)
	require.NoError(t, err)
	require.Equal(t, w.Address, addr)
}
