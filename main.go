package main

import (
	"flag"
	"log"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/technicallyty/xray/chain"
	"github.com/technicallyty/xray/chain/cosmos"
	"github.com/technicallyty/xray/chain/eth"
)

func main() {
	config := flag.String("config", "", "path to toml config file")
	flag.Parse()

	cfg := defaultConfig
	if config != nil && *config != "" {
		var err error
		cfg, err = ReadConfig(*config)
		if err != nil {
			log.Fatal(err)
		}
	}

	xrays := getXrays(cfg)

	m := Model{
		xrays:       xrays,
		pollingRate: 500 * time.Millisecond,
		spinner:     spinner.New(),
	}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Fatal(err)
	}
}

func getXrays(cfg Config) []chain.MempoolXray {
	xrays := make([]chain.MempoolXray, 0, len(cfg.ChainConfigs))
	for _, c := range cfg.ChainConfigs {
		switch c.CType {
		case ChainTypeCosmos:
			client, err := cosmos.NewCosmosRPCClient(c.RPCEndpoint)
			if err != nil {
				log.Fatal(err)
			}
			xrays = append(xrays, cosmos.NewCosmosModel(client, c.RPCEndpoint))
		case ChainTypeEthereum:
			client, err := eth.NewEthereumRPCClient(c.RPCEndpoint)
			if err != nil {
				log.Fatal(err)
			}
			xrays = append(xrays, eth.NewEthModel(client, c.RPCEndpoint))
		}
	}
	return xrays
}
