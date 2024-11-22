package genesis

import (
	tmtypes "github.com/tendermint/tendermint/types"
)

func TestnetGenesisDoc(chainId string) (*tmtypes.GenesisDoc, error) {
	genDoc, err := tmtypes.GenesisDocFromJSON(jsonBlobTestnetGenesis)
	if err != nil {
		return nil, err
	}
	genDoc.ChainID = chainId
	return genDoc, nil
}

var jsonBlobTestnetGenesis = []byte(`"message": "not yet"`)
