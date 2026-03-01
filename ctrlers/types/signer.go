package types

import (
	"fmt"

	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

type ISigner interface {
	SignSender(*Trx, bytes.HexBytes) (bytes.HexBytes, error)
	SignPayer(*Trx, bytes.HexBytes) (bytes.HexBytes, error)
	VerifySender(*Trx) (bytes.HexBytes, bytes.HexBytes, xerrors.XError)
	VerifyPayer(*Trx) (bytes.HexBytes, bytes.HexBytes, xerrors.XError)
}

var signerV0 *SignerV0
var signerV1 *SignerV1

func InitSigner(chainId *uint256.Int) {
	signerV0 = NewSignerV0(chainId)
	signerV1 = NewSignerV1(chainId)
}

func getSigner(v byte) (ISigner, xerrors.XError) {
	if signerV0 == nil || signerV1 == nil {
		panic("signer not initialized")
	}
	switch v {
	case 0, 1:
		return signerV0, nil
	case 27, 28:
		return signerV1, nil
	default:
		return nil, xerrors.ErrInvalidTrxSig.Wrap(fmt.Errorf("invalid v value - %v", v))
	}
}

func VerifyTrxRLP(tx *Trx) (types.Address, bytes.HexBytes, xerrors.XError) {
	signer, xerr := getSigner(tx.Sig[64])
	if xerr != nil {
		return nil, nil, xerr
	}
	return signer.VerifySender(tx)
}

func VerifyPayerTrxRLP(tx *Trx) (types.Address, bytes.HexBytes, xerrors.XError) {
	signer, xerr := getSigner(tx.PayerSig[64])
	if xerr != nil {
		return nil, nil, xerr
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
