package results

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joacominatel/minadb/internal/database"
	"github.com/joacominatel/minadb/internal/tui/theme"
)

// Model is the query results component.
type Model struct {
	result    *database.QueryResult
	err       error
	width     int
	height    int
	focused   bool
	scrollY   int
	cursorY   int
	colOffset int
	loading   bool
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
	m.cursorY = 0
	m.colOffset = 0
	m.loading = false
	m.calculateColumnWidths()
}

// SetError sets an error to display.
func (m *Model) SetError(err error) {
	m.err = err
	m.result = nil
	m.scrollY = 0
	m.cursorY = 0
	m.colOffset = 0
	m.loading = false
}

func (m *Model) calculateColumnWidths() {
	if m.result == nil || len(m.result.Columns) == 0 {
		m.colWidths = nil
		return
	}

	m.colWidths = make([]int, len(m.result.Columns))

	// Use display width (not byte length) for accurate measurement
	for i, col := range m.result.Columns {
		m.colWidths[i] = lipgloss.Width(col)
	}

	for _, row := range m.result.Rows {
		for i, cell := range row {
			w := lipgloss.Width(cell)
			if i < len(m.colWidths) && w > m.colWidths[i] {
				m.colWidths[i] = w
			}
		}
	}

	// Keep columns readable but bounded.
	for i := range m.colWidths {
		if m.colWidths[i] < 8 {
			m.colWidths[i] = 8
		}
		if m.colWidths[i] > 40 {
			m.colWidths[i] = 40
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
			if m.result != nil && m.cursorY > 0 {
				m.cursorY--
				m.ensureVerticalWindow()
			}
		case "down", "j":
			if m.result != nil && m.cursorY < m.result.RowCount-1 {
				m.cursorY++
				m.ensureVerticalWindow()
			}
		case "pgup":
			if m.result != nil {
				step := max(1, m.visibleRows())
				m.cursorY -= step
				if m.cursorY < 0 {
					m.cursorY = 0
				}
				m.ensureVerticalWindow()
			}
		case "pgdown":
			if m.result != nil {
				step := max(1, m.visibleRows())
				m.cursorY += step
				if m.cursorY > m.result.RowCount-1 {
					m.cursorY = m.result.RowCount - 1
				}
				m.ensureVerticalWindow()
			}
		case "left", "h":
			if m.colOffset > 0 {
				m.colOffset--
			}
		case "right", "l":
			if m.result != nil && m.colOffset < len(m.result.Columns)-1 {
				m.colOffset++
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
		m.result.Duration.Round(time.Microsecond).String(),
	)
	header := titleStyle.Render("Results") + "  " +
		theme.StyleMuted.Render(stats)

	if len(m.result.Columns) == 0 {
		return header + "\n" + theme.StyleSuccess.Render("  Query executed successfully")
	}

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")

	fromCol, toCol := m.visibleColumnRange()
	tableCols := m.result.Columns[fromCol:toCol]
	widths := slices.Clone(m.colWidths[fromCol:toCol])

	b.WriteString(m.renderTopBorder(widths))
	b.WriteString("\n")
	b.WriteString(m.renderRow(tableCols, widths, true, false))
	b.WriteString("\n")
	b.WriteString(m.renderSeparator(widths))
	b.WriteString("\n")

	visibleRows := m.visibleRows()
	rowEnd := min(len(m.result.Rows), m.scrollY+visibleRows)
	for i := m.scrollY; i < rowEnd; i++ {
		line := m.renderRow(m.result.Rows[i][fromCol:toCol], widths, false, i == m.cursorY)
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString(m.renderBottomBorder(widths))
	b.WriteString("\n")
	b.WriteString(m.renderFooter(fromCol, toCol, visibleRows))

	return b.String()
}

func (m Model) renderRow(cells []string, widths []int, isHeader bool, selected bool) string {
	parts := make([]string, len(cells))
	for i, cell := range cells {
		width := widths[i]
		display := fitCell(cell, width)
		if isHeader {
			display = lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.ColorPrimary).
				Render(display)
		}
		parts[i] = " " + display + " "
	}
	line := "│" + strings.Join(parts, "│") + "│"
	if selected {
		line = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Render(line)
	}
	return line
}

func (m Model) renderTopBorder(widths []int) string {
	return m.renderBorder("┌", "┬", "┐", widths)
}

func (m Model) renderBottomBorder(widths []int) string {
	return m.renderBorder("└", "┴", "┘", widths)
}

func (m Model) renderSeparator(widths []int) string {
	return m.renderBorder("├", "┼", "┤", widths)
}

func (m Model) renderBorder(left, center, right string, widths []int) string {
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strings.Repeat("─", w+2)
	}
	line := left + strings.Join(parts, center) + right
	return lipgloss.NewStyle().Foreground(theme.ColorBorder).Render(line)
}

func (m Model) visibleRows() int {
	v := m.height - 8
	if v < 1 {
		v = 1
	}
	return v
}

func (m *Model) ensureVerticalWindow() {
	if m.result == nil || m.result.RowCount == 0 {
		m.scrollY = 0
		m.cursorY = 0
		return
	}

	if m.cursorY < 0 {
		m.cursorY = 0
	}
	if m.cursorY >= m.result.RowCount {
		m.cursorY = m.result.RowCount - 1
	}

	visible := m.visibleRows()
	if m.cursorY < m.scrollY {
		m.scrollY = m.cursorY
	}
	if m.cursorY >= m.scrollY+visible {
		m.scrollY = m.cursorY - visible + 1
	}

	maxScroll := max(0, m.result.RowCount-visible)
	if m.scrollY > maxScroll {
		m.scrollY = maxScroll
	}
}

func (m Model) visibleColumnRange() (int, int) {
	if m.result == nil || len(m.result.Columns) == 0 {
		return 0, 0
	}

	if m.colOffset >= len(m.result.Columns) {
		return 0, len(m.result.Columns)
	}

	available := max(20, m.width-4)
	start := max(0, m.colOffset)
	end := start
	total := 1

	for i := start; i < len(m.colWidths); i++ {
		colWidth := m.colWidths[i] + 3
		if end > start && total+colWidth > available {
			break
		}
		total += colWidth
		end = i + 1
	}

	if end == start {
		end = min(start+1, len(m.result.Columns))
	}

	return start, end
}

func (m Model) renderFooter(fromCol, toCol, visibleRows int) string {
	if m.result == nil {
		return ""
	}

	rowStart := 0
	rowEnd := 0
	if m.result.RowCount > 0 {
		rowStart = m.scrollY + 1
		rowEnd = min(m.result.RowCount, m.scrollY+visibleRows)
	}

	colLeft := ""
	colRight := ""
	if fromCol > 0 {
		colLeft = "← "
	}
	if toCol < len(m.result.Columns) {
		colRight = " →"
	}

	colInfo := fmt.Sprintf("%sColumn %d-%d of %d%s", colLeft, fromCol+1, toCol, len(m.result.Columns), colRight)
	rowInfo := fmt.Sprintf("Row %d-%d of %d (selected %d)", rowStart, rowEnd, m.result.RowCount, m.cursorY+1)
	nav := "← → columns | ↑ ↓ rows | PgUp/PgDn pages"

	return theme.StyleMuted.Render(colInfo + " | " + rowInfo + " | " + nav)
}

func fitCell(v string, width int) string {
	if width < 1 {
		width = 1
	}
	v = strings.ReplaceAll(v, "\n", " ")
	v = strings.ReplaceAll(v, "\r", " ")
	v = strings.TrimSpace(v)
	if v == "" {
		v = "null"
	}

	if lipgloss.Width(v) > width {
		v = truncateDisplay(v, width)
	}

	pad := width - lipgloss.Width(v)
	if pad > 0 {
		v += strings.Repeat(" ", pad)
	}
	return v
}

func truncateDisplay(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}

	target := width - 3
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > target {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}
