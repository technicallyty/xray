package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	sdkTx "github.com/cosmos/cosmos-sdk/types/tx"
)

const maxTxsPerEndpoint = 10

type trackedTx struct {
	tx     *sdkTx.Tx
	status string // "unconfirmed" or "confirmed"
}

type tickMsg struct {
	name string
}
type txsMsg struct {
	name string
	txs  []*sdkTx.Tx
	err  error
}

type model struct {
	endpoints []endpointModel
}

type endpointModel struct {
	name     string
	config   Config
	txs      []trackedTx
	err      error
	loading  bool
	lastPoll time.Time
	spinner  spinner.Model
}

func main() {
	endpoints := []endpointModel{
		{
			name:    "Cosmos Hub",
			config:  Config{CometRPC: "https://rpc.cosmos.directory/cosmoshub"},
			spinner: spinner.New(),
		},
		//{
		//	name:    "Osmosis",
		//	config:  Config{CometRPC: "https://rpc.osmosis.zone"},
		//	spinner: spinner.New(),
		//},
	}

	for i := range endpoints {
		endpoints[i].spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	}

	m := model{endpoints: endpoints}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Fatal(err)
	}
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, ep := range m.endpoints {
		cmds = append(cmds,
			pollCmd(ep.name, ep.config),
			tickCmd(ep.name, ep.config.PollingRate),
			ep.spinner.Tick,
		)
	}
	return tea.Batch(cmds...)
}

func pollCmd(name string, config Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		txs, err := getUnconfirmedTxsAndDecode(ctx, config, 10)
		return txsMsg{name: name, txs: txs, err: err}
	}
}

func tickCmd(name string, pollingRate time.Duration) tea.Cmd {
	return tea.Tick(pollingRate, func(t time.Time) tea.Msg {
		return tickMsg{name: name}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	for i := range m.endpoints {
		ep := &m.endpoints[i]

		switch msg := msg.(type) {

		case tea.KeyMsg:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}

		case txsMsg:
			if msg.name != ep.name {
				continue
			}

			if msg.err == nil {
				ep.err = nil
				ep.lastPoll = time.Now()

				// map for quick lookup of incoming txs
				seen := make(map[string]bool)
				for _, tx := range msg.txs {
					seen[tx.String()] = true
				}

				// mark existing txs as "confirmed" if not seen anymore
				var updatedTxs []trackedTx
				for _, tracked := range ep.txs {
					if seen[tracked.tx.String()] {
						// still unconfirmed
						updatedTxs = append(updatedTxs, trackedTx{
							tx:     tracked.tx,
							status: "unconfirmed",
						})
					} else {
						// it disappeared — mark confirmed
						updatedTxs = append(updatedTxs, trackedTx{
							tx:     tracked.tx,
							status: "confirmed",
						})
					}
				}

				// prepend new txs that aren't already tracked
				existing := make(map[string]bool)
				for _, t := range updatedTxs {
					existing[t.tx.String()] = true
				}

				for i := len(msg.txs) - 1; i >= 0; i-- {
					tx := msg.txs[i]
					if !existing[tx.String()] {
						updatedTxs = append([]trackedTx{
							{tx: tx, status: "unconfirmed"},
						}, updatedTxs...)
					}
				}

				// truncate to max limit
				if len(updatedTxs) > maxTxsPerEndpoint {
					updatedTxs = updatedTxs[:maxTxsPerEndpoint]
				}

				ep.txs = updatedTxs

			} else {
				// preserve previous txs if errored
				ep.err = msg.err
			}

			cmds = append(cmds, tickCmd(ep.name, ep.config.PollingRate))
		case tickMsg:
			if msg.name == ep.name {
				ep.loading = true
				cmds = append(cmds, pollCmd(ep.name, ep.config))
			}

		case spinner.TickMsg:
			var cmd tea.Cmd
			ep.spinner, cmd = ep.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

var boxStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	Padding(0, 1).
	Height(10).
	Width(60)

func (m model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🌐 Multi-RPC Unconfirmed Tx Monitor"))
	b.WriteString("\n\n")

	for _, ep := range m.endpoints {
		b.WriteString(renderEndpointBox(ep))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("Press q or Ctrl+C to quit.\n"))
	return b.String()
}

func renderEndpointBox(ep endpointModel) string {
	var content strings.Builder

	// header and status (always 2 lines)
	content.WriteString(titleStyle.Render(fmt.Sprintf("Endpoint: %s", ep.name)) + "\n")
	if ep.loading {
		content.WriteString(helpStyle.Render(ep.spinner.View()+" Polling...") + "\n")
	} else {
		content.WriteString(helpStyle.Render("Last updated: "+ep.lastPoll.Format("15:04:05")) + "\n")
	}

	lineCount := 2

	if len(ep.txs) == 0 {
		content.WriteString("No unconfirmed transactions.\n")
		lineCount++
	} else {
		// show up to 5 transactions max
		maxTxs := 5
		shown := 0
		for _, tracked := range ep.txs {
			if shown >= maxTxs {
				break
			}

			var prefix string
			switch tracked.status {
			case "unconfirmed":
				prefix = ep.spinner.View() + " "
			case "confirmed":
				prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✔ ")
			}

			line := prefix + styleTxLine(tracked.tx)
			content.WriteString(line + "\n")
			lineCount++ // only 1 line per tx now
			shown++
		}
	}

	// pad remaining lines with blank lines to maintain box height
	boxTotalHeight := 10 // matches boxStyle.Height
	contentHeight := boxTotalHeight - 2
	for lineCount < contentHeight {
		content.WriteRune('\n')
		lineCount++
	}

	return boxStyle.Render(content.String())
}

func styleTxLine(tx *sdkTx.Tx) string {
	var msgs []string
	for _, msg := range tx.Body.Messages {
		msgs = append(msgs, shortenTypeURL(msg.TypeUrl))
	}
	var seqs []string
	for _, s := range tx.AuthInfo.SignerInfos {
		seqs = append(seqs, strconv.FormatUint(s.Sequence, 10))
	}
	return fmt.Sprintf("Tx: %s | Seq: %s", strings.Join(msgs, ","), strings.Join(seqs, ","))
}
