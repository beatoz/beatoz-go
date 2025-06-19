package types

import (
	"fmt"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
	"io"
	"time"
)

const (
	TRX_TRANSFER int32 = 1 + iota
	TRX_STAKING
	TRX_UNSTAKING
	TRX_PROPOSAL
	TRX_VOTING
	TRX_CONTRACT
	TRX_SETDOC
	TRX_WITHDRAW
	TRX_MIN_TYPE = TRX_TRANSFER
	TRX_MAX_TYPE = TRX_WITHDRAW
)

const (
	EVENT_ATTR_TXSTATUS = "status"
	EVENT_ATTR_TXTYPE   = "type"
	EVENT_ATTR_TXSENDER = "sender"
	EVENT_ATTR_TXRECVER = "receiver"
	EVENT_ATTR_ADDRPAIR = "addrpair"
	EVENT_ATTR_AMOUNT   = "amount"
)

type trxRLP struct {
	Version  uint64
	Time     uint64
	Nonce    uint64
	From     types.Address
	To       types.Address
	Amount   bytes.HexBytes
	Gas      uint64
	GasPrice bytes.HexBytes
	Type     uint64
	Payload  bytes.HexBytes
	Sig      bytes.HexBytes

	Payer    types.Address  `rlp:"-"`
	PayerSig bytes.HexBytes `rlp:"-"`
}

type ITrxPayload interface {
	Type() int32
	Equal(ITrxPayload) bool
	Encode() ([]byte, xerrors.XError)
	Decode([]byte) xerrors.XError
	rlp.Encoder
	rlp.Decoder
}

type Trx struct {
	Version  int32          `json:"version,omitempty"`
	Time     int64          `json:"time"`
	Nonce    int64          `json:"nonce"`
	From     types.Address  `json:"from"`
	To       types.Address  `json:"to"`
	Amount   *uint256.Int   `json:"amount"`
	Gas      int64          `json:"gas"`
	GasPrice *uint256.Int   `json:"gasPrice"`
	Type     int32          `json:"type"`
	Payload  ITrxPayload    `json:"payload,omitempty"`
	Sig      bytes.HexBytes `json:"sig"`

	// Payer is the account covering the transaction fees, if it’s not the From address.
	Payer    types.Address  `json:"payer,omitempty"`
	PayerSig bytes.HexBytes `json:"payerSig,omitempty"`
}

func NewTrx(ver int32, from, to types.Address, nonce, gas int64, gasPrice, amt *uint256.Int, payload ITrxPayload) *Trx {
	return &Trx{
		Version:  ver,
		Time:     time.Now().Round(0).UTC().UnixNano(),
		Nonce:    nonce,
		From:     from,
		To:       to,
		Amount:   amt,
		Gas:      gas,
		GasPrice: gasPrice,
		Type:     payload.Type(),
		Payload:  payload,
	}
}

func (tx *Trx) Equal(_tx *Trx) bool {
	if tx.Version != _tx.Version {
		return false
	}
	if tx.Time != _tx.Time {
		return false
	}
	if tx.Nonce != _tx.Nonce {
		return false
	}
	if tx.From.Compare(_tx.From) != 0 {
		return false
	}
	if tx.To.Compare(_tx.To) != 0 {
		return false
	}
	if tx.Amount.Cmp(_tx.Amount) != 0 {
		return false
	}
	if tx.Gas != _tx.Gas {
		return false
	}
	if tx.GasPrice.Cmp(_tx.GasPrice) != 0 {
		return false
	}
	if tx.Type != _tx.Type {
		return false
	}
	if tx.Version != _tx.Version {
		return false
	}
	if bytes.Compare(tx.Sig, _tx.Sig) != 0 {
		return false
	}
	if tx.Payload != nil {
		return tx.Payload.Equal(_tx.Payload)
	} else if _tx.Payload != nil {
		return false
	}
	return true
}

func (tx *Trx) EncodeRLP(w io.Writer) error {

	var payload bytes.HexBytes
	if tx.Payload != nil {
		_tmp, err := rlp.EncodeToBytes(tx.Payload)
		if err != nil {
			return err
		}
		payload = _tmp
	}

	tmpTx := &trxRLP{
		Version:  uint64(tx.Version),
		Time:     uint64(tx.Time),
		Nonce:    uint64(tx.Nonce),
		From:     tx.From,
		To:       tx.To,
		Amount:   tx.Amount.Bytes(),
		Gas:      uint64(tx.Gas),
		GasPrice: tx.GasPrice.Bytes(),
		Type:     uint64(tx.Type),
		Payload:  payload,
		Sig:      tx.Sig,
		Payer:    tx.Payer,
		PayerSig: tx.PayerSig,
	}
	return rlp.Encode(w, tmpTx)
}

