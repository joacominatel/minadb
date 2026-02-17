package results

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) getCellValue() string {
	if m.result == nil || m.cursorY < 0 || m.cursorY >= len(m.result.Rows) {
		return ""
	}
	row := m.result.Rows[m.cursorY]
	if m.cursorX < 0 || m.cursorX >= len(row) {
		return ""
	}
	return row[m.cursorX]
}

func (m Model) getColumnName() string {
	if m.result == nil || m.cursorX < 0 || m.cursorX >= len(m.result.Columns) {
		return ""
	}
	return m.result.Columns[m.cursorX]
}

func (m Model) getCellValueAt(row, col int) string {
	if m.result == nil || row < 0 || row >= len(m.result.Rows) {
		return ""
	}
	r := m.result.Rows[row]
	if col < 0 || col >= len(r) {
		return ""
	}
	return r[col]
}

// --- Copy ---

func (m *Model) doCopyCell() {
	val := m.getCellValue()
	if val == "" {
		m.statusMessage = "Nothing to copy"
		return
	}
	if err := clipboard.WriteAll(val); err != nil {
		m.statusMessage = "Copy failed: " + err.Error()
		return
	}
	m.statusMessage = "Copied: " + truncateStatus(val, 40)
}

func (m *Model) doCopyCellAt(row, col int) {
	val := m.getCellValueAt(row, col)
	if val == "" {
		m.statusMessage = "Nothing to copy"
		return
	}
	if err := clipboard.WriteAll(val); err != nil {
		m.statusMessage = "Copy failed: " + err.Error()
		return
	}
	m.statusMessage = "Copied: " + truncateStatus(val, 40)
}

func (m *Model) doCopyRowJSON() {
	if m.result == nil || m.cursorY < 0 || m.cursorY >= len(m.result.Rows) {
		m.statusMessage = "No row to copy"
		return
	}
	row := m.result.Rows[m.cursorY]
	jsonStr := rowToJSON(m.result.Columns, row)
	if err := clipboard.WriteAll(jsonStr); err != nil {
		m.statusMessage = "Copy failed: " + err.Error()
		return
	}
	m.statusMessage = "Copied row as JSON"
}

func (m *Model) doCopyRowCSV() {
	if m.result == nil || m.cursorY < 0 || m.cursorY >= len(m.result.Rows) {
		m.statusMessage = "No row to copy"
		return
	}
	row := m.result.Rows[m.cursorY]
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write(m.result.Columns)
	_ = w.Write(row)
	w.Flush()
	if err := clipboard.WriteAll(b.String()); err != nil {
		m.statusMessage = "Copy failed: " + err.Error()
		return
	}
	m.statusMessage = "Copied row as CSV"
}

func (m *Model) doCopyRowText() {
	if m.result == nil || m.cursorY < 0 || m.cursorY >= len(m.result.Rows) {
		m.statusMessage = "No row to copy"
		return
	}
	row := m.result.Rows[m.cursorY]
	if err := clipboard.WriteAll(strings.Join(row, "\t")); err != nil {
		m.statusMessage = "Copy failed: " + err.Error()
		return
	}
	m.statusMessage = "Copied row as text"
}

// --- Filter ---

func (m *Model) doFilterByValue() tea.Cmd {
	col := m.getColumnName()
	val := m.getCellValue()
	table := extractTableName(m.lastQuery)
	if col == "" || table == "" {
		m.statusMessage = "Cannot filter: no cell selected"
		return nil
	}

	var condition string
	if val == "null" {
		condition = col + " IS NULL"
	} else {
		escaped := strings.ReplaceAll(val, "'", "''")
		condition = fmt.Sprintf("%s = '%s'", col, escaped)
	}
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s", table, condition)

	return func() tea.Msg {
		return SetEditorQueryMsg{Query: query}
	}
}

// --- Delete ---

