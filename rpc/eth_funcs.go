package rpc

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmrpccore "github.com/tendermint/tendermint/rpc/core"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmrpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

func EthChainId(ctx *tmrpctypes.Context) (string, error) {
	if resp, err := tmrpccore.ABCIQuery(ctx, "chain_id", nil, 0, false); err != nil {
		return "", err
	} else {
		return "0x" + new(big.Int).SetBytes(resp.Response.Value).Text(16), nil
	}
}

func EthGetBlockNumber(ctx *tmrpctypes.Context) (string, error) {
	if resp, err := tmrpccore.ABCIQuery(ctx, "block_height", nil, 0, false); err != nil {
		return "", err
	} else {
		return strconv.FormatInt(int64(binary.BigEndian.Uint64(resp.Response.Value)), 10), nil
	}
}

var ethAddrReg = regexp.MustCompile(`^0x(?i)[a-f0-9]$`)

func EthGetBlockByNumber(ctx *tmrpctypes.Context, number string, txDetail bool) (*coretypes.ResultBlock, error) {
	ptrHeight := new(int64)
	*ptrHeight = int64(0) // latest block
	if ethAddrReg.MatchString(number) {
		h, err := strconv.ParseInt(number, 16, 64)
		if err != nil {
			return nil, err
		}
		*ptrHeight = h
	}
	if ptrHeight != nil && *ptrHeight == 0 {
		ptrHeight = nil
	}
	return tmrpccore.Block(ctx, ptrHeight)
}

func EthGetStorageAt(ctx *tmrpctypes.Context, address string, storageSlot string, blockNumber string) (string, error) {
	addr, err := parseEthAddress(address)
	if err != nil {
		return "", err
	}
	slot, err := parseEthStorageSlot(storageSlot)
	if err != nil {
		return "", err
	}

	height, err := parseEthBlockNumber(blockNumber)
	if err != nil {
		return "", err
	}

	data := append(addr.Bytes(), slot.Bytes()...)
	if resp, err := tmrpccore.ABCIQuery(ctx, "eth_getStorageAt", data, height, false); err != nil {
		return "", err
	} else {
		return formatEthStorageAtResponse(resp.Response)
	}
}

func formatEthStorageAtResponse(response abcitypes.ResponseQuery) (string, error) {
	if response.Code != abcitypes.CodeTypeOK {
		if response.Log != "" {
			return "", fmt.Errorf("%s", response.Log)
		}
		return "", fmt.Errorf("eth_getStorageAt failed with code %d", response.Code)
	}

	value := response.Value
	if len(value) == 0 {
		value = common.Hash{}.Bytes()
	}
	if len(value) != common.HashLength {
		return "", fmt.Errorf("invalid storage value length: %d", len(value))
	}
	return "0x" + hex.EncodeToString(value), nil
}

func trimHexPrefix(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		return value[2:]
	}
	return value
}

func parseEthAddress(address string) (common.Address, error) {
	trimmed := trimHexPrefix(address)
	if len(trimmed) != common.AddressLength*2 {
		return common.Address{}, fmt.Errorf("invalid address length: %s", address)
	}
	if _, err := hex.DecodeString(trimmed); err != nil {
		return common.Address{}, fmt.Errorf("invalid address: %w", err)
	}
	return common.HexToAddress("0x" + trimmed), nil
}

func parseEthStorageSlot(storageSlot string) (common.Hash, error) {
	trimmed := trimHexPrefix(storageSlot)
	if trimmed == "" {
		trimmed = "0"
	}
	if len(trimmed) > common.HashLength*2 {
		return common.Hash{}, fmt.Errorf("storage slot exceeds 32 bytes")
	}
	if len(trimmed)%2 == 1 {
		trimmed = "0" + trimmed
	}
	if _, err := hex.DecodeString(trimmed); err != nil {
		return common.Hash{}, fmt.Errorf("invalid storage slot: %w", err)
	}
	return common.HexToHash("0x" + trimmed), nil
}

func parseEthBlockNumber(blockNumber string) (int64, error) {
	normalized := strings.ToLower(strings.TrimSpace(blockNumber))
	switch normalized {
	case "latest":
		return 0, nil
	case "earliest":
		return 1, nil
	case "":
		return 0, fmt.Errorf("block number is required")
	case "pending", "safe", "finalized":
		return 0, fmt.Errorf("unsupported block tag: %s", blockNumber)
	}

	if strings.HasPrefix(normalized, "0x") {
		height, err := strconv.ParseInt(normalized[2:], 16, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid block number: %w", err)
		}
		if height <= 0 {
			return 0, fmt.Errorf("block number must be positive")
		}
		return height, nil
	}

	height, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid block number: %w", err)
	}
	if height <= 0 {
		return 0, fmt.Errorf("block number must be positive")
	}
	return height, nil
}
