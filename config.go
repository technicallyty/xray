package main

import (
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	ChainConfigs []ChainConfig `toml:"chain_configs"`
}

type ChainType string

const (
	ChainTypeCosmos   ChainType = "cosmos"
	ChainTypeEthereum ChainType = "eth"
	ChainTypeETHSub   ChainType = "eth_sub"
)

var (
	defaultConfig = Config{
		ChainConfigs: []ChainConfig{
			{RPCEndpoint: "https://rpc.cosmos.directory/cosmoshub", CType: ChainTypeCosmos, PollingRate: 300 * time.Millisecond},
		},
	}
)

type ChainConfig struct {
	CType       ChainType     `toml:"chain_type"`
	RPCEndpoint string        `toml:"rpc_endpoint"`
	PollingRate time.Duration `toml:"polling_rate"`
}

func ReadConfig(fileName string) (Config, error) {
	bz, err := os.ReadFile(fileName)
	if err != nil {
		return Config{}, err
	}
	var config Config
	err = toml.Unmarshal(bz, &config)
	if err != nil {
		return Config{}, err
	}
	return config, nil
}
