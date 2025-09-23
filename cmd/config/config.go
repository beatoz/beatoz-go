package config

import (
	"math/big"

	"github.com/beatoz/beatoz-go/types"
	tmcfg "github.com/tendermint/tendermint/config"
)

type Config struct {
	*tmcfg.Config
	chainId *big.Int
}

func DefaultConfig(chainId ...string) *Config {
	_chainId := big.NewInt(0)
	if len(chainId) > 0 {
		cid, err := types.ChainIdFrom(chainId[0])
		if err != nil {
			panic(err)
		}
		_chainId = cid
	}

	return &Config{
		Config:  tmcfg.DefaultConfig(),
		chainId: _chainId,
	}
}

func DefaultConfigWith(cfg *tmcfg.Config, chainId ...string) *Config {
	_chainId := big.NewInt(0)
	if len(chainId) > 0 {
		cid, err := types.ChainIdFrom(chainId[0])
		if err != nil {
			panic(err)
		}
		_chainId = cid
	}
	return &Config{
		Config:  cfg,
		chainId: _chainId,
	}
}

func (c *Config) SetChainId(chainId string) {
	cid, err := types.ChainIdFrom(chainId)
	if err != nil {
		panic(err)
	}
	c.chainId = cid
}

func (c *Config) ChainId() *big.Int {
	return c.chainId
}

// ChainID is override BaseConfig.ChainID() of tendermint
func (c *Config) ChainID() *big.Int {
	return c.chainId
}
