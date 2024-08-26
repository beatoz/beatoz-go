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

var jsonBlobMainnetGenesis = []byte(`{
  "genesis_time": "2023-11-07T05:44:37.169318Z",
  "chain_id": "mainnet",
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
    "version": {
      "app_version": "1"
    }
  },
  "validators": [
    {
      "address": "A154CA44AB0FB8CC193C28C36506F10860B2F94B",
      "pub_key": {
        "type": "tendermint/PubKeySecp256k1",
        "value": "A129pIqszgELsjaGbrpkgQ8qOwc/VH83RIYCL+wvxDs7"
      },
      "power": "10000000",
      "name": "Validator#1"
    },
    {
      "address": "A593B9A2B5C77314D4ACA6F7BA2590E85D2C0948",
      "pub_key": {
        "type": "tendermint/PubKeySecp256k1",
        "value": "A9RNANFpSmGwzz3CIy/0doM4zR4MXJb4cAFSks6aeYUZ"
      },
      "power": "10000000",
      "name": "Validator#2"
    },
    {
      "address": "34AE6A76203A2DBF1E3FDA3DD6729F125AB857F3",
      "pub_key": {
        "type": "tendermint/PubKeySecp256k1",
        "value": "A5FCbUmx0MNDouWCijFcoP5k8GRW+Grff/xORKf4Qtk5"
      },
      "power": "10000000",
      "name": "Validator#3"
    },
    {
      "address": "417095FC2F970EC3939D63FBE6E7BB82D6858DEE",
      "pub_key": {
        "type": "tendermint/PubKeySecp256k1",
        "value": "A2xKI0+mnx0aeUYgKjPlLTFteKBFlRn/y7zZZzBulAEu"
      },
      "power": "10000000",
      "name": "Validator#4"
    },
    {
      "address": "3686A6BDCAE57A234C44F65AAD21499D80132204",
      "pub_key": {
        "type": "tendermint/PubKeySecp256k1",
        "value": "A9UI2RBlso3kO0lYXiUNfrJtErtnKsmpr0Y1YHwPY2rg"
      },
      "power": "10000000",
      "name": "Validator#5"
    }
  ],
  "app_hash": "84BA94A405CBFD316777252D5780340AB283FC804CE562BFB350BB171B99B843",
  "app_state": {
    "assetHolders": [
      {
        "address": "B91BD76189D15FBC960F775070CE23EC5D38DE55",
        "balance": "923800000000000000000000000"
      },
      {
        "address": "18493824C4D8F044D53FADB521A79108783C4155",
        "balance": "6000000000000000000000000"
      },
      {
        "address": "5D27E6BD982B6B27122910C52D22BE49337C3A3A",
        "balance": "200000000000000000000000"
      },
      {
        "address": "2C8F07CEE602F9F7050F4CFA345992AF0D3A1D24",
        "balance": "20000000000000000000000000"
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
}
`)
