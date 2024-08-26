package types

import (
	"bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/rlp"
	"google.golang.org/protobuf/proto"
	"io"
)

type TrxPayloadContract struct {
	Data []byte `json:"data"`
}

func (tx *TrxPayloadContract) Type() int32 {
	return TRX_CONTRACT
}

func (tx *TrxPayloadContract) Equal(_tx ITrxPayload) bool {
	if _tx == nil {
		return false
	}
	_tx0, ok := (_tx).(*TrxPayloadContract)
	if !ok {
		return false
	}
	return bytes.Compare(tx.Data, _tx0.Data) == 0
}

func (tx *TrxPayloadContract) Encode() ([]byte, xerrors.XError) {
	pm := &TrxPayloadContractProto{
		XData: tx.Data,
	}

	bz, err := proto.Marshal(pm)
	return bz, xerrors.From(err)
}

func (tx *TrxPayloadContract) Decode(bz []byte) xerrors.XError {
	pm := &TrxPayloadContractProto{}
	if err := proto.Unmarshal(bz, pm); err != nil {
		return xerrors.From(err)
	}

	tx.Data = pm.XData
	return nil
}

func (tx *TrxPayloadContract) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, tx.Data)
}

func (tx *TrxPayloadContract) DecodeRLP(s *rlp.Stream) error {
	bz, err := s.Bytes()
	if err != nil {
		return err
	}
	tx.Data = bz
	return nil
}

var _ ITrxPayload = (*TrxPayloadContract)(nil)
