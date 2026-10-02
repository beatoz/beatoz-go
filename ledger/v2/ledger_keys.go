package v2

import (
	"github.com/beatoz/beatoz-go/ledger/common"
	"github.com/beatoz/beatoz-go/types"
)

var (
	KeyPrefixAccount          = common.KeyPrefixAccount
	KeyPrefixGovParams        = common.KeyPrefixGovParams
	KeyPrefixProposal         = common.KeyPrefixProposal
	KeyPrefixFrozenProp       = common.KeyPrefixFrozenProp
	KeyPrefixDelegatee        = common.KeyPrefixDelegatee
	KeyPrefixVPower           = common.KeyPrefixVPower
	KeyPrefixFrozenVPower     = common.KeyPrefixFrozenVPower
	KeyPrefixMissedBlockCount = common.KeyPrefixMissedBlockCount
	KeyPrefixTombstone        = common.KeyPrefixTombstone
	KeyPrefixTotalSupply      = common.KeyPrefixTotalSupply
	KeyPrefixReward           = common.KeyPrefixReward
)

func LedgerKeyProposal(txhash []byte) LedgerKey {
	return common.LedgerKeyProposal(txhash)
}

func LedgerKeyFrozenProp(txhash []byte) LedgerKey {
	return common.LedgerKeyFrozenProp(txhash)
}

func LedgerKeyAccount(addr types.Address) LedgerKey {
	return common.LedgerKeyAccount(addr)
}

func LedgerKeyGovParams() LedgerKey {
	return common.LedgerKeyGovParams()
}

func LedgerKeyVPower(from, to types.Address) LedgerKey {
	return common.LedgerKeyVPower(from, to)
}

func LedgerKeyDelegatee(addr types.Address) LedgerKey {
	return common.LedgerKeyDelegatee(addr)
}

func LedgerKeyFrozenVPower(height int64, from types.Address) LedgerKey {
	return common.LedgerKeyFrozenVPower(height, from)
}

func LedgerKeyMissedBlockCount(signer types.Address) LedgerKey {
	return common.LedgerKeyMissedBlockCount(signer)
}

func LedgerKeyTombstone(addr types.Address) LedgerKey {
	return common.LedgerKeyTombstone(addr)
}

func LedgerKeyTotalSupply() LedgerKey {
	return common.LedgerKeyTotalSupply()
}

func LedgerKeyReward(owner types.Address) LedgerKey {
	return common.LedgerKeyReward(owner)
}

func UnwrapKeyPrefix(key LedgerKey) []byte {
	return common.UnwrapKeyPrefix(key)
}
