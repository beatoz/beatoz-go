package types

import "github.com/beatoz/beatoz-go/types/xerrors"

type IEncoder interface {
	Encode() ([]byte, xerrors.XError)
	Decode([]byte) xerrors.XError
}
