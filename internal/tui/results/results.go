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

// ViewMode tracks what the results pane is currently showing
type ViewMode int

const (
	ViewNormal        ViewMode = iota
	ViewRecordDetail           // vertical single-record view
	ViewCopyRowPrompt          // format picker for copy row
	ViewExportPrompt           // format picker for export
	ViewDeleteConfirm          // red delete warning
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
	cursorX   int
	colOffset int
	loading   bool
	colWidths []int

	viewMode      ViewMode
	menuCursor    int    // field selector in record detail
	lastQuery     string // SQL that produced the current result
	statusMessage string // temporary feedback
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
	m.cursorX = 0
	m.colOffset = 0
	m.loading = false
	m.viewMode = ViewNormal
	m.menuCursor = 0
	m.statusMessage = ""
	m.calculateColumnWidths()
}

// SetError sets an error to display.
func (m *Model) SetError(err error) {
	m.err = err
	m.result = nil
	m.scrollY = 0
	m.cursorY = 0
	m.cursorX = 0
	m.colOffset = 0
	m.loading = false
	m.viewMode = ViewNormal
	m.menuCursor = 0
	m.statusMessage = ""
}

// SetLastQuery stores the SQL that produced the current result.
func (m *Model) SetLastQuery(q string) {
	m.lastQuery = q
}

// HasResult reports whether there's a result with columns.
func (m Model) HasResult() bool {
	return m.result != nil && len(m.result.Columns) > 0
}

func (m *Model) calculateColumnWidths() {
	if m.result == nil || len(m.result.Columns) == 0 {
		m.colWidths = nil
		return
	}

	m.colWidths = make([]int, len(m.result.Columns))
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

// ── Update ──────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.statusMessage = ""
		switch m.viewMode {
		case ViewRecordDetail:
			return m.updateRecordDetail(msg)
		case ViewCopyRowPrompt:
			return m.updateCopyRowPrompt(msg)
		case ViewExportPrompt:
			return m.updateExportPrompt(msg)
		case ViewDeleteConfirm:
			return m.updateDeleteConfirm(msg)
		default:
			return m.updateNormal(msg)
		}
	}

	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	// navigation
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
	case "left", "h":
		if m.result != nil && m.cursorX > 0 {
			m.cursorX--
			m.ensureHorizontalWindow()
		}
	case "right", "l":
		if m.result != nil && m.cursorX < len(m.result.Columns)-1 {
			m.cursorX++
			m.ensureHorizontalWindow()
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
	case "home":
		m.cursorX = 0
		m.ensureHorizontalWindow()
	case "end":
		if m.result != nil && len(m.result.Columns) > 0 {
			m.cursorX = len(m.result.Columns) - 1
			m.ensureHorizontalWindow()
		}
	case "g":
		m.cursorY = 0
		m.ensureVerticalWindow()
	case "G":
		if m.result != nil && m.result.RowCount > 0 {
			m.cursorY = m.result.RowCount - 1
			m.ensureVerticalWindow()
		}

	// actions
	case "c":
		m.doCopyCell()
	case "y":
		if m.HasResult() {
			m.viewMode = ViewCopyRowPrompt
		}
	case "e":
		if m.HasResult() {
			m.viewMode = ViewExportPrompt
		}
	case "D":
		if m.HasResult() {
			m.viewMode = ViewDeleteConfirm
		}
	case "enter":
		if m.HasResult() {
			m.viewMode = ViewRecordDetail
			m.menuCursor = m.cursorX
		}
	case "f":
		if m.HasResult() {
			cmd := m.doFilterByValue()
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) updateRecordDetail(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewMode = ViewNormal
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.result != nil && m.menuCursor < len(m.result.Columns)-1 {
			m.menuCursor++
		}
	case "c":
		m.doCopyCellAt(m.cursorY, m.menuCursor)
	case "f":
		if m.result != nil && m.menuCursor < len(m.result.Columns) {
			origX := m.cursorX
			m.cursorX = m.menuCursor
			cmd := m.doFilterByValue()
			m.cursorX = origX
			m.viewMode = ViewNormal
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) updateCopyRowPrompt(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewMode = ViewNormal
	case "j":
		m.doCopyRowJSON()
		m.viewMode = ViewNormal
	case "c":
		m.doCopyRowCSV()
		m.viewMode = ViewNormal
	case "t":
		m.doCopyRowText()
		m.viewMode = ViewNormal
	}
	return m, nil
}

func (m Model) updateExportPrompt(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewMode = ViewNormal
	case "j":
		m.viewMode = ViewNormal
		m.statusMessage = "Exporting JSON..."
		return m, m.exportJSONCmd()
	case "c":
		m.viewMode = ViewNormal
		m.statusMessage = "Exporting CSV..."
		return m, m.exportCSVCmd()
	}
	return m, nil
}

