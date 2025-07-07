package genesis

import (
	"github.com/beatoz/beatoz-go/libs/jsonx"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmtypes "github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func NewGenesisDoc(
	chainID string,
	consensusParams *tmproto.ConsensusParams,
	validators []tmtypes.GenesisValidator,
	appState *GenesisAppState,
) (*tmtypes.GenesisDoc, error) {
	appStateJsonBlob, err := jsonx.Marshal(appState)
	if err != nil {
		return nil, err
	}
	appHash, err := appState.Hash()
	if err != nil {
		return nil, err
	}

	return &tmtypes.GenesisDoc{
		ChainID:         chainID,
		GenesisTime:     tmtime.Now(),
		ConsensusParams: consensusParams,
		Validators:      validators,
		AppState:        appStateJsonBlob,
		AppHash:         appHash[:],
	}, nil
}
