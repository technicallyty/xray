package main

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/technicallyty/xray/chain"
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
	viewport    viewport.Model
	ready       bool
	content     string // Cache content to avoid resetting viewport
	cancel      context.CancelFunc
}

func (m *Model) Init() tea.Cmd {
	for _, xray := range m.xrays {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel
		xray.Start(ctx)
	}
	return tea.Batch(
		tickCmd(m.pollingRate),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg: // window resizing
		headerHeight := 4 // Title + help text
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
			// Initialize content on first render
			m.updateContent()
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight
			// Update content layout when window resizes
			m.updateContent()
		}
	case tea.KeyMsg: // handles keypress
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.cancel()
			return m, tea.Quit
		case "up", "k":
			m.viewport.ScrollUp(1)
		case "down", "j":
			m.viewport.ScrollDown(1)
		case "pgup", "b":
			m.viewport.HalfPageUp()
		case "pgdown", "f":
			m.viewport.HalfPageDown()
		}
	case tickMsg: // handles updating UI
		// Update content when data changes
		m.updateContent()
		cmds = append(cmds, tickCmd(m.pollingRate))
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1)
	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Padding(0, 0, 1, 0)
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
)

func (m *Model) updateContent() {
	// Calculate boxes per row based on viewport width
	const boxWidth = 62
	viewportWidth := m.viewport.Width
	if viewportWidth < boxWidth {
		viewportWidth = boxWidth
	}
	boxesPerRow := viewportWidth / boxWidth
	if boxesPerRow < 1 {
		boxesPerRow = 1
	}

	var allRows []string

	// Process each xray's displays as separate sections
	for _, xray := range m.xrays {
		displays := xray.Displays()
		if len(displays) > 0 {
			// Add section title
			sectionTitle := sectionTitleStyle.Render(xray.Name())
			allRows = append(allRows, sectionTitle)

			// Group this xray's displays into rows
			for i := 0; i < len(displays); i += boxesPerRow {
				end := i + boxesPerRow
				if end > len(displays) {
					end = len(displays)
				}
				row := lipgloss.JoinHorizontal(lipgloss.Top, displays[i:end]...)
				allRows = append(allRows, row)
			}

			// Add some spacing between sections
			allRows = append(allRows, "")
		}
	}

	newContent := ""
	if len(allRows) > 0 {
		newContent = lipgloss.JoinVertical(lipgloss.Left, allRows...)
	}

	// Only update if content changed
	if newContent != m.content {
		m.content = newContent
		m.viewport.SetContent(m.content)
	}
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	// Header
	header := titleStyle.Render("🌐 Mempool X-Ray") + "\n\n"

	// Footer
	footer := helpStyle.Render("↑/↓: scroll • PgUp/PgDn: half page • q: quit")

	return header + m.viewport.View() + "\n" + footer
}
