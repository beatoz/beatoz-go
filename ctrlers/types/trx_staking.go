package types

import (
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/rlp"
	"google.golang.org/protobuf/proto"
	"io"
)

type TrxPayloadStaking struct{}

var _ ITrxPayload = (*TrxPayloadStaking)(nil)

func (tx *TrxPayloadStaking) Type() int32 {
	return TRX_STAKING
}
func (tx *TrxPayloadStaking) Equal(_tx ITrxPayload) bool {
	return true
}
func (tx *TrxPayloadStaking) Decode(bz []byte) xerrors.XError {
	return nil
}

func (tx *TrxPayloadStaking) Encode() ([]byte, xerrors.XError) {
	return nil, nil
}

func (tx *TrxPayloadStaking) EncodeRLP(w io.Writer) error {
	return nil
}

func (tx *TrxPayloadStaking) DecodeRLP(s *rlp.Stream) error {
	return nil
}

//
// TrxPayloadUnstaking

type TrxPayloadUnstaking struct {
	TxHash bytes.HexBytes `json:"txhash"`
}

var _ ITrxPayload = (*TrxPayloadUnstaking)(nil)

func (tx *TrxPayloadUnstaking) Type() int32 {
	return TRX_UNSTAKING
}
func (tx *TrxPayloadUnstaking) Equal(_tx ITrxPayload) bool {
	if _tx == nil {
		return false
	}
	_tx0, ok := (_tx).(*TrxPayloadUnstaking)
	if !ok {
		return false
	}
	return bytes.Compare(tx.TxHash, _tx0.TxHash) == 0
}

func (tx *TrxPayloadUnstaking) Decode(bz []byte) xerrors.XError {
	pm := &TrxPayloadUnstakingProto{}
	if err := proto.Unmarshal(bz, pm); err != nil {
		return xerrors.From(err)
	}
	tx.TxHash = pm.TxHash
	return nil
}

func (tx *TrxPayloadUnstaking) Encode() ([]byte, xerrors.XError) {
	pm := &TrxPayloadUnstakingProto{
		TxHash: tx.TxHash,
	}

	bz, err := proto.Marshal(pm)
	return bz, xerrors.From(err)
}

func (tx *TrxPayloadUnstaking) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, tx.TxHash)
}

func (tx *TrxPayloadUnstaking) DecodeRLP(s *rlp.Stream) error {
	bz, err := s.Bytes()
	if err != nil {
		return err
	}
	tx.TxHash = bz
	return nil
}
