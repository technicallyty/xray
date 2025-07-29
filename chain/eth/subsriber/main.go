package subscriber

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient/gethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/technicallyty/xray/chain"
)

type SubModel struct {
	client     *rpc.Client
	name       string
	txs        []*types.Transaction
	maxDisplay int
}

var _ chain.MempoolXray = &SubModel{}

func NewRPCClient(url string) (*rpc.Client, error) {
	return rpc.Dial(url)
}

func NewSubModel(client *rpc.Client, name string, maxDisplay int) *SubModel {
	return &SubModel{client, name, make([]*types.Transaction, 0, maxDisplay), maxDisplay}
}

func (s *SubModel) Start(ctx context.Context) {
	go func() {
		subscriber := gethclient.New(s.client)
		txChannel := make(chan *types.Transaction, 1000)
		sub, err := subscriber.SubscribeFullPendingTransactions(ctx, txChannel)
		if err != nil {
			log.Fatalln(err)
		}
		defer func() {
			s.client.Close()
			sub.Unsubscribe()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case tx, open := <-txChannel:
				if !open {
					log.Println("ethereum sub: closed")
					return
				}

				s.txs = append([]*types.Transaction{tx}, s.txs...)

				// trim to maxDisplay size
				if len(s.txs) > s.maxDisplay {
					s.txs = s.txs[:s.maxDisplay]
				}
			}
		}
	}()
}

var (
	inMempoolStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("33")) // bright blue

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			Padding(0, 1).
			Width(60)
)

func (s *SubModel) Displays() []string {
	var displays []string

	txs := s.txs

	var lines []string
	lines = append(lines, fmt.Sprintf("Pending Transactions"))
	lines = append(lines, strings.Repeat("-", 50))

	for _, tx := range txs {
		shortHash := shortenHash(tx.Hash().Hex())
		gasStr := formatGas(tx.Gas())

		line := fmt.Sprintf(
			"%s | N:%d | G:%s",
			shortHash, tx.Nonce(), gasStr,
		)

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

	return displays
}

func (s *SubModel) Name() string {
	return s.name
}

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
