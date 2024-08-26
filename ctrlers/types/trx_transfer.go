package types

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/rlp"
	"io"
)

type TrxPayloadAssetTransfer struct{}

func (tx *TrxPayloadAssetTransfer) Type() int32 {
	return TRX_TRANSFER
}

func (tx *TrxPayloadAssetTransfer) Equal(_tx ITrxPayload) bool {
	return true
}

func (tx *TrxPayloadAssetTransfer) Decode(bz []byte) xerrors.XError {
	return nil
}

func (tx *TrxPayloadAssetTransfer) Encode() ([]byte, xerrors.XError) {
	return nil, nil
}

func (tx *TrxPayloadAssetTransfer) EncodeRLP(w io.Writer) error {
	return nil
}

func (tx *TrxPayloadAssetTransfer) DecodeRLP(s *rlp.Stream) error {
	return nil
}

var _ ITrxPayload = (*TrxPayloadAssetTransfer)(nil)
