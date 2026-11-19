package upgrade

import (
	"bytes"

	"github.com/beatoz/beatoz-go/libs/jsonx"
	abytes "github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"google.golang.org/protobuf/proto"
)

type UpgradePlan struct {
	_proto UpgradePlanProto
}

func NewUpgradePlan(name string, height int64, info string, txhash []byte) *UpgradePlan {
	return &UpgradePlan{
		_proto: UpgradePlanProto{
			Name:           name,
			Height:         height,
			Info:           info,
			ProposalTxHash: bytes.Clone(txhash),
		},
	}
}

func (p *UpgradePlan) Encode() ([]byte, xerrors.XError) {
	if d, err := proto.Marshal(&p._proto); err != nil {
		return nil, xerrors.From(err)
	} else {
		return d, nil
	}
}

func (p *UpgradePlan) Decode(k, v []byte) xerrors.XError {
	if err := proto.Unmarshal(v, &p._proto); err != nil {
		return xerrors.From(err)
	}
	return nil
}

func (p *UpgradePlan) Name() string {
	return p._proto.Name
}

func (p *UpgradePlan) Height() int64 {
	return p._proto.Height
}

func (p *UpgradePlan) Info() string {
	return p._proto.Info
}

func (p *UpgradePlan) ProposalTxHash() []byte {
	return bytes.Clone(p._proto.ProposalTxHash)
}

func (p *UpgradePlan) MarshalJSON() ([]byte, error) {
	_tmp := &struct {
		Name           string          `json:"name"`
		Height         int64           `json:"height"`
		Info           string          `json:"info,omitempty"`
		ProposalTxHash abytes.HexBytes `json:"proposalTxHash,omitempty"`
	}{
		Name:           p._proto.Name,
		Height:         p._proto.Height,
		Info:           p._proto.Info,
		ProposalTxHash: p._proto.ProposalTxHash,
	}
	return jsonx.Marshal(_tmp)
}

type UpgradeDone struct {
	_proto UpgradeDoneProto
}

func NewUpgradeDone(name string, height int64) *UpgradeDone {
	return &UpgradeDone{
		_proto: UpgradeDoneProto{
			Name:   name,
			Height: height,
		},
	}
}

func (d *UpgradeDone) Encode() ([]byte, xerrors.XError) {
	if bz, err := proto.Marshal(&d._proto); err != nil {
		return nil, xerrors.From(err)
	} else {
		return bz, nil
	}
}

func (d *UpgradeDone) Decode(k, v []byte) xerrors.XError {
	if err := proto.Unmarshal(v, &d._proto); err != nil {
		return xerrors.From(err)
	}
	return nil
}

func (d *UpgradeDone) Name() string {
	return d._proto.Name
}

func (d *UpgradeDone) Height() int64 {
	return d._proto.Height
}

func (d *UpgradeDone) MarshalJSON() ([]byte, error) {
	_tmp := &struct {
		Name   string `json:"name"`
		Height int64  `json:"height"`
	}{
		Name:   d._proto.Name,
		Height: d._proto.Height,
	}
	return jsonx.Marshal(_tmp)
}
