package main

import (
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	ChainConfigs []ChainConfig `toml:"chain_configs"`
}

type ChainType string

const (
	ChainTypeCosmos   ChainType = "cosmos"
	ChainTypeEthereum ChainType = "eth"
)

var (
	defaultConfig = Config{
		ChainConfigs: []ChainConfig{
			{RPCEndpoint: "https://rpc.cosmos.directory/cosmoshub", CType: ChainTypeCosmos},
		},
	}
)

type ChainConfig struct {
	CType       ChainType `toml:"chain_type"`
	RPCEndpoint string    `toml:"rpc_endpoint"`
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
