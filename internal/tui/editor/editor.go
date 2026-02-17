package editor

import (
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joacominatel/minadb/internal/tui/theme"
)

// ExecuteQueryMsg is sent when the user triggers query execution.
type ExecuteQueryMsg struct {
	Query string
}

// SQL keywords for formatting and completion.
var sqlKeywords = map[string]bool{
	"select": true, "from": true, "where": true, "and": true, "or": true,
	"insert": true, "into": true, "update": true, "delete": true,
	"create": true, "drop": true, "alter": true, "table": true,
	"index": true, "join": true, "inner": true, "outer": true,
	"left": true, "right": true, "cross": true, "on": true,
	"not": true, "in": true, "is": true, "null": true, "like": true,
	"order": true, "by": true, "group": true, "having": true,
	"limit": true, "offset": true, "as": true, "distinct": true,
	"count": true, "sum": true, "avg": true, "min": true, "max": true,
	"between": true, "exists": true, "case": true, "when": true,
	"then": true, "else": true, "end": true, "values": true,
	"set": true, "begin": true, "commit": true, "rollback": true,
	"union": true, "all": true, "asc": true, "desc": true,
	"primary": true, "key": true, "foreign": true, "references": true,
	"cascade": true, "restrict": true, "default": true,
	"true": true, "false": true, "ilike": true, "returning": true,
}

var sqlKeywordList = []string{
	"SELECT", "FROM", "WHERE", "AND", "OR", "INSERT", "INTO", "UPDATE", "DELETE",
	"CREATE", "DROP", "ALTER", "TABLE", "INDEX", "JOIN", "INNER", "OUTER", "LEFT",
	"RIGHT", "CROSS", "ON", "NOT", "IN", "IS", "NULL", "LIKE", "ILIKE", "ORDER",
	"BY", "GROUP", "HAVING", "LIMIT", "OFFSET", "AS", "DISTINCT", "COUNT", "SUM",
	"AVG", "MIN", "MAX", "BETWEEN", "EXISTS", "CASE", "WHEN", "THEN", "ELSE", "END",
	"VALUES", "SET", "BEGIN", "COMMIT", "ROLLBACK", "UNION", "ALL", "ASC", "DESC",
	"PRIMARY", "KEY", "FOREIGN", "REFERENCES", "CASCADE", "RESTRICT", "DEFAULT", "RETURNING",
}

