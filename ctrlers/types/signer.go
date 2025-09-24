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

var signerV0 *SignerV0
var signerV1 *SignerV1

func InitSigner(chainId string) {
	signerV0 = NewSignerV0(chainId)

	_chainId, err := types.ChainIdInt(chainId)
	if err != nil {
		panic(err)
	}
	signerV1 = NewSignerV1(_chainId)
}

func getSigner(chainId string, v byte) (ISigner, xerrors.XError) {
	switch v {
	case 0, 1:
		// In signerV0, the type of chainId is string. If chainId is a hex string,
		// case-sensitivity issues may occur. Therefore, it is safer to enforce using
		// the chainId recorded in the current block.
		// The signing side will use the chainId from genesis, which is the same
		// as the chainId in the block.
		signerV0.chainId = chainId
		return signerV0, nil
	case 27, 28:
		return signerV1, nil
	default:
		return nil, xerrors.ErrInvalidTrxSig.Wrap(fmt.Errorf("invalid v value - %v", v))
	}
}

func VerifyTrxRLP(tx *Trx, chainId string) (types.Address, bytes.HexBytes, xerrors.XError) {
	signer, xerr := getSigner(chainId, tx.Sig[64])
	if xerr != nil {
		return nil, nil, xerr
	}
	return signer.VerifySender(tx)
}

func VerifyPayerTrxRLP(tx *Trx, chainId string) (types.Address, bytes.HexBytes, xerrors.XError) {
	signer, xerr := getSigner(chainId, tx.PayerSig[64])
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
