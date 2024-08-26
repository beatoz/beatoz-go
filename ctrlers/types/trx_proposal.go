package types

import (
	"bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/rlp"
	"google.golang.org/protobuf/proto"
	"io"
)

type TrxPayloadProposal struct {
	Message            string
	StartVotingHeight  int64
	VotingPeriodBlocks int64
	ApplyingHeight     int64
	OptType            int32
	Options            [][]byte
}

var _ ITrxPayload = (*TrxPayloadProposal)(nil)

func (tx *TrxPayloadProposal) Type() int32 {
	return TRX_PROPOSAL
}

func (tx *TrxPayloadProposal) Equal(_tx ITrxPayload) bool {
	if _tx == nil {
		return false
	}
	_tx0, ok := (_tx).(*TrxPayloadProposal)
	if !ok {
		return false
	}

	if tx.Message != _tx0.Message {
		return false
	}
	if tx.StartVotingHeight != _tx0.StartVotingHeight {
		return false
	}
	if tx.VotingPeriodBlocks != _tx0.VotingPeriodBlocks {
		return false
	}
	if tx.ApplyingHeight != _tx0.ApplyingHeight {
		return false
	}
	if tx.OptType != _tx0.OptType {
		return false
	}
	if len(tx.Options) != len(_tx0.Options) {
		return false
	}
	for i, d0 := range tx.Options {
		if bytes.Compare(d0, _tx0.Options[i]) != 0 {
			return false
		}
	}
	return true
}

func (tx *TrxPayloadProposal) Decode(bz []byte) xerrors.XError {
	pm := &TrxPayloadProposalProto{}
	if err := proto.Unmarshal(bz, pm); err != nil {
		return xerrors.From(err)
	}

	tx.Message = pm.Message
	tx.StartVotingHeight = pm.StartVotingHeight
	tx.VotingPeriodBlocks = pm.VotingBlocks
	tx.ApplyingHeight = pm.ApplyingHeight
	tx.OptType = pm.OptType
	tx.Options = pm.Options
	return nil
}

func (tx *TrxPayloadProposal) Encode() ([]byte, xerrors.XError) {
	pm := &TrxPayloadProposalProto{
		Message:           tx.Message,
		StartVotingHeight: tx.StartVotingHeight,
		VotingBlocks:      tx.VotingPeriodBlocks,
		ApplyingHeight:    tx.ApplyingHeight,
		OptType:           tx.OptType,
		Options:           tx.Options,
	}

	bz, err := proto.Marshal(pm)
	return bz, xerrors.From(err)
}

func (tx *TrxPayloadProposal) EncodeRLP(w io.Writer) error {

	rlpPayload := &struct {
		Message            string
		StartVotingHeight  uint64
		VotingPeriodBlocks uint64
		ApplyingHeight     uint64
		OptType            uint32
		Options            [][]byte
	}{
		Message:            tx.Message,
		StartVotingHeight:  uint64(tx.StartVotingHeight),
		VotingPeriodBlocks: uint64(tx.VotingPeriodBlocks),
		ApplyingHeight:     uint64(tx.ApplyingHeight),
		OptType:            uint32(tx.OptType),
		Options:            tx.Options,
	}
	return rlp.Encode(w, rlpPayload)
}

func (tx *TrxPayloadProposal) DecodeRLP(s *rlp.Stream) error {
	rlpPayload := &struct {
		Message            string
		StartVotingHeight  uint64
		VotingPeriodBlocks uint64
		ApplyingHeight     uint64
		OptType            uint32
		Options            [][]byte
	}{}

	if err := s.Decode(rlpPayload); err != nil {
		return err
	}

	tx.Message = rlpPayload.Message
	tx.StartVotingHeight = int64(rlpPayload.StartVotingHeight)
	tx.VotingPeriodBlocks = int64(rlpPayload.VotingPeriodBlocks)
	tx.ApplyingHeight = int64(rlpPayload.ApplyingHeight)
	tx.OptType = int32(rlpPayload.OptType)
	tx.Options = rlpPayload.Options
	return nil
}

func (tx *TrxPayloadProposal) GetOptionCount() int {
	return len(tx.Options)
}

func (tx *TrxPayloadProposal) GetOption(i int) []byte {
	if i >= len(tx.Options) {
		return nil
	}
	return tx.Options[i]
}
