package types

import (
	"fmt"

	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/rlp"
)

type ISigner interface {
	SignSender(*Trx, bytes.HexBytes) (bytes.HexBytes, error)
	SignPayer(*Trx, bytes.HexBytes) (bytes.HexBytes, error)
	VerifySender(*Trx) (bytes.HexBytes, bytes.HexBytes, xerrors.XError)
	VerifyPayer(*Trx) (bytes.HexBytes, bytes.HexBytes, xerrors.XError)
}

func init() {

}

func VerifyTrxRLP(tx *Trx, chainId string) (types.Address, bytes.HexBytes, xerrors.XError) {
	var signer ISigner

	v := tx.Sig[64]
	switch v {
	case 0, 1:
		signer = NewSignerV0(chainId)
	case 27, 28:
		signer = NewSignerV1(chainId)
	default:
		return nil, nil, xerrors.ErrInvalidTrxSig.Wrap(fmt.Errorf("invalid v value - %v", v))
	}

	return signer.VerifySender(tx)
}

func VerifyPayerTrxRLP(tx *Trx, chainId string) (types.Address, bytes.HexBytes, xerrors.XError) {
	var signer ISigner

	v := tx.PayerSig[64]
	switch v {
	case 0, 1:
		signer = NewSignerV0(chainId)
	case 27, 28:
		signer = NewSignerV1(chainId)
	default:
		return nil, nil, xerrors.ErrInvalidTrxSig.Wrap(fmt.Errorf("invalid v value - %v", v))
	}

	return signer.VerifyPayer(tx)
}

// DEPRECATED
// GetPreimageSenderTrxRLP does not include the sender's sig.
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

// DEPRECATED
// GetPreimagePayerTrxRLP includes the sender's sig.
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
