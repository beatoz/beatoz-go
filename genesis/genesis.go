package genesis

import (
	"github.com/beatoz/beatoz-go/cmd/version"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmtypes "github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func NewGenesisDoc(chainID string, validators []tmtypes.GenesisValidator, assetHolders []*GenesisAssetHolder, govParams *ctrlertypes.GovParams) (*tmtypes.GenesisDoc, error) {
	appState := GenesisAppState{
		AssetHolders: assetHolders,
		GovParams:    govParams,
	}
	appStateJsonBlob, err := jsonx.Marshal(appState)
	if err != nil {
		return nil, err
	}
	appHash, err := appState.Hash()
	if err != nil {
		return nil, err
	}

	return &tmtypes.GenesisDoc{
		ChainID:     chainID,
		GenesisTime: tmtime.Now(),
		ConsensusParams: &tmproto.ConsensusParams{
			Block:    tmtypes.DefaultBlockParams(),
			Evidence: tmtypes.DefaultEvidenceParams(),
			Validator: tmproto.ValidatorParams{
				PubKeyTypes: []string{tmtypes.ABCIPubKeyTypeSecp256k1},
			},
			Version: tmproto.VersionParams{
				AppVersion: version.Major(),
			},
		},
		Validators: validators,
		AppState:   appStateJsonBlob,
		AppHash:    appHash[:],
	}, nil
}
