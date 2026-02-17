package results

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joacominatel/minadb/internal/database"
	"github.com/joacominatel/minadb/internal/tui/theme"
)

// Model is the query results component.
type Model struct {
	result   *database.QueryResult
	err      error
	width    int
	height   int
	focused  bool
	scrollY  int
	loading  bool
	colWidths []int
}

// New creates a new results model.
func New() Model {
	return Model{}
}

// SetSize updates the component dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetFocused sets the focus state.
func (m *Model) SetFocused(f bool) {
	m.focused = f
}

// Focused returns whether the results pane has focus.
func (m Model) Focused() bool {
	return m.focused
}

// SetLoading sets the loading state.
func (m *Model) SetLoading(l bool) {
	m.loading = l
}

// SetResult sets the query result to display.
func (m *Model) SetResult(r *database.QueryResult) {
	m.result = r
	m.err = nil
	m.scrollY = 0
	m.loading = false
	m.calculateColumnWidths()
}

// SetError sets an error to display.
func (m *Model) SetError(err error) {
	m.err = err
	m.result = nil
	m.scrollY = 0
	m.loading = false
}

func (m *Model) calculateColumnWidths() {
	if m.result == nil || len(m.result.Columns) == 0 {
		m.colWidths = nil
		return
	}

	m.colWidths = make([]int, len(m.result.Columns))
	for i, col := range m.result.Columns {
		m.colWidths[i] = len(col)
	}

	for _, row := range m.result.Rows {
		for i, cell := range row {
			if i < len(m.colWidths) && len(cell) > m.colWidths[i] {
				m.colWidths[i] = len(cell)
			}
		}
	}

	// Cap column widths to prevent overflow
	maxColWidth := 40
	for i := range m.colWidths {
		if m.colWidths[i] > maxColWidth {
			m.colWidths[i] = maxColWidth
		}
	}
}

// Init returns the initial command (none).
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the results pane.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.scrollY > 0 {
				m.scrollY--
			}
		case "down", "j":
			if m.result != nil && m.scrollY < m.result.RowCount-1 {
				m.scrollY++
			}
		case "pgup":
			m.scrollY -= m.height / 2
			if m.scrollY < 0 {
				m.scrollY = 0
			}
		case "pgdown":
			if m.result != nil {
				m.scrollY += m.height / 2
				maxScroll := m.result.RowCount - 1
				if m.scrollY > maxScroll {
					m.scrollY = maxScroll
				}
				if m.scrollY < 0 {
					m.scrollY = 0
				}
			}
		}
	}

	return m, nil
}

// View renders the results pane.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(theme.ColorPrimary).
		Bold(true).
		Padding(0, 1)

	if m.loading {
		return titleStyle.Render("Results") + "\n" + theme.StyleMuted.Render("  Executing query...")
	}

	if m.err != nil {
		return titleStyle.Render("Results") + "\n" +
			theme.StyleError.Render("  Error: "+m.err.Error())
	}

	if m.result == nil {
		return titleStyle.Render("Results") + "\n" +
			theme.StyleMuted.Render("  Execute a query to see results")
	}

	// Header with stats
	stats := fmt.Sprintf("%d row(s) | %s",
		m.result.RowCount,
		m.result.Duration.Round(1000).String(),
	)
	header := titleStyle.Render("Results") + "  " +
		theme.StyleMuted.Render(stats)

	if len(m.result.Columns) == 0 {
		return header + "\n" + theme.StyleSuccess.Render("  Query executed successfully")
	}

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")

	// Render table header
	headerLine := m.renderRow(m.result.Columns, true)
	b.WriteString(headerLine)
	b.WriteString("\n")

	// Separator
	sep := m.renderSeparator()
	b.WriteString(sep)
	b.WriteString("\n")

	// Visible rows
	visibleRows := m.height - 4 // header + title + separator + padding
	if visibleRows < 1 {
		visibleRows = 1
	}

	for i := m.scrollY; i < len(m.result.Rows) && i < m.scrollY+visibleRows; i++ {
		line := m.renderRow(m.result.Rows[i], false)
		b.WriteString(line)
		if i < m.scrollY+visibleRows-1 && i < len(m.result.Rows)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderRow(cells []string, isHeader bool) string {
	parts := make([]string, len(cells))
	for i, cell := range cells {
		width := 10
		if i < len(m.colWidths) {
			width = m.colWidths[i]
		}

		// Truncate cell content
		display := cell
		if len(display) > width {
			display = display[:width-1] + "…"
		}

		// Pad to width
		display = display + strings.Repeat(" ", width-len(display))

		if isHeader {
			parts[i] = lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.ColorPrimary).
				Render(display)
		} else {
			parts[i] = display
		}
	}
	return "  " + strings.Join(parts, " │ ")
}

func (m Model) renderSeparator() string {
	parts := make([]string, len(m.colWidths))
	for i, w := range m.colWidths {
		parts[i] = strings.Repeat("─", w)
	}
	return "  " + lipgloss.NewStyle().Foreground(theme.ColorBorder).Render(strings.Join(parts, "─┼─"))
}
