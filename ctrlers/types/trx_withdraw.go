package types

import (
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
	"io"
)

type TrxPayloadWithdraw struct {
	ReqAmt *uint256.Int `json:"reqAmt"`
}

var _ ITrxPayload = (*TrxPayloadWithdraw)(nil)

func (tx *TrxPayloadWithdraw) Type() int32 {
	return TRX_WITHDRAW
}
func (tx *TrxPayloadWithdraw) Equal(_tx ITrxPayload) bool {
	if _tx == nil {
		return false
	}
	_tx0, ok := (_tx).(*TrxPayloadWithdraw)
	if !ok {
		return false
	}
	return tx.ReqAmt.Cmp(_tx0.ReqAmt) == 0
}
func (tx *TrxPayloadWithdraw) Decode(bz []byte) xerrors.XError {
	pm := &TrxPayloadWithdrawProto{}
	if err := proto.Unmarshal(bz, pm); err != nil {
		return xerrors.From(err)
	}
	tx.ReqAmt = new(uint256.Int).SetBytes(pm.XReqAmt)
	return nil
}

func (tx *TrxPayloadWithdraw) Encode() ([]byte, xerrors.XError) {
	pm := &TrxPayloadWithdrawProto{
		XReqAmt: tx.ReqAmt.Bytes(),
	}

	bz, err := proto.Marshal(pm)
	return bz, xerrors.From(err)
}

func (tx *TrxPayloadWithdraw) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, tx.ReqAmt.Bytes())
}

func (tx *TrxPayloadWithdraw) DecodeRLP(s *rlp.Stream) error {
	bz, err := s.Bytes()
	if err != nil {
		return err
	}
	tx.ReqAmt = new(uint256.Int).SetBytes(bz)
	return nil
}