func (tx *Trx) DecodeRLP(s *rlp.Stream) error {
	rtx := &trxRLP{}
	err := s.Decode(rtx)
	if err != nil {
		return err
	}

	tx.Version = int32(rtx.Version)
	tx.Time = int64(rtx.Time)
	tx.Nonce = int64(rtx.Nonce)
	tx.From = rtx.From
	tx.To = rtx.To
	tx.Amount = new(uint256.Int).SetBytes(rtx.Amount)
	tx.Gas = int64(rtx.Gas)
	tx.GasPrice = new(uint256.Int).SetBytes(rtx.GasPrice)
	tx.Type = int32(rtx.Type)
	tx.Sig = rtx.Sig
	tx.Payer = rtx.Payer
	tx.PayerSig = rtx.PayerSig

	var payload ITrxPayload
	if rtx.Payload != nil && len(rtx.Payload) > 0 {
		switch int32(rtx.Type) {
		case TRX_TRANSFER:
			payload = &TrxPayloadAssetTransfer{}
		case TRX_STAKING:
			payload = &TrxPayloadStaking{}
		case TRX_UNSTAKING:
			payload = &TrxPayloadUnstaking{}
		case TRX_WITHDRAW:
			payload = &TrxPayloadWithdraw{}
		case TRX_PROPOSAL:
			payload = &TrxPayloadProposal{}
		case TRX_VOTING:
			payload = &TrxPayloadVoting{}
		case TRX_CONTRACT:
			payload = &TrxPayloadContract{}
		case TRX_SETDOC:
			payload = &TrxPayloadSetDoc{}
		default:
			return xerrors.ErrInvalidTrxPayloadType
		}

		if err := rlp.DecodeBytes(rtx.Payload, payload); err != nil {
			return err
		}

		//if err := payload.RLPDecode(rtx.Payload); err != nil {
		//	return err
		//}
	}

	tx.Payload = payload
	return nil
}

var _ rlp.Encoder = (*Trx)(nil)
var _ rlp.Decoder = (*Trx)(nil)

func (tx *Trx) GetType() int32 {
	return tx.Type
}

func (tx *Trx) TypeString() string {
	return TrxTypeString(tx.GetType())
}

func (tx *Trx) Decode(bz []byte) xerrors.XError {
	pm := TrxProto{}
	if err := proto.Unmarshal(bz, &pm); err != nil {
		return xerrors.From(err)
	} else if err := tx.fromProto(&pm); err != nil {
		return err
	}
	return nil
}

func (tx *Trx) Encode() ([]byte, xerrors.XError) {
	if pm, err := tx.toProto(); err != nil {
		return nil, xerrors.From(err)
	} else if bz, err := proto.Marshal(pm); err != nil {
		return nil, xerrors.From(err)
	} else {
		return bz, nil
	}
}

func (tx *Trx) fromProto(txProto *TrxProto) xerrors.XError {
	var payload ITrxPayload
	switch txProto.Type {
	case TRX_TRANSFER, TRX_STAKING:
		if txProto.XPayload != nil {
			return xerrors.ErrInvalidTrxPayloadType.Wrapf("the payload of tx type(%v) should be nil", txProto.Type)
		}
	case TRX_UNSTAKING:
		payload = &TrxPayloadUnstaking{}
		if err := payload.Decode(txProto.XPayload); err != nil {
			return err
		}
	case TRX_WITHDRAW:
		payload = &TrxPayloadWithdraw{}
		if err := payload.Decode(txProto.XPayload); err != nil {
			return err
		}
	case TRX_PROPOSAL:
		payload = &TrxPayloadProposal{}
		if err := payload.Decode(txProto.XPayload); err != nil {
			return err
		}
	case TRX_VOTING:
		payload = &TrxPayloadVoting{}
		if err := payload.Decode(txProto.XPayload); err != nil {
			return err
		}
	case TRX_CONTRACT:
		payload = &TrxPayloadContract{}
		if err := payload.Decode(txProto.XPayload); err != nil {
			return err
		}
	case TRX_SETDOC:
		payload = &TrxPayloadSetDoc{}
		if err := payload.Decode(txProto.XPayload); err != nil {
			return err
		}
	default:
		return xerrors.ErrInvalidTrxPayloadType
	}

	tx.Version = txProto.Version
	tx.Time = txProto.Time
	tx.Nonce = txProto.Nonce
	tx.From = txProto.From
	tx.To = txProto.To
	tx.Amount = new(uint256.Int).SetBytes(txProto.XAmount)
	tx.Gas = txProto.Gas
	tx.GasPrice = new(uint256.Int).SetBytes(txProto.XGasPrice)
	tx.Type = txProto.Type
	tx.Payload = payload
	tx.Sig = txProto.Sig
	tx.Payer = txProto.Payer
	tx.PayerSig = txProto.PayerSig
	return nil
}

func (tx *Trx) toProto() (*TrxProto, xerrors.XError) {
	var payload []byte
	if tx.Payload != nil {
		if bz, err := tx.Payload.Encode(); err != nil {
			return nil, err
		} else {
			payload = bz
		}
	}

	return &TrxProto{
		Version:   tx.Version,
		Time:      tx.Time,
		Nonce:     tx.Nonce,
		From:      tx.From,
		To:        tx.To,
		XAmount:   tx.Amount.Bytes(),
		Gas:       tx.Gas,
		XGasPrice: tx.GasPrice.Bytes(),
		Type:      tx.Type,
		XPayload:  payload,
		Sig:       tx.Sig,
		Payer:     tx.Payer,
		PayerSig:  tx.PayerSig,
	}, nil
}