func (m *Model) doGenerateDelete() tea.Cmd {
	if m.result == nil || m.cursorY < 0 || m.cursorY >= len(m.result.Rows) {
		return nil
	}
	table := extractTableName(m.lastQuery)
	row := m.result.Rows[m.cursorY]

	var conditions []string
	for i, col := range m.result.Columns {
		if i >= len(row) {
			break
		}
		val := row[i]
		if val == "null" {
			conditions = append(conditions, col+" IS NULL")
		} else {
			escaped := strings.ReplaceAll(val, "'", "''")
			conditions = append(conditions, fmt.Sprintf("%s = '%s'", col, escaped))
		}
	}

	// send to editor for review, never auto-execute deletes
	query := fmt.Sprintf("-- review before executing!\nDELETE FROM %s WHERE %s",
		table, strings.Join(conditions, " AND "))

	return func() tea.Msg {
		return SetEditorQueryMsg{Query: query}
	}
}

// --- Export ---

func (m Model) exportJSONCmd() tea.Cmd {
	result := m.result
	if result == nil {
		return nil
	}
	return func() tea.Msg {
		ts := time.Now().Format("20060102_150405")
		filename := fmt.Sprintf("minadb_export_%s.json", ts)

		var b strings.Builder
		b.WriteString("[\n")
		for ri, row := range result.Rows {
			if ri > 0 {
				b.WriteString(",\n")
			}
			b.WriteString("  ")
			b.WriteString(rowToJSON(result.Columns, row))
		}
		b.WriteString("\n]")

		if err := os.WriteFile(filename, []byte(b.String()), 0644); err != nil {
			return StatusNotifyMsg{Message: "Export failed: " + err.Error()}
		}
		return StatusNotifyMsg{Message: fmt.Sprintf("Exported %d rows to %s", len(result.Rows), filename)}
	}
}

func (m Model) exportCSVCmd() tea.Cmd {
	result := m.result
	if result == nil {
		return nil
	}
	return func() tea.Msg {
		ts := time.Now().Format("20060102_150405")
		filename := fmt.Sprintf("minadb_export_%s.csv", ts)

		f, err := os.Create(filename)
		if err != nil {
			return StatusNotifyMsg{Message: "Export failed: " + err.Error()}
		}
		defer f.Close()

		w := csv.NewWriter(f)
		_ = w.Write(result.Columns)
		for _, row := range result.Rows {
			_ = w.Write(row)
		}
		w.Flush()

		if err := w.Error(); err != nil {
			return StatusNotifyMsg{Message: "Export failed: " + err.Error()}
		}
		return StatusNotifyMsg{Message: fmt.Sprintf("Exported %d rows to %s", len(result.Rows), filename)}
	}
}

// --- Helpers ---

func extractTableName(query string) string {
	if query == "" {
		return "<table>"
	}
	tokens := strings.Fields(query)
	upper := make([]string, len(tokens))
	for i, t := range tokens {
		upper[i] = strings.ToUpper(t)
	}
	for i, tok := range upper {
		if (tok == "FROM" || tok == "INTO" || tok == "UPDATE") && i+1 < len(tokens) {
			name := tokens[i+1]
			name = strings.TrimRight(name, ";,()")
			if name != "" {
				return name
			}
		}
	}
	return "<table>"
}

// rowToJSON preserves column order unlike map marshaling
func rowToJSON(columns []string, row []string) string {
	var b strings.Builder
	b.WriteString("{")
	for i, col := range columns {
		if i > 0 {
			b.WriteString(", ")
		}
		key, _ := json.Marshal(col)
		b.WriteString(string(key))
		b.WriteString(": ")
		if i < len(row) {
			if row[i] == "null" {
				b.WriteString("null")
			} else {
				val, _ := json.Marshal(row[i])
				b.WriteString(string(val))
			}
		} else {
			b.WriteString("null")
		}
	}
	b.WriteString("}")
	return b.String()
}

func truncateStatus(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
