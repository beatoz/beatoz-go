package types

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/rlp"
	"google.golang.org/protobuf/proto"
	"io"
)

type TrxPayloadSetDoc struct {
	Name string `json:"string,omitempty"`
	URL  string `json:"url,omitempty"`
}

func (tx *TrxPayloadSetDoc) Type() int32 {
	return TRX_SETDOC
}

func (tx *TrxPayloadSetDoc) Equal(_tx ITrxPayload) bool {
	if _tx == nil {
		return false
	}
	_tx0, ok := (_tx).(*TrxPayloadSetDoc)
	if !ok {
		return false
	}
	return (tx.Name == _tx0.Name) && (tx.URL == _tx0.URL)
}

func (tx *TrxPayloadSetDoc) Encode() ([]byte, xerrors.XError) {
	pm := &TrxPayloadSetDocProto{
		Name: tx.Name,
		Url:  tx.URL,
	}

	bz, err := proto.Marshal(pm)
	return bz, xerrors.From(err)
}

func (tx *TrxPayloadSetDoc) Decode(bz []byte) xerrors.XError {
	pm := &TrxPayloadSetDocProto{}
	if err := proto.Unmarshal(bz, pm); err != nil {
		return xerrors.From(err)
	}

	tx.Name = pm.Name
	tx.URL = pm.Url
	return nil
}

func (tx *TrxPayloadSetDoc) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{tx.Name, tx.URL})
}

func (tx *TrxPayloadSetDoc) DecodeRLP(s *rlp.Stream) error {
	var item struct {
		Name, URL string
	}
	if err := s.Decode(&item); err != nil {
		return err
	}
	tx.Name, tx.URL = item.Name, item.URL
	return nil
}

var _ ITrxPayload = (*TrxPayloadSetDoc)(nil)
