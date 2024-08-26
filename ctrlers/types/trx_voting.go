package types

import (
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/rlp"
	"google.golang.org/protobuf/proto"
	"io"
)

type TrxPayloadVoting struct {
	TxHash bytes.HexBytes
	Choice int32
}

var _ ITrxPayload = (*TrxPayloadVoting)(nil)

func (tx *TrxPayloadVoting) Type() int32 {
	return TRX_VOTING
}

func (tx *TrxPayloadVoting) Equal(_tx ITrxPayload) bool {
	if _tx == nil {
		return false
	}
	_tx0, ok := (_tx).(*TrxPayloadVoting)
	if !ok {
		return false
	}
	if bytes.Compare(tx.TxHash, _tx0.TxHash) != 0 {
		return false
	}
	if tx.Choice != _tx0.Choice {
		return false
	}
	return true
}

func (tx *TrxPayloadVoting) Decode(bz []byte) xerrors.XError {
	pm := &TrxPayloadVotingProto{}
	if err := proto.Unmarshal(bz, pm); err != nil {
		return xerrors.From(err)
	}

	tx.TxHash = pm.TxHash
	tx.Choice = pm.Choice
	return nil
}

func (tx *TrxPayloadVoting) Encode() ([]byte, xerrors.XError) {
	pm := &TrxPayloadVotingProto{
		TxHash: tx.TxHash,
		Choice: tx.Choice,
	}

	bz, err := proto.Marshal(pm)
	return bz, xerrors.From(err)
}

func (tx *TrxPayloadVoting) EncodeRLP(w io.Writer) error {
	rlpPayload := &struct {
		TxHash []byte
		Choice uint32
	}{
		TxHash: tx.TxHash,
		Choice: uint32(tx.Choice),
	}
	return rlp.Encode(w, rlpPayload)
}

func (tx *TrxPayloadVoting) DecodeRLP(s *rlp.Stream) error {
	rlpPayload := &struct {
		TxHash []byte
		Choice uint32
	}{}

	if err := s.Decode(rlpPayload); err != nil {
		return err
	}

	tx.TxHash = rlpPayload.TxHash
	tx.Choice = int32(rlpPayload.Choice)
	return nil
}
