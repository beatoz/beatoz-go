package rpc

import (
	"fmt"
	"math/big"

	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/ethereum/go-ethereum/common"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

type ethAccount struct {
	Nonce   uint64         `json:"nonce,string"`
	Balance string         `json:"balance"`
	Code    bytes.HexBytes `json:"code,omitempty"`
}

// Pending is latest: there is no mempool account view.
func ethQueryAccount(ctx *tmrpctypes.Context, address common.Address, blockTag *ethBlockNumberOrHashParam) (*ethAccount, error) {
	height, err := ethQueryHeight(ctx, blockTag)
	if err != nil {
		return nil, err
	}
	data, err := ethABCIQuery(ctx, "account", address[:], height)
	if err != nil {
		return nil, err
	}
	var account ethAccount
	if err := jsonx.Unmarshal(data, &account); err != nil {
		return nil, err
	}
	return &account, nil
}

func ethGetBalance(ctx *tmrpctypes.Context, address common.Address, blockTag *ethBlockNumberOrHashParam) (string, error) {
	account, err := ethQueryAccount(ctx, address, blockTag)
	if err != nil {
		return "", err
	}
	balance, ok := new(big.Int).SetString(account.Balance, 10)
	if !ok {
		return "", fmt.Errorf("invalid account balance %q", account.Balance)
	}
	return ethQuantityBig(balance)
}

func ethGetTransactionCount(ctx *tmrpctypes.Context, address common.Address, blockTag *ethBlockNumberOrHashParam) (string, error) {
	account, err := ethQueryAccount(ctx, address, blockTag)
	if err != nil {
		return "", err
	}
	return ethQuantityUint64(account.Nonce), nil
}

func ethGetCode(ctx *tmrpctypes.Context, address common.Address, blockTag *ethBlockNumberOrHashParam) (string, error) {
	account, err := ethQueryAccount(ctx, address, blockTag)
	if err != nil {
		return "", err
	}
	return ethData(account.Code), nil
}

func ethGetStorageAt(ctx *tmrpctypes.Context, address common.Address, slot string, blockTag *ethBlockNumberOrHashParam) (string, error) {
	key, err := parseEthStorageSlot(slot)
	if err != nil {
		return "", err
	}
	height, err := ethQueryHeight(ctx, blockTag)
	if err != nil {
		return "", err
	}
	value, err := ethABCIQuery(ctx, "eth_getStorageAt", append(address[:], key...), height)
	if err != nil {
		return "", err
	}
	if len(value) != 0 && len(value) != 32 {
		return "", fmt.Errorf("invalid eth_getStorageAt value length: got %d, want 32", len(value))
	}
	return ethFixedData(value, 32), nil
}
