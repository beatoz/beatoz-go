package types

import (
	"fmt"
	"math/big"

	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/beatoz/beatoz-go/types/xerrors"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

type SignerV1 struct {
	chainId *big.Int
}

func NewSignerV1(chainId *big.Int) *SignerV1 {
	return &SignerV1{chainId: chainId}
}

func (s *SignerV1) SignSender(tx *Trx, prvBytes bytes.HexBytes) (bytes.HexBytes, error) {
	return s.sign(tx, prvBytes, false)
}

func (s *SignerV1) SignPayer(tx *Trx, prvBytes bytes.HexBytes) (bytes.HexBytes, error) {
	return s.sign(tx, prvBytes, true)
}

func (s *SignerV1) sign(tx *Trx, prvBytes bytes.HexBytes, isPayer bool) (bytes.HexBytes, error) {
	preimg, xerr := s.getPreimage(tx, isPayer)
	if xerr != nil {
		return nil, xerr
	}

	prvKey, err := ethcrypto.ToECDSA(prvBytes)
	if err != nil {
		return nil, xerrors.From(err)
	}

	hmsg := crypto.DefaultHash(preimg)
	sig, err := ethcrypto.Sign(hmsg, prvKey)
	if err != nil {
		return nil, xerrors.From(err)
	}
	if len(sig) != ethcrypto.SignatureLength {
		return nil, xerrors.From(fmt.Errorf("invalid signature length - expected: %d, actual: %d", ethcrypto.SignatureLength, len(sig)))
	}

	v := sig[64]
	switch v {
	case 0, 1:
		sig[64] = v + 27
	case 27, 28:
		// do nothing
	}

	if isPayer {
		tx.PayerSig = sig
	} else {
		tx.Sig = sig
	}

	return sig, nil
}

func (s *SignerV1) VerifySender(tx *Trx) (bytes.HexBytes, bytes.HexBytes, xerrors.XError) {
	return s.verify(tx, false)
}

func (s *SignerV1) VerifyPayer(tx *Trx) (bytes.HexBytes, bytes.HexBytes, xerrors.XError) {
	return s.verify(tx, true)
}

func (s *SignerV1) verify(tx *Trx, isPayer bool) (bytes.HexBytes, bytes.HexBytes, xerrors.XError) {
	preimg, xerr := s.getPreimage(tx, isPayer)
	if xerr != nil {
		return nil, nil, xerr
	}

	addr0 := tx.From
	sig := tx.Sig
	if isPayer {
		addr0 = tx.Payer
		sig = tx.PayerSig
	}
	if len(sig) != ethcrypto.SignatureLength {
		return nil, nil, xerrors.From(fmt.Errorf("invalid signature length - expected: %d, actual: %d", ethcrypto.SignatureLength, len(sig)))
	}

	v := sig[64]
	if v == 27 || v == 28 {
		sig[64] = v - 27
	} else {
		return nil, nil, xerrors.ErrInvalidTrxSig.Wrap(fmt.Errorf("invalid signature v - expected: 27 or 28, actual: %d", v))
	}
	defer func() {
		sig[64] = v
	}()

	addr, pubKey, xerr := crypto.Sig2Addr(preimg, sig)
	if xerr != nil {
		return nil, nil, xerrors.ErrInvalidTrxSig.Wrap(xerr)
	}
	if bytes.Compare(addr0, addr) != 0 {
		return nil, nil, xerrors.ErrInvalidTrxSig.Wrap(fmt.Errorf("wrong recover address - expected: %v, actual: %v", addr0, addr))
	}
	return addr, pubKey, nil
}

func (s *SignerV1) GetPreimageSender(tx *Trx) (bytes.HexBytes, xerrors.XError) {
	return s.getPreimage(tx, false)
}

func (s *SignerV1) GetPreimagePayer(tx *Trx) (bytes.HexBytes, xerrors.XError) {
	return s.getPreimage(tx, true)
}

func (s *SignerV1) getPreimage(tx *Trx, isPayer bool) (bytes.HexBytes, xerrors.XError) {
	sig, payer, payerSig := tx.Sig, tx.Payer, tx.PayerSig
	tx.Payer, tx.PayerSig = nil, nil
	if !isPayer {
		tx.Sig = nil
	}
	defer func() {
		tx.Sig = sig
		tx.Payer = payer
		tx.PayerSig = payerSig
	}()

	bz, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return nil, xerrors.From(err)
	}
	return append(bz, s.chainId.Bytes()...), nil
}
