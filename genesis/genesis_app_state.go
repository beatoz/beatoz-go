package genesis

import (
	types2 "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types/crypto"
)

type GenesisAppState struct {
	AssetHolders []*GenesisAssetHolder `json:"assetHolders"`
	GovParams    *types2.GovParams     `json:"govParams"`
}

func (ga *GenesisAppState) Hash() ([]byte, error) {
	hasher := crypto.DefaultHasher()
	if bz, err := ga.GovParams.Encode(); err != nil {
		return nil, err
	} else if _, err := hasher.Write(bz); err != nil {
		return nil, err
	} else {
		for _, h := range ga.AssetHolders {
			if _, err := hasher.Write(h.Hash()); err != nil {
				return nil, err
			}
		}
	}
	return hasher.Sum(nil), nil
}
