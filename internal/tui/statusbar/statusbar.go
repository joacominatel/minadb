package statusbar

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joacominatel/minadb/internal/tui/theme"
)

// Model is the status bar component.
type Model struct {
	width      int
	connected  bool
	connName   string
	activePane string
	message    string
}

// New creates a new status bar model.
func New() Model {
	return Model{
		activePane: "explorer",
	}
}

// SetWidth updates the component width.
func (m *Model) SetWidth(w int) {
	m.width = w
}

// SetConnected updates the connection status display.
func (m *Model) SetConnected(connected bool, name string) {
	m.connected = connected
	m.connName = name
}

// SetActivePane updates the displayed active pane name.
func (m *Model) SetActivePane(pane string) {
	m.activePane = pane
}

// SetMessage sets a temporary status message.
func (m *Model) SetMessage(msg string) {
	m.message = msg
}

// Init returns the initial command (none).
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages (status bar has no interactive behavior).
func (m Model) Update(_ tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

// View renders the status bar.
func (m Model) View() string {
	style := theme.StyleStatusBar.Width(m.width)

	// Connection indicator
	var connIndicator string
	if m.connected {
		connIndicator = lipgloss.NewStyle().
			Foreground(theme.ColorSuccess).
			Render("●") + " " + m.connName
	} else {
		connIndicator = lipgloss.NewStyle().
			Foreground(theme.ColorError).
			Render("●") + " disconnected"
	}

	// Keybinding hints
	hints := "Ctrl+E: Execute │ Tab: Switch pane │ ?: Help │ q: Quit"

	// Message or hints
	right := hints
	if m.message != "" {
		right = m.message
	}

	// Calculate spacing
	leftLen := lipgloss.Width(connIndicator)
	rightLen := len(right)
	padding := m.width - leftLen - rightLen - 4 // borders + spacing
	if padding < 1 {
		padding = 1
	}

	bar := connIndicator + strings.Repeat(" ", padding) + right

	return style.Render(bar)
}
