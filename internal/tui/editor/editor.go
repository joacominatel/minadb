package editor

import (
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

// Model is the SQL query editor component.
type Model struct {
	textarea textarea.Model
	width    int
	height   int
	focused  bool

	// Completion state
	tableNames  []string // cached table names from database
	completing  bool     // in completion mode
	completions []string // current candidates
	compIndex   int      // which candidate is active
	compPartial string   // the partial text that triggered completion
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
			// Format: uppercase SQL keywords
			m.formatKeywords()
			return m, nil

		case "tab":
			// Try table name completion
			if m.tryCompletion() {
				return m, nil
			}
			// If no completions, fall through to textarea

		case "esc":
			if m.completing {
				m.cancelCompletion()
				return m, nil
			}
		}

		// Any key other than Tab/Esc cancels completion mode
		if m.completing && key != "tab" && key != "esc" {
			m.cancelCompletion()
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
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
func (m *Model) tryCompletion() bool {
	if len(m.tableNames) == 0 {
		return false
	}

	val := m.textarea.Value()
	if val == "" {
		return false
	}

	// If already completing, cycle through candidates
	if m.completing && len(m.completions) > 0 {
		m.compIndex = (m.compIndex + 1) % len(m.completions)
		m.applyCompletion()
		return true
	}

	// Find the partial word at the end of the text
	partial := extractLastWord(val)
	if partial == "" {
		return false
	}

	// Check if we're in a completion-worthy context (after FROM, JOIN, etc.)
	upperVal := strings.ToUpper(val)
	inTableContext := strings.Contains(upperVal, "FROM") ||
		strings.Contains(upperVal, "JOIN") ||
		strings.Contains(upperVal, "TABLE") ||
		strings.Contains(upperVal, "INTO")

	if !inTableContext {
		return false
	}

	// Find matching table names
	lower := strings.ToLower(partial)
	var matches []string
	for _, name := range m.tableNames {
		if strings.HasPrefix(strings.ToLower(name), lower) {
			matches = append(matches, name)
		}
	}

	if len(matches) == 0 {
		return false
	}

	m.completing = true
	m.completions = matches
	m.compIndex = 0
	m.compPartial = partial
	m.applyCompletion()
	return true
}

// applyCompletion replaces the partial word with the current completion candidate.
func (m *Model) applyCompletion() {
	if len(m.completions) == 0 {
		return
	}

	val := m.textarea.Value()
	// Remove the partial (or previous completion) from the end
	base := strings.TrimSuffix(val, extractLastWord(val))
	newVal := base + m.completions[m.compIndex]
	m.textarea.SetValue(newVal)
}

func (m *Model) cancelCompletion() {
	m.completing = false
	m.completions = nil
	m.compIndex = 0
	m.compPartial = ""
}

// extractLastWord returns the last word-like token from the text.
func extractLastWord(s string) string {
	s = strings.TrimRight(s, " \t\n\r")
	if s == "" {
		return ""
	}
	i := len(s) - 1
	for i >= 0 && isIdentChar(rune(s[i])) {
		i--
	}
	return s[i+1:]
}

func isIdentChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_' || c == '.'
}

// View renders the editor.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(theme.ColorPrimary).
		Bold(true).
		Padding(0, 1)

	title := titleStyle.Render("Query Editor")

	var completionHint string
	if m.completing && len(m.completions) > 1 {
		hint := make([]string, 0, len(m.completions))
		for i, c := range m.completions {
			if i == m.compIndex {
				hint = append(hint, lipgloss.NewStyle().Foreground(theme.ColorHighlight).Bold(true).Render(c))
			} else {
				hint = append(hint, theme.StyleMuted.Render(c))
			}
		}
		completionHint = "\n" + lipgloss.NewStyle().Padding(0, 1).Render(
			theme.StyleMuted.Render("Tab: ")+strings.Join(hint, " │ "),
		)
	}

	return title + "\n" + m.textarea.View() + completionHint
}
