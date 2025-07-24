package cosmos

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/types/tx"
)

type CosmosRPCClient struct {
	client *http.HTTP
}

func NewCosmosRPCClient(endpoint string) (*CosmosRPCClient, error) {
	client, err := http.New(endpoint, "/websocket")
	if err != nil {
		return nil, fmt.Errorf("failed to create RPC client: %w", err)
	}
	return &CosmosRPCClient{client: client}, nil
}

func (c *CosmosRPCClient) MempoolTxs(ctx context.Context, limit int) ([]*tx.Tx, error) {
	result, err := c.client.UnconfirmedTxs(ctx, &limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query unconfirmed txs: %w", err)
	}

	txs := make([]*tx.Tx, 0, len(result.Txs))
	for _, txData := range result.Txs {
		decodedTx, err := decodeTransaction(txData)
		if err != nil {
			// log.Println("failed to decode tx: ", err.Error())
			continue
		}
		txs = append(txs, decodedTx)
	}
	return txs, nil
}

func decodeTransaction(txBytes []byte) (*tx.Tx, error) {
	var decodedTx tx.Tx
	if err := decodedTx.Unmarshal(txBytes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	return &decodedTx, nil
}

// TxStatus queries the status of a transaction by hash
func (c *CosmosRPCClient) TxStatus(ctx context.Context, txHash string) (*TxResult, error) {
	// Convert hex string to bytes for CometBFT query
	hashBytes, err := hex.DecodeString(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex hash: %w", err)
	}

	result, err := c.client.Tx(ctx, hashBytes, false)
	if err != nil {
		return nil, fmt.Errorf("failed to query transaction: %w", err)
	}

	return &TxResult{
		TxResult: result.TxResult,
		Height:   result.Height,
	}, nil
}

// BatchTxStatus queries multiple transaction statuses
func (c *CosmosRPCClient) BatchTxStatus(ctx context.Context, txHashes []string) ([]*TxResult, error) {
	results := make([]*TxResult, len(txHashes))

	for i, hash := range txHashes {
		result, err := c.TxStatus(ctx, hash)
		if err != nil {
			results[i] = nil
		} else {
			results[i] = result
		}
	}

	return results, nil
}

type TxResult struct {
	TxResult interface{} // The actual transaction result from CometBFT
	Height   int64       // Block height where tx was included
}
