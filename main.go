package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/types/tx"
)

type Config struct {
	CometRPC string `mapstructure:"cometRPC"`
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

func main() {
	config := Config{
		CometRPC: "https://rpc.cosmos.directory/cosmoshub",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	txs, err := getUnconfirmedTxsAndDecode(ctx, config, 10)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d unconfirmed transactions\n", len(txs))
	for _, transaction := range txs {
		fmt.Println(printTxInfo(transaction))
	}
}

func printTxInfo(tx *tx.Tx) string {
	txs := make([]string, 0, len(tx.Body.Messages))
	for _, msg := range tx.Body.Messages {
		txs = append(txs, shortenTypeURL(msg.TypeUrl))
	}
	seqs := make([]string, 0, len(tx.AuthInfo.SignerInfos))
	for _, msg := range tx.AuthInfo.SignerInfos {
		seqs = append(seqs, strconv.FormatUint(msg.Sequence, 10))
	}
	return fmt.Sprintf(`
	Tx: %s
	Sequence: %s
`, strings.Join(txs, ","), strings.Join(seqs, ","))
}

func shortenTypeURL(s string) string {
	s = s[1:]
	split := strings.Split(s, ".")
	return split[0] + "." + split[len(split)-1]
}
