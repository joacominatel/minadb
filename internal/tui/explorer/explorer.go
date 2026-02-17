package explorer

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joacominatel/minadb/internal/app"
	"github.com/joacominatel/minadb/internal/database"
	"github.com/joacominatel/minadb/internal/tui/theme"
)

// NodeKind identifies the type of a tree node.
type NodeKind int

const (
	NodeDatabase NodeKind = iota
	NodeSchema
	NodeTable
	NodeColumn
)

// TreeNode represents a single node in the schema tree.
type TreeNode struct {
	Kind     NodeKind
	Name     string
	Children []*TreeNode
	Expanded bool
	Loaded   bool // whether children have been fetched

	// Metadata
	Schema   string // parent schema name (for tables/columns)
	Table    string // parent table name (for columns)
	DataType string // column data type
	RowCount int64  // table row count
}

// flatItem is a visible item in the flattened tree view.
type flatItem struct {
	node  *TreeNode
	depth int
}

// Model is the explorer (schema tree) component.
type Model struct {
	tree    *TreeNode
	items   []flatItem
	cursor  int
	width   int
	height  int
	focused bool
	loading bool
}

// New creates a new explorer model.
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

// Focused returns whether the explorer has focus.
func (m Model) Focused() bool {
	return m.focused
}

// SetLoading sets the loading state.
func (m *Model) SetLoading(l bool) {
	m.loading = l
}

// ColumnsLoadedMsg signals that columns have been loaded for a table.
type ColumnsLoadedMsg struct {
	Schema  string
	Table   string
	Columns []database.Column
	Err     error
}

// SetTree populates the explorer from a schema tree.
func (m *Model) SetTree(schema *app.SchemaTree) {
	root := &TreeNode{
		Kind:     NodeDatabase,
		Name:     schema.Database,
		Expanded: true,
		Loaded:   true,
	}

	for _, s := range schema.Schemas {
		schemaNode := &TreeNode{
			Kind:     NodeSchema,
			Name:     s.Name,
			Expanded: false,
			Loaded:   true,
		}
		for _, t := range s.Tables {
			tableNode := &TreeNode{
				Kind:   NodeTable,
				Name:   t,
				Schema: s.Name,
				Loaded: false,
			}
			schemaNode.Children = append(schemaNode.Children, tableNode)
		}
		root.Children = append(root.Children, schemaNode)
	}

	m.tree = root
	m.flatten()
	m.loading = false
}

// SetColumns adds column nodes to a table node.
func (m *Model) SetColumns(schema, table string, columns []database.Column) {
	if m.tree == nil {
		return
	}
	m.visitTable(schema, table, func(node *TreeNode) {
		node.Children = nil
		for _, col := range columns {
			node.Children = append(node.Children, &TreeNode{
				Kind:     NodeColumn,
				Name:     col.Name,
				Schema:   schema,
				Table:    table,
				DataType: col.DataType,
			})
		}
		node.Loaded = true
	})
	m.flatten()
}

func (m *Model) visitTable(schema, table string, fn func(*TreeNode)) {
	if m.tree == nil {
		return
	}
	for _, s := range m.tree.Children {
		if s.Name != schema {
			continue
		}
		for _, t := range s.Children {
			if t.Name == table {
				fn(t)
				return
			}
		}
	}
}

// SelectedTable returns the schema and table name of the currently selected table node, if any.
func (m Model) SelectedTable() (schema, table string, ok bool) {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return "", "", false
	}
	node := m.items[m.cursor].node
	switch node.Kind {
	case NodeTable:
		return node.Schema, node.Name, true
	case NodeColumn:
		return node.Schema, node.Table, true
	}
	return "", "", false
}

// flatten rebuilds the flat item list from the tree.
func (m *Model) flatten() {
	m.items = nil
	if m.tree != nil {
		m.flattenNode(m.tree, 0)
	}
	if m.cursor >= len(m.items) {
		m.cursor = max(0, len(m.items)-1)
	}
}

func (m *Model) flattenNode(node *TreeNode, depth int) {
	m.items = append(m.items, flatItem{node: node, depth: depth})
	if node.Expanded {
		for _, child := range node.Children {
			m.flattenNode(child, depth+1)
		}
	}
}

// Init returns the initial command (none).
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the explorer.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter", "right", "l":
			return m, m.toggleExpand()
		case "left", "h":
			return m, m.collapse()
		}
	}

	return m, nil
}

func (m *Model) toggleExpand() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	node := m.items[m.cursor].node

	// Columns have no children
	if node.Kind == NodeColumn {
		return nil
	}

	if node.Expanded {
		node.Expanded = false
		m.flatten()
		return nil
	}

	node.Expanded = true
	m.flatten()

	// If this is a table and columns aren't loaded yet, request them
	if node.Kind == NodeTable && !node.Loaded {
		schema := node.Schema
		table := node.Name
		return func() tea.Msg {
			return requestColumnsMsg{Schema: schema, Table: table}
		}
	}

	return nil
}

func (m *Model) collapse() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	node := m.items[m.cursor].node

	if node.Expanded {
		node.Expanded = false
		m.flatten()
	}
	return nil
}

// requestColumnsMsg is sent when a table is expanded and needs column data.
type requestColumnsMsg struct {
	Schema string
	Table  string
}

// RequestColumnsMsg returns the message type for external use.
func IsRequestColumnsMsg(msg tea.Msg) (schema, table string, ok bool) {
	if m, ok := msg.(requestColumnsMsg); ok {
		return m.Schema, m.Table, true
	}
	return "", "", false
}

// View renders the explorer.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(theme.ColorPrimary).
		Bold(true).
		Padding(0, 1)

	title := titleStyle.Render("Schema Explorer")

	if m.loading {
		return title + "\n" + theme.StyleMuted.Render("  Loading...")
	}

	if m.tree == nil {
		return title + "\n" + theme.StyleMuted.Render("  No connection")
	}

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")

	// Calculate visible area
	visibleHeight := m.height - 2 // title + padding
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	// Scroll offset to keep cursor visible
	scrollOffset := 0
	if m.cursor >= visibleHeight {
		scrollOffset = m.cursor - visibleHeight + 1
	}

	for i := scrollOffset; i < len(m.items) && i < scrollOffset+visibleHeight; i++ {
		item := m.items[i]
		line := m.renderNode(item, i == m.cursor)
		b.WriteString(line)
		if i < scrollOffset+visibleHeight-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderNode(item flatItem, selected bool) string {
	node := item.node
	indent := strings.Repeat("  ", item.depth)

	var icon string
	switch node.Kind {
	case NodeDatabase:
		if node.Expanded {
			icon = "▼ "
		} else {
			icon = "▶ "
		}
	case NodeSchema:
		if node.Expanded {
			icon = "▼ "
		} else {
			icon = "▶ "
		}
	case NodeTable:
		if node.Expanded {
			icon = "▼ "
		} else {
			icon = "▶ "
		}
	case NodeColumn:
		icon = "  "
	}

	name := node.Name
	if node.Kind == NodeColumn && node.DataType != "" {
		name = fmt.Sprintf("%s %s", node.Name, lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(node.DataType))
	}

	line := indent + icon + name

	// Truncate to width
	if m.width > 0 && lipgloss.Width(line) > m.width-2 {
		line = line[:m.width-4] + ".."
	}

	if selected {
		return lipgloss.NewStyle().
			Foreground(theme.ColorHighlight).
			Bold(true).
			Render(line)
	}

	return line
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
