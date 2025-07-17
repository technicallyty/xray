package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/types/tx"
)

type Config struct {
	Name        string        `mapstructure:"name"`
	PollingRate time.Duration `mapstructure:"polling_rate"`
	CometRPC    string        `mapstructure:"cometRPC"`
}

func queryUnconfirmedTxs(ctx context.Context, rpcURL string, limit int) ([][]byte, error) {
	client, err := http.New(rpcURL, "/websocket")
	if err != nil {
		return nil, fmt.Errorf("failed to create RPC client: %w", err)
	}

	result, err := client.UnconfirmedTxs(ctx, &limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query unconfirmed txs: %w", err)
	}

	var txBytes [][]byte
	for _, txData := range result.Txs {
		txBytes = append(txBytes, txData)
	}

	return txBytes, nil
}

func decodeTransactions(txBytes [][]byte) ([]*tx.Tx, error) {
	var decodedTxs []*tx.Tx

	for i, txRaw := range txBytes {
		var decodedTx tx.Tx
		if err := decodedTx.Unmarshal(txRaw); err != nil {
			log.Printf("Failed to decode transaction %d: %v", i, err)
			continue
		}
		decodedTxs = append(decodedTxs, &decodedTx)
	}

	return decodedTxs, nil
}

func getUnconfirmedTxsAndDecode(ctx context.Context, config Config, limit int) ([]*tx.Tx, error) {
	rawTxs, err := queryUnconfirmedTxs(ctx, config.CometRPC, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query unconfirmed transactions: %w", err)
	}

	decodedTxs, err := decodeTransactions(rawTxs)
	if err != nil {
		return nil, fmt.Errorf("failed to decode transactions: %w", err)
	}

	return decodedTxs, nil
}

func shortenTypeURL(s string) string {
	s = s[1:]
	split := strings.Split(s, ".")
	return split[0] + "." + split[len(split)-1]
}
