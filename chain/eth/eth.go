package eth

import (
	"context"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	BlockHash           *common.Hash                 `json:"blockHash"`
	BlockNumber         *hexutil.Big                 `json:"blockNumber"`
	From                common.Address               `json:"from"`
	Gas                 hexutil.Uint64               `json:"gas"`
	GasPrice            *hexutil.Big                 `json:"gasPrice"`
	GasFeeCap           *hexutil.Big                 `json:"maxFeePerGas,omitempty"`
	GasTipCap           *hexutil.Big                 `json:"maxPriorityFeePerGas,omitempty"`
	MaxFeePerBlobGas    *hexutil.Big                 `json:"maxFeePerBlobGas,omitempty"`
	Hash                common.Hash                  `json:"hash"`
	Input               hexutil.Bytes                `json:"input"`
	Nonce               hexutil.Uint64               `json:"nonce"`
	To                  *common.Address              `json:"to"`
	TransactionIndex    *hexutil.Uint64              `json:"transactionIndex"`
	Value               *hexutil.Big                 `json:"value"`
	Type                hexutil.Uint64               `json:"type"`
	Accesses            *types.AccessList            `json:"accessList,omitempty"`
	ChainID             *hexutil.Big                 `json:"chainId,omitempty"`
	BlobVersionedHashes []common.Hash                `json:"blobVersionedHashes,omitempty"`
	AuthorizationList   []types.SetCodeAuthorization `json:"authorizationList,omitempty"`
	V                   *hexutil.Big                 `json:"v"`
	R                   *hexutil.Big                 `json:"r"`
	S                   *hexutil.Big                 `json:"s"`
	YParity             *hexutil.Uint64              `json:"yParity,omitempty"`
}

// EthereumRPCClient wraps the JSON-RPC client
type EthereumRPCClient struct {
	client *rpc.Client
}

// NewEthereumRPCClient creates a new Ethereum RPC client
func NewEthereumRPCClient(address string) (*EthereumRPCClient, error) {
	client, err := rpc.Dial(address)
	if err != nil {
		return nil, err
	}
	return &EthereumRPCClient{client: client}, nil
}

// Close closes the RPC client connection
func (c *EthereumRPCClient) Close() {
	c.client.Close()
}

// TxPoolContentResponse is map[poolName][account_addr][nonce]RPCTx
type TxPoolContentResponse map[string]map[string]map[string]*RPCTransaction

type Transaction struct {
	PoolName string
	Status   StatusType
	Data     *RPCTransaction
}

type StatusType string

const (
	StatusTypeInMempool StatusType = "in-mempool"
	StatusTypeFailed    StatusType = "failed"
	StatusTypeSuccess   StatusType = "success"
	StatusTypeEvicted   StatusType = "evicted"
)

// TxPoolContent calls the txpool_content method.
func (c *EthereumRPCClient) TxPoolContent(ctx context.Context) (TxPoolContentResponse, error) {
	var result map[string]map[string]map[string]*RPCTransaction
	err := c.client.CallContext(ctx, &result, "txpool_content")
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (p TxPoolContentResponse) ConvertToMap() map[string][]*Transaction {
	converted := make(map[string][]*Transaction, len(p))
	for poolName, txs := range p { // poolName -> map[accountAddr]map[nonce]Tx
		for _, txMap := range txs { // accountAddr -> map[nonce]Tx
			for _, tx := range txMap {
				converted[poolName] = append(converted[poolName], &Transaction{
					PoolName: poolName,
					Data:     tx,
					Status:   StatusTypeInMempool,
				})
			}
		}
	}
	return converted
}

// TransactionReceipt gets the receipt for the tx.
func (c *EthereumRPCClient) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	var r *types.Receipt
	err := c.client.CallContext(ctx, &r, "eth_getTransactionReceipt", txHash)
	if err == nil && r == nil {
		return nil, ethereum.NotFound
	}
	return r, err
}

// BatchTransactionReceipts gets receipts for multiple transactions in a single batch call
func (c *EthereumRPCClient) BatchTransactionReceipts(ctx context.Context, txHashes []common.Hash) ([]*types.Receipt, error) {
	if len(txHashes) == 0 {
		return nil, nil
	}

	// Create batch elements
	batchElems := make([]rpc.BatchElem, len(txHashes))
	receipts := make([]*types.Receipt, len(txHashes))

	for i, hash := range txHashes {
		batchElems[i] = rpc.BatchElem{
			Method: "eth_getTransactionReceipt",
			Args:   []interface{}{hash},
			Result: &receipts[i],
		}
	}

	// Execute batch call
	err := c.client.BatchCallContext(ctx, batchElems)
	if err != nil {
		return nil, err
	}

	// Check for individual errors
	for i, elem := range batchElems {
		if elem.Error != nil {
			receipts[i] = nil
		}
	}

	return receipts, nil
}