// Model is the SQL query editor component.
type Model struct {
	textarea textarea.Model
	width    int
	height   int
	focused  bool

	// Completion state
	tableNames          []string // cached table names from database
	showingCompletions  bool
	completions         []string
	completionIndex     int
	completionStartByte int
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
	m.textarea.SetWidth(w - 2)
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

// SetQuery replaces the editor content.
func (m *Model) SetQuery(query string) {
	m.textarea.SetValue(query)
}

// SetTableNames sets the available table names for autocompletion.
func (m *Model) SetTableNames(names []string) {
	m.tableNames = names
}

// Clear empties the editor.
func (m *Model) Clear() {
	m.textarea.Reset()
	m.cancelCompletion()
}

// CompletionActive reports if completion UI is open.
func (m Model) CompletionActive() bool {
	return m.showingCompletions
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
		if msg.Type == tea.KeyCtrlAt || msg.Type == tea.KeyNull {
			m.openCompletions()
			return m, nil
		}

		key := msg.String()

		switch key {
		case "ctrl+e", "f5":
			query := strings.TrimSpace(m.textarea.Value())
			if query != "" {
				m.cancelCompletion()
				return m, func() tea.Msg {
					return ExecuteQueryMsg{Query: query}
				}
			}
			return m, nil

		case "ctrl+k":
			m.Clear()
			return m, nil

		case "ctrl+l":
			// Manual formatting is still useful for pasted queries.
			m.formatKeywords()
			return m, nil

		case "ctrl+space", "ctrl+@":
			m.openCompletions()
			return m, nil

		case "up":
			if m.showingCompletions && len(m.completions) > 0 {
				if m.completionIndex > 0 {
					m.completionIndex--
				}
				return m, nil
			}

		case "down":
			if m.showingCompletions && len(m.completions) > 0 {
				if m.completionIndex < len(m.completions)-1 {
					m.completionIndex++
				}
				return m, nil
			}

		case "enter", "tab":
			if m.showingCompletions {
				m.acceptCompletion()
				return m, nil
			}

		case "esc":
			if m.showingCompletions {
				m.cancelCompletion()
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		m.afterTextInput(keyMsg)
	}

	return m, cmd
}

func (m *Model) afterTextInput(msg tea.KeyMsg) {
	key := msg.String()

	if key == " " || key == "enter" || key == ";" {
		m.autoUppercaseLastWord()
	}

	if m.showingCompletions {
		m.refreshCompletions()
	}
}

func (m *Model) autoUppercaseLastWord() {
	val := m.textarea.Value()
	runes := []rune(val)
	if len(runes) < 2 {
		return
	}

	i := len(runes) - 1
	if !isWordBoundary(runes[i]) {
		return
	}
	i--
	for i >= 0 && isWordBoundary(runes[i]) {
		i--
	}
	if i < 0 {
		return
	}

	end := i
	for i >= 0 && (unicode.IsLetter(runes[i]) || runes[i] == '_') {
		i--
	}
	start := i + 1
	if start > end {
		return
	}

	word := string(runes[start : end+1])
	if !sqlKeywords[strings.ToLower(word)] {
		return
	}

	runes = append(runes[:start], append([]rune(strings.ToUpper(word)), runes[end+1:]...)...)
	m.textarea.SetValue(string(runes))
}

// formatKeywords uppercases all SQL keywords in the editor content.
func (m *Model) formatKeywords() {
	val := m.textarea.Value()
	if val == "" {
		return
	}

	var result strings.Builder
	word := strings.Builder{}
	inString := false
	quote := rune(0)

	for _, ch := range val {
		// Track string literals
		if (ch == '\'' || ch == '"') && !inString {
			inString = true
			quote = ch
			m.flushWord(&word, &result)
			result.WriteRune(ch)
			continue
		}
		if inString && ch == quote {
			inString = false
			result.WriteRune(ch)
			continue
		}
		if inString {
			result.WriteRune(ch)
			continue
		}

		// Word boundary
		if !unicode.IsLetter(ch) && ch != '_' {
			m.flushWord(&word, &result)
			result.WriteRune(ch)
		} else {
			word.WriteRune(ch)
		}
	}
	m.flushWord(&word, &result)

	m.textarea.SetValue(result.String())
}

func (m *Model) flushWord(word *strings.Builder, result *strings.Builder) {
	if word.Len() == 0 {
		return
	}
	w := word.String()
	if sqlKeywords[strings.ToLower(w)] {
		result.WriteString(strings.ToUpper(w))
	} else {
		result.WriteString(w)
	}
	word.Reset()
}

// tryCompletion attempts table name completion at the current cursor position.
// Returns true if a completion was applied.
func (m *Model) openCompletions() {
	prefix, start := extractTrailingIdentifier(m.textarea.Value())
	suggestions := m.getSuggestions(prefix, start)
	if len(suggestions) == 0 {
		m.cancelCompletion()
		return
	}

	m.showingCompletions = true
	m.completions = suggestions
	m.completionIndex = 0
	m.completionStartByte = start
}

func (m *Model) refreshCompletions() {
	if !m.showingCompletions {
		return
	}
	prefix, start := extractTrailingIdentifier(m.textarea.Value())
	suggestions := m.getSuggestions(prefix, start)
	if len(suggestions) == 0 {
		m.cancelCompletion()
		return
	}
	m.completions = suggestions
	m.completionStartByte = start
	if m.completionIndex >= len(m.completions) {
		m.completionIndex = len(m.completions) - 1
	}
}

func (m *Model) acceptCompletion() {
	if !m.showingCompletions || len(m.completions) == 0 {
		return
	}

	val := m.textarea.Value()
	if m.completionStartByte < 0 || m.completionStartByte > len(val) {
		m.cancelCompletion()
		return
	}

	newVal := val[:m.completionStartByte] + m.completions[m.completionIndex]
	m.textarea.SetValue(newVal)
	m.cancelCompletion()
}

func (m *Model) cancelCompletion() {
	m.showingCompletions = false
	m.completions = nil
	m.completionIndex = 0
	m.completionStartByte = 0
}

func isIdentChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_' || c == '.'
}

func isWordBoundary(c rune) bool {
	return unicode.IsSpace(c) || c == ';' || c == ',' || c == ')' || c == '('
}

func extractTrailingIdentifier(s string) (string, int) {
	if s == "" {
		return "", 0
	}

	i := len(s)
	for i > 0 {
		r := rune(s[i-1])
		if isIdentChar(r) {
			i--
			continue
		}
		break
	}

	return s[i:], i
}

func (m Model) getSuggestions(prefix string, start int) []string {
	prefixLower := strings.ToLower(prefix)
	prefixUpper := strings.ToUpper(prefix)

	context := strings.TrimSpace(strings.ToUpper(m.textarea.Value()[:start]))
	last := lastSQLToken(context)

	candidates := make([]string, 0, len(sqlKeywordList)+len(m.tableNames)+1)

	switch last {
	case "FROM", "JOIN", "INTO", "UPDATE", "TABLE":
		candidates = append(candidates, m.tableNames...)
	case "SELECT":
		candidates = append(candidates, "*")
		candidates = append(candidates, sqlKeywordList...)
		candidates = append(candidates, m.tableNames...)
	default:
		candidates = append(candidates, sqlKeywordList...)
		candidates = append(candidates, m.tableNames...)
	}

	seen := make(map[string]struct{}, len(candidates))
	filtered := make([]string, 0, 10)
	for _, c := range candidates {
		if c == "" {
			continue
		}
		k := strings.ToLower(c)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}

		if prefixLower == "" || strings.HasPrefix(strings.ToLower(c), prefixLower) || strings.HasPrefix(strings.ToUpper(c), prefixUpper) {
			filtered = append(filtered, c)
		}
	}

	sort.Strings(filtered)
	if len(filtered) > 10 {
		filtered = filtered[:10]
	}

	return filtered
}

func lastSQLToken(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// View renders the editor.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(theme.ColorPrimary).
		Bold(true).
		Padding(0, 1)

	title := titleStyle.Render("Query Editor")

	editorView := title + "\n" + m.textarea.View()

	if !m.showingCompletions || len(m.completions) == 0 {
		return editorView
	}

	return lipgloss.JoinVertical(lipgloss.Left, editorView, m.renderCompletionDropdown())
}

func (m Model) renderCompletionDropdown() string {
	itemStyle := lipgloss.NewStyle().Padding(0, 1)
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(theme.ColorHighlight).
		Bold(true).
		Padding(0, 1)

	items := make([]string, 0, len(m.completions))
	for i, c := range m.completions {
		if i == m.completionIndex {
			items = append(items, selectedStyle.Render(c))
		} else {
			items = append(items, itemStyle.Render(c))
		}
	}

	dropdown := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Padding(0, 0).
		MaxWidth(max(20, m.width-4)).
		Render(lipgloss.JoinVertical(lipgloss.Left, items...))

	hint := theme.StyleMuted.Render("  ↑/↓ navigate | Enter/Tab accept | Esc cancel")

	return lipgloss.JoinVertical(lipgloss.Left, dropdown, hint)
}