func (m Model) updateDeleteConfirm(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		m.viewMode = ViewNormal
	case "y", "enter":
		m.viewMode = ViewNormal
		cmd := m.doGenerateDelete()
		return m, cmd
	}
	return m, nil
}

// ── View ────────────────────────────────────────────────────────────────

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

	stats := fmt.Sprintf("%d row(s) | %s",
		m.result.RowCount,
		m.result.Duration.Round(time.Microsecond).String(),
	)
	header := titleStyle.Render("Results") + "  " +
		theme.StyleMuted.Render(stats)

	if len(m.result.Columns) == 0 {
		return header + "\n" + theme.StyleSuccess.Render("  Query executed successfully")
	}

	if m.viewMode == ViewRecordDetail {
		return header + "\n" + m.renderRecordDetail()
	}

	return header + "\n" + m.renderTableView()
}

func (m Model) renderTableView() string {
	var b strings.Builder

	fromCol, toCol := m.visibleColumnRange()
	tableCols := m.result.Columns[fromCol:toCol]
	widths := slices.Clone(m.colWidths[fromCol:toCol])

	activeCol := -1
	if m.cursorX >= fromCol && m.cursorX < toCol {
		activeCol = m.cursorX - fromCol
	}

	b.WriteString(m.renderTopBorder(widths))
	b.WriteString("\n")
	b.WriteString(m.renderRow(tableCols, widths, true, false, activeCol))
	b.WriteString("\n")
	b.WriteString(m.renderSeparator(widths))
	b.WriteString("\n")

	visibleRows := m.visibleRows()
	rowEnd := min(len(m.result.Rows), m.scrollY+visibleRows)
	for i := m.scrollY; i < rowEnd; i++ {
		line := m.renderRow(m.result.Rows[i][fromCol:toCol], widths, false, i == m.cursorY, activeCol)
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString(m.renderBottomBorder(widths))
	b.WriteString("\n")

	switch m.viewMode {
	case ViewCopyRowPrompt:
		b.WriteString(m.renderCopyRowPrompt())
	case ViewExportPrompt:
		b.WriteString(m.renderExportPrompt())
	case ViewDeleteConfirm:
		b.WriteString(m.renderDeleteConfirm())
	default:
		b.WriteString(m.renderNormalFooter(fromCol, toCol, visibleRows))
	}

	return b.String()
}

func (m Model) renderRow(cells []string, widths []int, isHeader bool, selected bool, activeCol int) string {
	var b strings.Builder

	sepStyle := lipgloss.NewStyle()
	if selected && !isHeader {
		sepStyle = sepStyle.Background(lipgloss.Color("236"))
	}

	b.WriteString(sepStyle.Render("│"))

	for i, cell := range cells {
		if i > 0 {
			b.WriteString(sepStyle.Render("│"))
		}

		width := widths[i]
		display := fitCell(cell, width)
		content := " " + display + " "

		style := lipgloss.NewStyle()
		switch {
		case isHeader && activeCol >= 0 && i == activeCol:
			style = style.Bold(true).Foreground(theme.ColorPrimary).Underline(true)
		case isHeader:
			style = style.Bold(true).Foreground(theme.ColorPrimary)
		case selected && activeCol >= 0 && i == activeCol:
			style = style.Background(theme.ColorPrimary).
				Foreground(lipgloss.Color("255")).Bold(true)
		case selected:
			style = style.Background(lipgloss.Color("236"))
		}

		b.WriteString(style.Render(content))
	}

	b.WriteString(sepStyle.Render("│"))
	return b.String()
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

func (m *Model) ensureHorizontalWindow() {
	if m.result == nil || len(m.result.Columns) == 0 {
		return
	}
	if m.cursorX < 0 {
		m.cursorX = 0
	}
	if m.cursorX >= len(m.result.Columns) {
		m.cursorX = len(m.result.Columns) - 1
	}

	// scroll left if cursor went past the left edge
	if m.cursorX < m.colOffset {
		m.colOffset = m.cursorX
	}

	// scroll right until cursor is visible
	for m.colOffset < len(m.result.Columns) {
		_, toCol := m.visibleColumnRange()
		if m.cursorX < toCol {
			break
		}
		m.colOffset++
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

// ── Footer / Overlays ───────────────────────────────────────────────────

func (m Model) renderNormalFooter(fromCol, toCol, visibleRows int) string {
	if m.result == nil {
		return ""
	}

	colInfo := fmt.Sprintf("Col %d/%d", m.cursorX+1, len(m.result.Columns))
	rowInfo := fmt.Sprintf("Row %d/%d", m.cursorY+1, m.result.RowCount)

	if m.statusMessage != "" {
		return theme.StyleSuccess.Render("  "+m.statusMessage) + "  " +
			theme.StyleMuted.Render(colInfo+" | "+rowInfo)
	}

	actions := "c:copy  y:row  e:export  f:filter  D:delete  Enter:detail"
	return theme.StyleMuted.Render(colInfo + " | " + rowInfo + " | " + actions)
}

func (m Model) renderRecordDetail() string {
	if m.result == nil || m.cursorY < 0 || m.cursorY >= len(m.result.Rows) {
		return ""
	}

	row := m.result.Rows[m.cursorY]

	recTitle := lipgloss.NewStyle().
		Foreground(theme.ColorHighlight).
		Bold(true).
		Render(fmt.Sprintf("  Record %d of %d", m.cursorY+1, m.result.RowCount))

	var b strings.Builder
	b.WriteString(recTitle)
	b.WriteString("\n")

	// col widths for the two-column layout
	nameWidth := 10
	for _, col := range m.result.Columns {
		w := lipgloss.Width(col)
		if w > nameWidth {
			nameWidth = w
		}
	}
	if nameWidth > 25 {
		nameWidth = 25
	}

	valueWidth := m.width - nameWidth - 7
	if valueWidth < 20 {
		valueWidth = 20
	}

	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorBorder)
	topBorder := borderStyle.Render(
		"┌" + strings.Repeat("─", nameWidth+2) + "┬" + strings.Repeat("─", valueWidth+2) + "┐")
	btmBorder := borderStyle.Render(
		"└" + strings.Repeat("─", nameWidth+2) + "┴" + strings.Repeat("─", valueWidth+2) + "┘")

	b.WriteString(topBorder)
	b.WriteString("\n")

	visible := m.height - 7
	if visible < 1 {
		visible = 1
	}

	scrollOff := 0
	if m.menuCursor >= scrollOff+visible {
		scrollOff = m.menuCursor - visible + 1
	}
	if m.menuCursor < scrollOff {
		scrollOff = m.menuCursor
	}

	endIdx := min(len(m.result.Columns), scrollOff+visible)
	for i := scrollOff; i < endIdx; i++ {
		col := m.result.Columns[i]
		val := ""
		if i < len(row) {
			val = row[i]
		}

		nameDisplay := fitCell(col, nameWidth)
		valDisplay := fitCell(val, valueWidth)

		nameContent := " " + nameDisplay + " "
		valContent := " " + valDisplay + " "

		if i == m.menuCursor {
			nameContent = lipgloss.NewStyle().
				Background(theme.ColorPrimary).
				Foreground(lipgloss.Color("255")).
				Bold(true).
				Render(nameContent)
			valContent = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Render(valContent)
		}

		b.WriteString("│" + nameContent + "│" + valContent + "│")
		b.WriteString("\n")
	}

	b.WriteString(btmBorder)
	b.WriteString("\n")

	if m.statusMessage != "" {
		b.WriteString(theme.StyleSuccess.Render("  " + m.statusMessage))
		b.WriteString("  ")
	}
	b.WriteString(theme.StyleMuted.Render("c:copy | f:filter | ↑/↓ navigate | Esc close"))

	return b.String()
}

func (m Model) renderCopyRowPrompt() string {
	label := lipgloss.NewStyle().
		Foreground(theme.ColorHighlight).
		Bold(true).
		Render("Copy row as: ")
	opts := theme.StyleMuted.Render("[j]JSON  [c]CSV  [t]Text  [Esc]cancel")
	return label + opts
}

func (m Model) renderExportPrompt() string {
	label := lipgloss.NewStyle().
		Foreground(theme.ColorHighlight).
		Bold(true).
		Render("Export results: ")
	opts := theme.StyleMuted.Render("[j]JSON file  [c]CSV file  [Esc]cancel")
	return label + opts
}

func (m Model) renderDeleteConfirm() string {
	warning := lipgloss.NewStyle().
		Foreground(theme.ColorError).
		Bold(true).
		Render("⚠ DELETE this record? ")
	hint := theme.StyleMuted.Render("Sends to editor for review. [y]Yes [Esc]Cancel")
	return warning + hint
}

// ── Cell helpers ────────────────────────────────────────────────────────

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
