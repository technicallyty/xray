package cosmos

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/technicallyty/xray/chain"
)

type CosmosTransaction struct {
	Hash   string
	Tx     *tx.Tx
	Status StatusType
}

type StatusType string

const (
	StatusTypeInMempool StatusType = "in-mempool"
	StatusTypeSuccess   StatusType = "success"
	StatusTypeFailed    StatusType = "failed"
	StatusTypeEvicted   StatusType = "evicted"
)

type CosmosModel struct {
	client       *CosmosRPCClient
	transactions map[string]*CosmosTransaction // hash -> transaction
	completed    []*CosmosTransaction
}

func NewCosmosModel(client *CosmosRPCClient) *CosmosModel {
	return &CosmosModel{
		client:       client,
		transactions: make(map[string]*CosmosTransaction),
		completed:    make([]*CosmosTransaction, 0),
	}
}

var (
	// Styles for cosmos transactions
	inMempoolStyleCosmos = lipgloss.NewStyle().Foreground(lipgloss.Color("33")) // bright blue
	successStyleCosmos   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // bright green
	failedStyleCosmos    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // bright red
	evictedStyleCosmos   = lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // orange

	boxStyleCosmos = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			Padding(0, 1).
			Width(60)
)

func getStatusPrefixCosmos(status StatusType) string {
	switch status {
	case StatusTypeSuccess:
		return "✓ "
	case StatusTypeFailed:
		return "✗ "
	case StatusTypeEvicted:
		return "⚠ "
	case StatusTypeInMempool:
		return ""
	default:
		return ""
	}
}

func getTxHash(tx *tx.Tx) string {
	txBytes, _ := tx.Marshal()
	hash := sha256.Sum256(txBytes)
	return fmt.Sprintf("%X", hash) // Uppercase hex without 0x prefix for Cosmos
}

func shortenHashCosmos(hash string) string {
	if len(hash) <= 10 {
		return hash
	}
	return hash[:6] + "..." + hash[len(hash)-4:]
}

func formatMessageTypes(tx *tx.Tx) string {
	if tx.Body == nil || len(tx.Body.Messages) == 0 {
		return "unknown"
	}

	var msgTypes []string
	for _, msg := range tx.Body.Messages {
		// Extract just the message type from the type URL
		typeURL := msg.TypeUrl
		if idx := strings.LastIndex(typeURL, "."); idx != -1 {
			typeURL = typeURL[idx+1:]
		}
		msgTypes = append(msgTypes, typeURL)
	}

	if len(msgTypes) > 2 {
		return fmt.Sprintf("%s, %s, +%d more", msgTypes[0], msgTypes[1], len(msgTypes)-2)
	}

	return strings.Join(msgTypes, ", ")
}

func (c *CosmosModel) Update(ctx context.Context) {
	// Query current mempool transactions
	currentTxs, err := c.client.MempoolTxs(ctx, 1000) // Limit to 1000 transactions
	if err != nil {
		// On error, keep existing state
		return
	}

	// Create map of current transaction hashes
	currentTxHashes := make(map[string]bool)
	currentTxMap := make(map[string]*CosmosTransaction)

	for _, tx := range currentTxs {
		hash := getTxHash(tx)
		currentTxHashes[hash] = true
		currentTxMap[hash] = &CosmosTransaction{
			Hash:   hash,
			Tx:     tx,
			Status: StatusTypeInMempool,
		}
	}

	// Find transactions that are no longer in mempool
	var removedTransactions []*CosmosTransaction
	for hash, tx := range c.transactions {
		if !currentTxHashes[hash] {
			removedTransactions = append(removedTransactions, tx)
		}
	}

	// Check status of removed transactions
	if len(removedTransactions) > 0 {
		txHashes := make([]string, len(removedTransactions))
		for i, tx := range removedTransactions {
			txHashes[i] = tx.Hash
		}
		
		results, err := c.client.BatchTxStatus(ctx, txHashes)
		if err != nil {
			// On error, assume transactions were evicted
			for i := range removedTransactions {
				removedTransactions[i].Status = StatusTypeEvicted
			}
		} else {
			// Update transaction status based on result
			for i, result := range results {
				if result == nil {
					// Transaction not found, likely evicted
					removedTransactions[i].Status = StatusTypeEvicted
				} else {
					// Transaction found on chain - check if successful
					// Note: In Cosmos, we'd need to check the TxResult for success
					// For now, assume successful if found on chain
					removedTransactions[i].Status = StatusTypeSuccess
				}
			}
		}
		
		// Add to completed
		c.completed = append(c.completed, removedTransactions...)

		// Keep only the last 50 completed transactions
		const maxCompleted = 50
		if len(c.completed) > maxCompleted {
			c.completed = c.completed[len(c.completed)-maxCompleted:]
		}
	}

	// Update current transactions
	c.transactions = currentTxMap
}

