package genesis

import (
	"encoding/json"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/crypto"
	"github.com/holiman/uint256"
)

type GenesisAssetHolder struct {
	Address types.Address
	Balance *uint256.Int
}

func (gh *GenesisAssetHolder) MarshalJSON() ([]byte, error) {
	tm := &struct {
		Address types.Address `json:"address"`
		Balance string        `json:"balance"`
	}{
		Address: gh.Address,
		Balance: gh.Balance.Dec(),
	}

	return json.Marshal(tm)
}

func (gh *GenesisAssetHolder) UnmarshalJSON(bz []byte) error {
	tm := &struct {
		Address types.Address `json:"address"`
		Balance string        `json:"balance"`
	}{}

	if err := json.Unmarshal(bz, tm); err != nil {
		return err
	}

	bal, err := uint256.FromDecimal(tm.Balance)
	if err != nil {
		return err
	}

	gh.Address = tm.Address
	gh.Balance = bal

	return nil
}

func (gh *GenesisAssetHolder) Hash() []byte {
	hasher := crypto.DefaultHasher()
	hasher.Write(gh.Address[:])
	hasher.Write(gh.Balance.Bytes())
	return hasher.Sum(nil)
}

var _ json.Marshaler = (*GenesisAssetHolder)(nil)
var _ json.Unmarshaler = (*GenesisAssetHolder)(nil)
