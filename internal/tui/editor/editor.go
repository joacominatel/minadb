package editor

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joacominatel/minadb/internal/tui/theme"
)

// ExecuteQueryMsg is sent when the user triggers query execution.
type ExecuteQueryMsg struct {
	Query string
}

// Model is the SQL query editor component.
type Model struct {
	textarea textarea.Model
	width    int
	height   int
	focused  bool
}

// New creates a new editor model.
func New() Model {
	ta := textarea.New()
	ta.Placeholder = "Enter SQL query..."
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // unlimited
	ta.Prompt = "│ "
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(theme.ColorMuted)
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(theme.ColorMuted)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(theme.ColorPrimary)
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(theme.ColorBorder)

	return Model{
		textarea: ta,
	}
}

// SetSize updates the component dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.textarea.SetWidth(w - 2) // Account for border padding
	m.textarea.SetHeight(h - 2)
}

// SetFocused sets the focus state.
func (m *Model) SetFocused(f bool) {
	m.focused = f
	if f {
		m.textarea.Focus()
	} else {
		m.textarea.Blur()
	}
}

// Focused returns whether the editor has focus.
func (m Model) Focused() bool {
	return m.focused
}

// Value returns the current editor content.
func (m Model) Value() string {
	return m.textarea.Value()
}

// Clear empties the editor.
func (m *Model) Clear() {
	m.textarea.Reset()
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages for the editor.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+e", "f5":
			query := strings.TrimSpace(m.textarea.Value())
			if query != "" {
				return m, func() tea.Msg {
					return ExecuteQueryMsg{Query: query}
				}
			}
			return m, nil
		case "ctrl+k":
			m.Clear()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// View renders the editor.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(theme.ColorPrimary).
		Bold(true).
		Padding(0, 1)

	title := titleStyle.Render("Query Editor")

	return title + "\n" + m.textarea.View()
}