func (c *CosmosModel) Displays() []string {
	var displays []string
	const maxTxsPerBox = 8 // Leave 2 lines for header and separator

	// Display mempool transactions box (always show)
	{
		var lines []string
		lines = append(lines, fmt.Sprintf("Mempool (%d txs)", len(c.transactions)))
		lines = append(lines, strings.Repeat("-", 50))

		// Convert map to slice for consistent ordering
		var txs []*CosmosTransaction
		for _, tx := range c.transactions {
			txs = append(txs, tx)
		}

		// Sort by hash for consistent display order
		slices.SortFunc(txs, func(a, b *CosmosTransaction) int {
			return strings.Compare(a.Hash, b.Hash)
		})

		// Limit to maxTxsPerBox transactions
		displayTxs := txs
		if len(txs) > maxTxsPerBox {
			displayTxs = txs[:maxTxsPerBox]
		}

		for _, tx := range displayTxs {
			shortHash := shortenHashCosmos(tx.Hash)
			msgTypes := formatMessageTypes(tx.Tx)

			line := fmt.Sprintf("%s | %s", shortHash, msgTypes)
			line = inMempoolStyleCosmos.Render(line)
			lines = append(lines, line)
		}

		// Fill remaining lines to maintain consistent box height
		for len(lines) < 10 {
			lines = append(lines, "")
		}

		content := strings.Join(lines, "\n")
		displays = append(displays, boxStyleCosmos.Render(content))
	}

	// Display completed transactions box (always show)
	{
		var lines []string
		lines = append(lines, fmt.Sprintf("Completed (%d txs)", len(c.completed)))
		lines = append(lines, strings.Repeat("-", 50))

		// Show last maxTxsPerBox completed transactions
		start := 0
		if len(c.completed) > maxTxsPerBox {
			start = len(c.completed) - maxTxsPerBox
		}

		for i := start; i < len(c.completed); i++ {
			tx := c.completed[i]
			shortHash := shortenHashCosmos(tx.Hash)
			msgTypes := formatMessageTypes(tx.Tx)
			prefix := getStatusPrefixCosmos(tx.Status)

			line := fmt.Sprintf("%s%s | %s | %d", prefix, shortHash, msgTypes, tx.Tx.AuthInfo.SignerInfos[0].Sequence)
			
			// Apply styling based on status
			switch tx.Status {
			case StatusTypeSuccess:
				line = successStyleCosmos.Render(line)
			case StatusTypeFailed:
				line = failedStyleCosmos.Render(line)
			case StatusTypeEvicted:
				line = evictedStyleCosmos.Render(line)
			case StatusTypeInMempool:
				line = inMempoolStyleCosmos.Render(line)
			}
			
			lines = append(lines, line)
		}

		// Fill remaining lines to maintain consistent box height
		for len(lines) < 10 {
			lines = append(lines, "")
		}

		content := strings.Join(lines, "\n")
		displays = append(displays, boxStyleCosmos.Render(content))
	}

	return displays
}

var _ chain.MempoolXray = &CosmosModel{}
