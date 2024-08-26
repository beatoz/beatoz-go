package genesis

import (
	tmtypes "github.com/tendermint/tendermint/types"
)

func DevnetGenesisDoc(chainId string) (*tmtypes.GenesisDoc, error) {
	genDoc, err := tmtypes.GenesisDocFromJSON(jsonBlobDevnetGenesis)
	if err != nil {
		return nil, err
	}
	genDoc.ChainID = chainId
	return genDoc, nil
}

var jsonBlobDevnetGenesis = []byte(`{
  "genesis_time": "2023-11-08T03:24:02.686739Z",
  "chain_id": "testnet",
  "initial_height": "0",
  "consensus_params": {
    "block": {
      "max_bytes": "22020096",
      "max_gas": "-1",
      "time_iota_ms": "1000"
    },
    "evidence": {
      "max_age_num_blocks": "100000",
      "max_age_duration": "172800000000000",
      "max_bytes": "1048576"
    },
    "validator": {
      "pub_key_types": [
        "secp256k1"
      ]
    },
    "version": {}
  },
  "validators": [
    {
      "address": "1594B3A79F75A81F0181DD6D113A95DCA419E7EC",
      "pub_key": {
        "type": "tendermint/PubKeySecp256k1",
        "value": null
      },
      "power": "10000000",
      "name": "Validator#1"
    },
    {
      "address": "3799CE8CE603BBE3F8D34FDF5C75A1E83AB23F76",
      "pub_key": {
        "type": "tendermint/PubKeySecp256k1",
        "value": null
      },
      "power": "10000000",
      "name": "Validator#2"
    },
    {
      "address": "70723973BC6E29031386EFF74BDD2D845DDB3CBD",
      "pub_key": {
        "type": "tendermint/PubKeySecp256k1",
        "value": null
      },
      "power": "10000000",
      "name": "Validator#3"
    }
  ],
  "app_hash": "2039711216871C4E46B97FA3F3A12E4454DC4C50ABA6E787DA06133373413C3A",
  "app_state": {
    "assetHolders": [
      {
        "address": "1594B3A79F75A81F0181DD6D113A95DCA419E7EC",
        "balance": "100000000000000000000000000"
      },
      {
        "address": "3799CE8CE603BBE3F8D34FDF5C75A1E83AB23F76",
        "balance": "100000000000000000000000000"
      },
      {
        "address": "70723973BC6E29031386EFF74BDD2D845DDB3CBD",
        "balance": "100000000000000000000000000"
      },
      {
        "address": "96D9FC4BBE0D5D061187B3BF2275372179269265",
        "balance": "100000000000000000000000000"
      },
      {
        "address": "E70B54C601F695A92E8D55299E8E2AA00E8061D5",
        "balance": "100000000000000000000000000"
      },
      {
        "address": "E88F36DF6606FA3BDB74E5101C164A6F9C53BF6D",
        "balance": "100000000000000000000000000"
      },
      {
        "address": "ED1805C5B90779CF87BD06023F951DB282D201B5",
        "balance": "100000000000000000000000000"
      },
      {
        "address": "F21D3DE0B740FAFA459AA4E3C3278C8066DD20DD",
        "balance": "100000000000000000000000000"
      },
      {
        "address": "F535500B493D03F2DF0F8C42F03E161C071FA529",
        "balance": "100000000000000000000000000"
      },
      {
        "address": "F549E736F805841B23653ACED2151FEB77FA1F0B",
        "balance": "100000000000000000000000000"
      }
    ],
    "govParams": {
      "version": "1",
      "maxValidatorCnt": "21",
      "minValidatorStake": "7000000000000000000000000",
      "rewardPerPower": "4756468797",
      "lazyRewardBlocks": "2592000",
      "lazyApplyingBlocks": "259200",
      "gasPrice": "250000000000",
      "minTrxGas": "4000",
      "maxTrxGas": "25000000",
      "maxBlockGas": "18446744073709551615",
      "minVotingPeriodBlocks": "259200",
      "maxVotingPeriodBlocks": "2592000",
      "minSelfStakeRatio": "50",
      "maxUpdatableStakeRatio": "33",
      "maxIndividualStakeRatio": "33",
      "slashRatio": "50",
      "signedBlocksWindow": "10000",
      "minSignedBlocks": "500"
    }
  }
}`)
