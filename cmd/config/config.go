package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/holiman/uint256"
	tmcfg "github.com/tendermint/tendermint/config"
)

type Config struct {
	*tmcfg.Config `mapstructure:",squash"`
	App           appConfig `mapstructure:"app"`
	chainId       *uint256.Int
}

type appConfig struct {
	QueryTimeout time.Duration `mapstructure:"query_timeout"`
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
		Config: tmcfg.DefaultConfig(),
		App: appConfig{
			QueryTimeout: time.Second,
		},
		chainId: cid,
	}
}

func DefaultConfigWith(cfg *tmcfg.Config, chainId ...string) *Config {
	conf := DefaultConfig(chainId...)
	conf.Config = cfg
	return conf
}

func WriteConfigFile(path string, config *Config) error {
	tmcfg.WriteConfigFile(path, config.Config)

	bz, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	suffix := []byte(fmt.Sprintf("\n# Beatoz application options\n[app]\nquery_timeout = %q\n", config.App.QueryTimeout))
	return os.WriteFile(path, append(bz, suffix...), 0644)
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
