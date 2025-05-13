package v1

import (
	"encoding/binary"
	"github.com/beatoz/beatoz-go/types"
)

var (
	KeyPrefixAccount        = []byte{0x00}
	KeyPrefixGovParams      = []byte{0x10}
	KeyPrefixProposal       = []byte{0x11}
	KeyPrefixFrozenProp     = []byte{0x12}
	KeyPrefixDelegatee      = []byte{0x20}
	KeyPrefixVPower         = []byte{0x21}
	KeyPrefixFrozenVPower   = []byte{0x22}
	KeyPrefixSignBlocks     = []byte{0x23}
	KeyPrefixTotalSupply    = []byte{0x30}
	KeyPrefixAdjustedSupply = []byte{0x31}
	KeyPrefixReward         = []byte{0x32}
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

func LedgerKeyVPower(from, to types.Address) LedgerKey {
	k := make([]byte, len(KeyPrefixVPower)+len(from)+len(to))
	copy(k, KeyPrefixVPower)
	copy(k[len(KeyPrefixVPower):], append(from, to...))
	return k
}

func LedgerKeyDelegatee(addr, from types.Address) LedgerKey {
	k := make([]byte, len(KeyPrefixDelegatee)+len(addr)+len(from))
	copy(k, KeyPrefixDelegatee)
	copy(k[len(KeyPrefixDelegatee):], append(addr, from...))
	return k
}

func LedgerKeyFrozenVPower(h int64, from types.Address) LedgerKey {
	k := make([]byte, len(KeyPrefixFrozenVPower)+8+len(from))
	copy(k, KeyPrefixFrozenVPower)
	binary.BigEndian.PutUint64(k[len(KeyPrefixFrozenVPower):], uint64(h))
	copy(k[len(KeyPrefixFrozenVPower)+8:], from)
	return k
}

func LedgerKeySignBlocks(signer types.Address) LedgerKey {
	k := make([]byte, len(KeyPrefixSignBlocks)+len(signer))
	copy(k, KeyPrefixFrozenVPower)
	copy(k[len(KeyPrefixFrozenVPower):], signer)
	return k
}

func LedgerKeyTotalSupply() LedgerKey {
	return append(KeyPrefixAdjustedSupply, []byte("total")...)
}
func LedgerKeyAdjustedSupply() LedgerKey {
	return append(KeyPrefixAdjustedSupply, []byte("adjust")...)
}

func LedgerKeyReward(owner types.Address) LedgerKey {
	return append(KeyPrefixReward, owner...)
}

func UnwrapKeyPrefix(key LedgerKey) []byte {
	return key[1:]
}
