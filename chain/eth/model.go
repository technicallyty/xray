package eth

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ethereum/go-ethereum/common"
	"github.com/technicallyty/xray/chain"
)

type EthModel struct {
	client *EthereumRPCClient
	// transactions are a map of poolName -> transactions
	transactions map[string][]*Transaction
	completed    []*Transaction
	name         string
}

func NewEthModel(client *EthereumRPCClient, endpoint string) *EthModel {
	return &EthModel{
		client:       client,
		transactions: make(map[string][]*Transaction),
		completed:    make([]*Transaction, 0),
		name:         fmt.Sprintf("Ethereum - %s", endpoint),
	}
}

var (
	// Styles for different transaction statuses
	inMempoolStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))  // bright blue
	successStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // bright green
	failedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // bright red
	evictedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // orange

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			Padding(0, 1).
			Width(60)
)

func shortenHash(hash string) string {
	if len(hash) <= 10 {
		return hash
	}
	return hash[:6] + "..." + hash[len(hash)-4:]
}

func formatGas(gas uint64) string {
	if gas >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(gas)/1000000)
	} else if gas >= 1000 {
		return fmt.Sprintf("%.1fK", float64(gas)/1000)
	}
	return fmt.Sprintf("%d", gas)
}

func getStatusPrefix(status StatusType) string {
	switch status {
	case StatusTypeSuccess:
		return "✓ "
	case StatusTypeFailed:
		return "✗ "
	case StatusTypeEvicted:
		return "⚠ "
	case StatusTypeInMempool:
		return "? "
	default:
		return ""
	}
}

func (e *EthModel) Displays() []string {
	var displays []string
	const maxTxsPerBox = 8 // Leave 2 lines for header and separator

	// Display active mempool transactions by pool
	// Always show common pool names, even if empty
	knownPools := []string{"pending", "queued"}
	poolNamesSet := make(map[string]bool)

	// Add known pools
	for _, poolName := range knownPools {
		poolNamesSet[poolName] = true
	}

	// Add any additional pools that exist
	for poolName := range e.transactions {
		poolNamesSet[poolName] = true
	}

	// Convert to sorted slice
	var poolNames []string
	for poolName := range poolNamesSet {
		poolNames = append(poolNames, poolName)
	}
	slices.Sort(poolNames)

	for _, poolName := range poolNames {
		txs := e.transactions[poolName]

		var lines []string
		lines = append(lines, fmt.Sprintf("Pool: %s (%d txs)", poolName, len(txs)))
		lines = append(lines, strings.Repeat("-", 50))

		// Limit to maxTxsPerBox transactions
		displayTxs := txs
		if len(txs) > maxTxsPerBox {
			displayTxs = txs[:maxTxsPerBox]
		}

		for _, tx := range displayTxs {
			shortHash := shortenHash(tx.Data.Hash.Hex())
			gasStr := formatGas(uint64(tx.Data.Gas))

			line := fmt.Sprintf("%s | N:%d | G:%s",
				shortHash, uint64(tx.Data.Nonce), gasStr)

			// Apply styling based on status
			line = inMempoolStyle.Render(line)
			lines = append(lines, line)
		}

		// Fill remaining lines to maintain consistent box height
		for len(lines) < 10 {
			lines = append(lines, "")
		}

		content := strings.Join(lines, "\n")
		displays = append(displays, boxStyle.Render(content))
	}

	// display completed transactions
	{
		var lines []string
		lines = append(lines, fmt.Sprintf("Completed"))
		lines = append(lines, strings.Repeat("-", 50))

		// Show last maxTxsPerBox completed transactions
		start := 0
		if len(e.completed) > maxTxsPerBox {
			start = len(e.completed) - maxTxsPerBox
		}

		for i := start; i < len(e.completed); i++ {
			tx := e.completed[i]
			shortHash := shortenHash(tx.Data.Hash.Hex())
			gasStr := formatGas(uint64(tx.Data.Gas))
			prefix := getStatusPrefix(tx.Status)

			line := fmt.Sprintf("%s%s | N:%d | G:%s",
				prefix, shortHash, uint64(tx.Data.Nonce), gasStr)

			// Apply styling based on status
			switch tx.Status {
			case StatusTypeSuccess:
				line = successStyle.Render(line)
			case StatusTypeFailed:
				line = failedStyle.Render(line)
			case StatusTypeEvicted:
				line = evictedStyle.Render(line)
			case StatusTypeInMempool:
				// Fallback for transactions where receipt fetch failed
				line = evictedStyle.Render(line)
			}

			lines = append(lines, line)
		}

		// Fill remaining lines to maintain consistent box height
		for len(lines) < 10 {
			lines = append(lines, "")
		}

		content := strings.Join(lines, "\n")
		displays = append(displays, boxStyle.Render(content))
	}

	return displays
}

func (e *EthModel) Name() string {
	return e.name
}

func (e *EthModel) Update(ctx context.Context) {
	res, err := e.client.TxPoolContent(ctx)
	if err != nil {
		//	log.Println("error while fetching transactions:", err)
		return
	}
	txMap := res.ConvertToMap()
	for poolName, txs := range txMap {
		slices.SortFunc(txs, func(a, b *Transaction) int {
			return cmp.Compare(a.Data.Gas, b.Data.Gas)
		})
		txMap[poolName] = txs
	}

	// create a set of current transaction hashes for fast lookup
	currentTxHashes := make(map[string]bool)
	for _, txs := range txMap {
		for _, tx := range txs {
			currentTxHashes[tx.Data.Hash.Hex()] = true
		}
	}

	// find all transactions in state that no longer exist in transactions
	var removedTransactions []*Transaction
	for _, txs := range e.transactions {
		for _, tx := range txs {
			if !currentTxHashes[tx.Data.Hash.Hex()] {
				removedTransactions = append(removedTransactions, tx)
			}
		}
	}

	// get receipts for removed transactions and update their status
	if len(removedTransactions) > 0 {
		txHashes := make([]common.Hash, len(removedTransactions))
		for i, tx := range removedTransactions {
			txHashes[i] = tx.Data.Hash
		}

		receipts, err := e.client.BatchTransactionReceipts(ctx, txHashes)
		if err != nil {
			// log.Println("error while fetching transaction receipts:", err)
			// When receipt fetch fails, assume transactions were evicted from mempool
			for i := range removedTransactions {
				removedTransactions[i].Status = StatusTypeEvicted
			}
		} else {
			// update transaction status based on receipt
			for i, receipt := range receipts {
				if receipt == nil {
					// transaction not found, likely failed or dropped
					removedTransactions[i].Status = StatusTypeEvicted
				} else if receipt.Status == 1 {
					// transaction successful
					removedTransactions[i].Status = StatusTypeSuccess
				} else {
					// transaction failed
					removedTransactions[i].Status = StatusTypeFailed
				}
			}
		}
		// append removed transactions to completed
		e.completed = append(e.completed, removedTransactions...)
		slices.SortFunc(e.completed, func(a, b *Transaction) int {
			return cmp.Compare(a.Data.Gas, b.Data.Gas)
		})

		// Keep only the last 50 completed transactions
		const maxCompleted = 50
		if len(e.completed) > maxCompleted {
			e.completed = e.completed[len(e.completed)-maxCompleted:]
		}
	}

	// update state with new transactions
	e.transactions = txMap
}

var _ chain.MempoolXray = &EthModel{}
