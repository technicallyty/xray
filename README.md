# Mempool Xray

A look into the mysterious mempools.

![App screenshot](https://github.com/user-attachments/assets/ac34806c-5e73-47da-a036-610319a389a8)

## Build

`go build .`

## Run

./xray --config config.toml

## Configs

Configs are defined as toml. You can define multiple chains to view in xray. It should look like this:

```shell
[[chain_configs]]
chain_type = "eth"
rpc_endpoint = "https://ethereum-rpc.publicnode.com"
polling_rate = "50ms"
```

There is a new websocket xray that can be used for live feed, but it will not display completed transactions, and it does not display the separate subpools.

```shell
[[chain_configs]]
chain_type = "eth_sub"
rpc_endpoint = "wss://ethereum-rpc.publicnode.com"
```

To monitor a cometBFT chain, use a config like this:

```shell
[[chain_configs]]
chain_type = "cosmos"
rpc_endpoint = "http://localhost:26657"
polling_rate = "50ms"
```
