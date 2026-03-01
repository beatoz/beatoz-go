package config

import (
	"strings"

	"github.com/holiman/uint256"
	tmcfg "github.com/tendermint/tendermint/config"
)

type Config struct {
	*tmcfg.Config
	chainId *uint256.Int
}

func DefaultConfig(chainId ...string) *Config {
	cid := uint256.NewInt(0)
	if len(chainId) > 0 {
		if strings.HasPrefix(chainId[0], "0x") {
			cid = uint256.MustFromHex(chainId[0])
		} else {
			cid = uint256.MustFromDecimal(chainId[0])
		}
	}

	return &Config{
		Config:  tmcfg.DefaultConfig(),
		chainId: cid,
	}
}

func DefaultConfigWith(cfg *tmcfg.Config, chainId ...string) *Config {
	conf := DefaultConfig(chainId...)
	conf.Config = cfg
	return conf
}

func (c *Config) SetChainId(chainId string) {
	cid := uint256.NewInt(0)
	if strings.HasPrefix(chainId, "0x") {
		cid = uint256.MustFromHex(chainId)
	} else {
		cid = uint256.MustFromDecimal(chainId)
	}
	c.chainId = cid
}

func (c *Config) ChainId() *uint256.Int {
	return c.chainId
}

// ChainID is override BaseConfig.ChainID() of tendermint
func (c *Config) ChainID() *uint256.Int {
	return c.chainId
}

func (c *Config) ChainIdHex() string {
	return c.chainId.Hex() // include prefix '0x' and lowercase
}
