package cosmos

import (
	"context"
	"crypto/sha256"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/technicallyty/xray/chain"
)

type CosmosTransaction struct {
	Hash            string
	Tx              *tx.Tx
	Status          StatusType
	TimeCompleted   time.Time
	HeightCompleted int64
}

type StatusType string

const (
	StatusTypeInMempool StatusType = "in-mempool"
	StatusTypeSuccess   StatusType = "success"
	StatusTypeFailed    StatusType = "failed"
	StatusTypeEvicted   StatusType = "evicted"
	StatusTypeUnknown   StatusType = "unknown"
)

type CosmosModel struct {
	client       *CosmosRPCClient
	transactions map[string]*CosmosTransaction // hash -> transaction
	completed    []*CosmosTransaction
	name         string
	pollingRate  time.Duration
}

func NewCosmosModel(client *CosmosRPCClient, endpoint string, pollingRate time.Duration) *CosmosModel {
	return &CosmosModel{
		client:       client,
		transactions: make(map[string]*CosmosTransaction),
		completed:    make([]*CosmosTransaction, 0),
		name:         fmt.Sprintf("Cosmos - %s", endpoint),
		pollingRate:  pollingRate,
	}
}

var (
	// Styles for cosmos transactions
	inMempoolStyleCosmos = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))              // bright blue
	successStyleCosmos   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))              // bright green
	failedStyleCosmos    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))             // bright red
	evictedStyleCosmos   = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))             // orange
	fadedStyleCosmos     = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true) // dim gray and faded

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
		return "? "
	}
}

func getTxHash(tx *tx.Tx) string {
	txBytes, _ := tx.Marshal()
	hash := sha256.Sum256(txBytes)
	return fmt.Sprintf("%X", hash) // Uppercase hex without 0x prefix for Cosmos
}

func truncateHash(hash string) string {
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
		typeURL := msg.TypeUrl
		if idx := strings.LastIndex(typeURL, "."); idx != -1 {
			typeURL = typeURL[idx+1:]
		}
		msgTypes = append(msgTypes, typeURL)
	}

	if len(msgTypes) > 1 {
		return fmt.Sprintf("%s +%d more", msgTypes[0], len(msgTypes)-1)
	}

	return strings.Join(msgTypes, ", ")
}

func (c *CosmosModel) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(c.pollingRate)
		defer ticker.Stop()

		for range ticker.C {
			select {
			case <-ctx.Done():
				return
			default:
			}
			currentTxs, err := c.client.MempoolTxs(ctx, 1000)
			if err != nil {
				continue
			}

			currentTxHashes := make(map[string]bool)
			currentTxMap := make(map[string]*CosmosTransaction)
			currentRemovedTxs := make(map[string]bool)

			for _, tx := range currentTxs {
				hash := getTxHash(tx)
				currentTxHashes[hash] = true
				currentTxMap[hash] = &CosmosTransaction{
					Hash:   hash,
					Tx:     tx,
					Status: StatusTypeInMempool,
				}
			}

			for _, tx := range c.completed {
				currentRemovedTxs[tx.Hash] = true
			}

			// find transactions that are no longer in mempool
			var removedTransactions []*CosmosTransaction
			for hash, tx := range c.transactions {
				if !currentTxHashes[hash] && !currentRemovedTxs[tx.Hash] {
					tx.TimeCompleted = time.Now()
					removedTransactions = append(removedTransactions, tx)
				}
			}

			// check status of removed transactions
			if len(removedTransactions) > 0 {
				txHashes := make([]string, len(removedTransactions))
				for i, tx := range removedTransactions {
					txHashes[i] = tx.Hash
				}

				var x int
			line:
				results, err := c.client.BatchTxStatus(ctx, txHashes)
				if err != nil {
					// error, assume transactions were evicted
					for i := range removedTransactions {
						removedTransactions[i].Status = StatusTypeUnknown
					}
					x++
					if x < 5 {
						goto line
					}
				} else {
					for i, result := range results {
						if result == nil {
							// tx not found, likely evicted
							removedTransactions[i].Status = StatusTypeUnknown
						} else {
							z, ok := result.TxResult.(types.ExecTxResult)
							if ok && z.Code == types.CodeTypeOK {
								removedTransactions[i].Status = StatusTypeSuccess
								removedTransactions[i].HeightCompleted = result.Height
							} else {
								removedTransactions[i].Status = StatusTypeFailed
							}
						}
					}
				}

				c.completed = append(c.completed, removedTransactions...)

				const maxCompleted = 50
				if len(c.completed) > maxCompleted {
					c.completed = c.completed[len(c.completed)-maxCompleted:]
				}
			}

			c.transactions = currentTxMap
		}
	}()
}

func (c *CosmosModel) Displays() []string {
	var displays []string
	const maxTxsPerBox = 8 // leave 2 lines for header and separator

	// CURRENT MEMPOOL TRANSACTIONS UI
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
			shortHash := truncateHash(tx.Hash)
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

	// COMPLETED TRANSACTIONS UI
	{

		completed := make([]*CosmosTransaction, len(c.completed))
		copy(completed, c.completed)

		// there may be duplicates, so here we normalize them by using a set.
		set := map[string]*CosmosTransaction{}
		for _, tx := range completed {
			set[tx.Hash] = tx
		}

		completed = slices.Collect(maps.Values(set))

		var lines []string
		lines = append(lines, fmt.Sprintf("Completed Txs"))
		lines = append(lines, strings.Repeat("-", 50))

		slices.SortFunc(completed, func(a, b *CosmosTransaction) int {
			if a.TimeCompleted.After(b.TimeCompleted) {
				return -1 // a is newer, so it should come first
			} else if a.TimeCompleted.Before(b.TimeCompleted) {
				return 1 // b is newer, so b should come first
			}
			return 0 // they're equal
		})

		for i := range completed {
			tx := completed[i]
			shortHash := truncateHash(tx.Hash)
			msgTypes := formatMessageTypes(tx.Tx)
			prefix := getStatusPrefixCosmos(tx.Status)

			var sequence string
			if len(tx.Tx.AuthInfo.SignerInfos) > 0 {
				sequence = fmt.Sprintf("%d", tx.Tx.AuthInfo.SignerInfos[0].Sequence)
			} else {
				sequence = "?"
			}
			line := fmt.Sprintf("%s%s | %s | %s	| Height %d", prefix, shortHash, msgTypes, sequence, tx.HeightCompleted)

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
			case StatusTypeUnknown:
				line = fadedStyleCosmos.Render(line)
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

func (c *CosmosModel) Name() string {
	return c.name
}

var _ chain.MempoolXray = &CosmosModel{}
