package genesis

import (
	tmtypes "github.com/tendermint/tendermint/types"
)

func MainnetGenesisDoc(chainId string) (*tmtypes.GenesisDoc, error) {
	genDoc, err := tmtypes.GenesisDocFromJSON(jsonBlobMainnetGenesis)
	if err != nil {
		return nil, err
	}
	return genDoc, nil
}

var jsonBlobMainnetGenesis = []byte(`{"message": "not yet"}`)