func (tx *Trx) Validate() xerrors.XError {
	if len(tx.From) != types.AddrSize {
		return xerrors.ErrInvalidAddress
	}
	if len(tx.To) != types.AddrSize {
		if tx.Type != TRX_CONTRACT || tx.To != nil {
			return xerrors.ErrInvalidAddress
		}
	}
	if tx.Amount.Sign() < 0 {
		return xerrors.ErrInvalidAmount
	}
	if tx.GasPrice.Sign() < 0 {
		return xerrors.ErrInvalidGasPrice
	}
	if tx.Type < TRX_MIN_TYPE && tx.Type > TRX_MAX_TYPE {
		return xerrors.ErrInvalidTrxType
	}
	if tx.Payload != nil && tx.Type != tx.Payload.Type() {
		return xerrors.ErrInvalidTrxPayloadType
	}
	if tx.Sig == nil {
		return xerrors.ErrInvalidTrxSig
	}
	if tx.Payer != nil && len(tx.Payer) != types.AddrSize {
		return xerrors.ErrInvalidAddress.Wrapf("payer(%v)'s addr is invalid", tx.Payer)
	}
	if tx.Payer != nil && tx.PayerSig == nil {
		return xerrors.ErrInvalidTrxSig.Wrapf("payer(%v)'s sig is nil", tx.Payer)
	}
	return nil
}

func TrxTypeString(t int32) string {
	switch t {
	case TRX_TRANSFER:
		return "transfer"
	case TRX_STAKING:
		return "staking"
	case TRX_UNSTAKING:
		return "unstaking"
	case TRX_WITHDRAW:
		return "withdraw"
	case TRX_PROPOSAL:
		return "proposal"
	case TRX_VOTING:
		return "voting"
	case TRX_CONTRACT:
		return "contract"
	case TRX_SETDOC:
		return "setdoc"
	default:
		return "unknown"
	}
}

func VerifyTrxRLP(tx *Trx, chainId string) (types.Address, bytes.HexBytes, xerrors.XError) {
	preimg, xerr := GetPreimageSenderTrxRLP(tx, chainId)
	if xerr != nil {
		return nil, nil, xerr
	}

	fromAddr, pubKey, xerr := crypto.Sig2Addr(preimg, tx.Sig)
	if xerr != nil {
		return nil, nil, xerrors.ErrInvalidTrxSig.Wrap(xerr)
	}
	if bytes.Compare(fromAddr, tx.From) != 0 {
		return nil, nil, xerrors.ErrInvalidTrxSig.Wrap(fmt.Errorf("wrong address(or sig) - expected: %v, actual: %v", tx.From, fromAddr))
	}
	return fromAddr, pubKey, nil
}

func GetPreimageSenderTrxRLP(tx *Trx, chainId string) ([]byte, xerrors.XError) {
	sig, payer, payerSig := tx.Sig, tx.Payer, tx.PayerSig
	tx.Sig, tx.Payer, tx.PayerSig = nil, nil, nil
	defer func() {
		tx.Sig = sig
		tx.Payer = payer
		tx.PayerSig = payerSig
	}()

	bz, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return nil, xerrors.From(err)
	}
	prefix := fmt.Sprintf("\x19BEATOZ(%s) Signed Message:\n%d", chainId, len(bz))
	return append([]byte(prefix), bz...), nil
}

func VerifyPayerTrxRLP(tx *Trx, chainId string) (types.Address, bytes.HexBytes, xerrors.XError) {
	preimg, xerr := GetPreimagePayerTrxRLP(tx, chainId)
	if xerr != nil {
		return nil, nil, xerr
	}

	payerAddr, pubKey, xerr := crypto.Sig2Addr(preimg, tx.PayerSig)
	if xerr != nil {
		return nil, nil, xerrors.ErrInvalidTrxSig.Wrap(xerr)
	}
	if bytes.Compare(payerAddr, tx.Payer) != 0 {
		return nil, nil, xerrors.ErrInvalidTrxSig.Wrap(fmt.Errorf("wrong payer(or sig) - expected: %v, actual: %v", tx.Payer, payerAddr))
	}
	return payerAddr, pubKey, nil
}

func GetPreimagePayerTrxRLP(tx *Trx, chainId string) ([]byte, xerrors.XError) {
	payer, sig := tx.Payer, tx.PayerSig
	tx.Payer, tx.PayerSig = nil, nil
	defer func() {
		tx.Payer = payer
		tx.PayerSig = sig
	}()

	bz, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return nil, xerrors.From(err)
	}
	prefix := fmt.Sprintf("\x19BEATOZ(%s) Signed Message:\n%d", chainId, len(bz))
	return append([]byte(prefix), bz...), nil
}
