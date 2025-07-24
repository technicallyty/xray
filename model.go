package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/technicallyty/xray/chain"
	"golang.org/x/term"
)

type tickMsg struct{}

func tickCmd(pollingRate time.Duration) tea.Cmd {
	return tea.Tick(pollingRate, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

type Model struct {
	xrays       []chain.MempoolXray
	pollingRate time.Duration
	spinner     spinner.Model
}

func (m *Model) pollCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, xray := range m.xrays {
			xray.Update(ctx)
		}
		return nil
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.pollCmd(),
		tickCmd(m.pollingRate),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "q" || keyMsg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case tickMsg:
		cmds = append(cmds, m.pollCmd(), tickCmd(m.pollingRate))
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1)
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
)

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🌐 Mempool X-Ray"))
	b.WriteString("\n\n")

	// Calculate boxes per row based on terminal width
	const boxWidth = 62
	terminalWidth := 80 // fallback
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		terminalWidth = width
	}
	if terminalWidth < boxWidth {
		terminalWidth = boxWidth
	}
	boxesPerRow := terminalWidth / boxWidth
	if boxesPerRow < 1 {
		boxesPerRow = 1
	}

	var allRows []string

	// Process each xray's displays as separate rows
	for _, xray := range m.xrays {
		displays := xray.Displays()
		if len(displays) > 0 {
			// Group this xray's displays into rows
			for i := 0; i < len(displays); i += boxesPerRow {
				end := i + boxesPerRow
				if end > len(displays) {
					end = len(displays)
				}
				row := lipgloss.JoinHorizontal(lipgloss.Top, displays[i:end]...)
				allRows = append(allRows, row)
			}
		}
	}

	if len(allRows) > 0 {
		// Join all rows vertically
		result := lipgloss.JoinVertical(lipgloss.Left, allRows...)
		b.WriteString(result)
		b.WriteString("\n")
	}
	// header and status (always 2 lines)
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Press q or Ctrl+C to quit.\n"))
	return b.String()
}
