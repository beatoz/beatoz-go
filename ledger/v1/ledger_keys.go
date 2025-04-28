package v1

import (
	"encoding/binary"
	"github.com/beatoz/beatoz-go/types"
)

var (
	KeyPrefixAccount      = []byte{0x00}
	KeyPrefixGovParams    = []byte{0x10}
	KeyPrefixProposal     = []byte{0x11}
	KeyPrefixFrozenProp   = []byte{0x12}
	KeyPrefixDelegatee    = []byte{0x20}
	KeyPrefixVPower       = []byte{0x21}
	KeyPrefixFrozenVPower = []byte{0x22}
)

func LedgerKeyProposal(txhash []byte) LedgerKey {
	_key := make([]byte, len(KeyPrefixProposal)+len(txhash))
	copy(_key, append(KeyPrefixProposal, txhash...))
	return _key
}

func LedgerKeyFrozenProp(txhash []byte) LedgerKey {
	_key := make([]byte, len(KeyPrefixFrozenProp)+len(txhash))
	copy(_key, append(KeyPrefixFrozenProp, txhash...))
	return _key
}

func LedgerKeyAccount(addr types.Address) LedgerKey {
	key := make([]byte, len(KeyPrefixAccount)+len(addr))
	copy(key, append(KeyPrefixAccount, addr...))
	return key
}

func LedgerKeyGovParams() LedgerKey {
	_key := make([]byte, len(KeyPrefixGovParams))
	copy(_key, KeyPrefixGovParams)
	return _key
}

func LedgerKeyVPower(k0, k1 []byte) LedgerKey {
	k := make([]byte, len(KeyPrefixVPower)+len(k0)+len(k1))
	copy(k, KeyPrefixVPower)
	copy(k[len(KeyPrefixVPower):], append(k0, k1...))
	return k
}

func LedgerKeyDelegatee(k0, k1 []byte) LedgerKey {
	k := make([]byte, len(KeyPrefixDelegatee)+len(k0)+len(k1))
	copy(k, KeyPrefixDelegatee)
	copy(k[len(KeyPrefixDelegatee):], append(k0, k1...))
	return k
}

func LedgerKeyFrozenVPower(h int64, from []byte) LedgerKey {
	k := make([]byte, len(KeyPrefixFrozenVPower)+8+len(from))
	copy(k, KeyPrefixFrozenVPower)
	binary.BigEndian.PutUint64(k[len(KeyPrefixFrozenVPower):], uint64(h))
	copy(k[len(KeyPrefixFrozenVPower)+8:], from)
	return k
}

func UnwrapKeyPrefix(key LedgerKey) []byte {
	return key[1:]
}
