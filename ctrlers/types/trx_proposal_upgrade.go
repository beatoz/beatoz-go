package types

import (
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

const (
	MaxUpgradeNameLength = 64
	MaxUpgradeInfoLength = 4096
)

type UpgradePlanOption struct {
	Name   string `json:"name"`
	Height int64  `json:"height"`
	Info   string `json:"info"`
}

func ParseUpgradePlanOption(bz []byte) (*UpgradePlanOption, xerrors.XError) {
	opt := &UpgradePlanOption{}
	if err := jsonx.Unmarshal(bz, opt); err != nil {
		return nil, xerrors.ErrInvalidTrxPayloadParams.Wrap(err)
	}
	if xerr := opt.ValidateBasic(); xerr != nil {
		return nil, xerr
	}
	return opt, nil
}

func (opt *UpgradePlanOption) ValidateBasic() xerrors.XError {
	if xerr := ValidateUpgradeName(opt.Name); xerr != nil {
		return xerr
	}
	if opt.Height <= 0 {
		return xerrors.ErrInvalidTrxPayloadParams.Wrapf("invalid upgrade height: %d", opt.Height)
	}
	if len(opt.Info) > MaxUpgradeInfoLength {
		return xerrors.ErrInvalidTrxPayloadParams.Wrapf("upgrade info is too long: %d > %d", len(opt.Info), MaxUpgradeInfoLength)
	}
	return nil
}

type UpgradeCancelOption struct {
	Name string `json:"name"`
}

func ParseUpgradeCancelOption(bz []byte) (*UpgradeCancelOption, xerrors.XError) {
	opt := &UpgradeCancelOption{}
	if err := jsonx.Unmarshal(bz, opt); err != nil {
		return nil, xerrors.ErrInvalidTrxPayloadParams.Wrap(err)
	}
	if xerr := ValidateUpgradeName(opt.Name); xerr != nil {
		return nil, xerr
	}
	return opt, nil
}

func ValidateUpgradeName(name string) xerrors.XError {
	if name == "" {
		return xerrors.ErrInvalidTrxPayloadParams.Wrapf("upgrade name is empty")
	}
	if len(name) > MaxUpgradeNameLength {
		return xerrors.ErrInvalidTrxPayloadParams.Wrapf("upgrade name is too long: %d > %d", len(name), MaxUpgradeNameLength)
	}
	if name == "." || name == ".." {
		return xerrors.ErrInvalidTrxPayloadParams.Wrapf("invalid upgrade name: %q", name)
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return xerrors.ErrInvalidTrxPayloadParams.Wrapf("invalid character in upgrade name: %q", name)
		}
	}
	return nil
}
