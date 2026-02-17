package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joacominatel/minadb/internal/app"
	"github.com/joacominatel/minadb/internal/database"
	"github.com/joacominatel/minadb/internal/tui/editor"
	"github.com/joacominatel/minadb/internal/tui/explorer"
	"github.com/joacominatel/minadb/internal/tui/results"
	"github.com/joacominatel/minadb/internal/tui/statusbar"
	"github.com/joacominatel/minadb/internal/tui/theme"
)

// Pane identifies a focusable area.
type Pane int

const (
	PaneExplorer Pane = iota
	PaneEditor
	PaneResults
)

func (p Pane) String() string {
	switch p {
	case PaneExplorer:
		return "explorer"
	case PaneEditor:
		return "editor"
	case PaneResults:
		return "results"
	default:
		return "unknown"
	}
}

// AppMode tracks whether we're connecting or in the main UI.
type AppMode int

const (
	ModeConnect AppMode = iota
	ModeMain
)

// Custom messages for async operations.
type (
	connectedMsg struct {
		err error
	}
	schemaLoadedMsg struct {
		tree *app.SchemaTree
		err  error
	}
	queryExecutedMsg struct {
		result *database.QueryResult
		err    error
	}
	columnsLoadedMsg struct {
		schema  string
		table   string
		columns []database.Column
		err     error
	}
)

// Model is the top-level bubbletea model orchestrating all components.
type Model struct {
	service    *app.Service
	explorer   explorer.Model
	editor     editor.Model
	results    results.Model
	statusbar  statusbar.Model
	connInput  textinput.Model
	activePane Pane
	mode       AppMode
	width      int
	height     int
	err        error
	showHelp   bool
	initialDSN string
}

// NewModel creates the top-level model.
func NewModel(service *app.Service, dsn string) Model {
	ti := textinput.New()
	ti.Placeholder = "postgresql://user:password@localhost:5432/dbname?sslmode=disable"
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 70

	m := Model{
		service:    service,
		explorer:   explorer.New(),
		editor:     editor.New(),
		results:    results.New(),
		statusbar:  statusbar.New(),
		connInput:  ti,
		activePane: PaneExplorer,
		mode:       ModeConnect,
		initialDSN: dsn,
	}

	return m
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		textinput.Blink,
	}

	// If a DSN was provided, connect immediately
	if m.initialDSN != "" {
		cmds = append(cmds, m.connectCmd(m.initialDSN))
	}

	return tea.Batch(cmds...)
}

// Update handles all messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Check for explorer column requests (from async commands)
	if schema, table, ok := explorer.IsRequestColumnsMsg(msg); ok {
		return m, m.loadColumnsCmd(schema, table)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case tea.KeyMsg:
		// Global keys
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}

		// Help toggle (not in connect mode, and not when editor is focused)
		if msg.String() == "?" && m.mode == ModeMain && m.activePane != PaneEditor {
			m.showHelp = !m.showHelp
			return m, nil
		}

		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		// Mode-specific key handling
		if m.mode == ModeConnect {
			return m.updateConnect(msg)
		}

		return m.updateMain(msg)

	case connectedMsg:
		if msg.err != nil {
			m.err = msg.err
			if m.mode == ModeConnect {
				m.statusbar.SetMessage("Connection failed: " + msg.err.Error())
			}
			return m, nil
		}
		m.mode = ModeMain
		m.err = nil
		m.explorer.SetLoading(true)
		m.statusbar.SetConnected(true, m.service.DatabaseName())
		m.setFocus(PaneExplorer)
		m.layout()
		return m, m.loadSchemaCmd()

	case schemaLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.explorer.SetLoading(false)
			m.statusbar.SetMessage("Failed to load schema: " + msg.err.Error())
			return m, nil
		}
		m.explorer.SetTree(msg.tree)
		m.statusbar.SetMessage("")
		return m, nil

	case queryExecutedMsg:
		m.results.SetLoading(false)
		if msg.err != nil {
			m.results.SetError(msg.err)
			m.statusbar.SetMessage("")
			return m, nil
		}
		m.results.SetResult(msg.result)
		m.statusbar.SetMessage("")
		return m, nil

	case columnsLoadedMsg:
		if msg.err != nil {
			m.statusbar.SetMessage("Failed to load columns: " + msg.err.Error())
			return m, nil
		}
		m.explorer.SetColumns(msg.schema, msg.table, msg.columns)
		return m, nil

	case editor.ExecuteQueryMsg:
		m.results.SetLoading(true)
		m.statusbar.SetMessage("Executing query...")
		return m, m.executeQueryCmd(msg.Query)
	}

	// Pass through to active component
	if m.mode == ModeMain {
		return m.updateComponents(msg)
	}

	return m, nil
}

func (m Model) updateConnect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		dsn := strings.TrimSpace(m.connInput.Value())
		if dsn != "" {
			m.statusbar.SetMessage("Connecting...")
			return m, m.connectCmd(dsn)
		}
		return m, nil
	case "q":
		if m.connInput.Value() == "" {
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.connInput, cmd = m.connInput.Update(msg)
	return m, cmd
}

func (m Model) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		// Only quit from explorer or results, not while typing in editor
		if m.activePane != PaneEditor {
			return m, tea.Quit
		}
	case "tab":
		m.cyclePane()
		return m, nil
	case "shift+tab":
		m.cyclePaneBack()
		return m, nil
	}

	return m.updateComponents(msg)
}

func (m Model) updateComponents(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.activePane {
	case PaneExplorer:
		m.explorer, cmd = m.explorer.Update(msg)
	case PaneEditor:
		m.editor, cmd = m.editor.Update(msg)
	case PaneResults:
		m.results, cmd = m.results.Update(msg)
	}

	return m, cmd
}

func (m *Model) cyclePane() {
	switch m.activePane {
	case PaneExplorer:
		m.setFocus(PaneEditor)
	case PaneEditor:
		m.setFocus(PaneResults)
	case PaneResults:
		m.setFocus(PaneExplorer)
	}
}

func (m *Model) cyclePaneBack() {
	switch m.activePane {
	case PaneExplorer:
		m.setFocus(PaneResults)
	case PaneEditor:
		m.setFocus(PaneExplorer)
	case PaneResults:
		m.setFocus(PaneEditor)
	}
}

func (m *Model) setFocus(pane Pane) {
	m.activePane = pane
	m.explorer.SetFocused(pane == PaneExplorer)
	m.editor.SetFocused(pane == PaneEditor)
	m.results.SetFocused(pane == PaneResults)
	m.statusbar.SetActivePane(pane.String())
}

func (m *Model) layout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	statusHeight := 1
	availHeight := m.height - statusHeight

	// Explorer takes ~25% width, min 22, max 35
	explorerWidth := m.width / 4
	if explorerWidth < 22 {
		explorerWidth = 22
	}
	if explorerWidth > 35 {
		explorerWidth = 35
	}

	rightWidth := m.width - explorerWidth - 1

	// Editor takes 40% of available height
	editorHeight := availHeight * 40 / 100
	if editorHeight < 5 {
		editorHeight = 5
	}
	resultsHeight := availHeight - editorHeight - 1

	m.explorer.SetSize(explorerWidth, availHeight)
	m.editor.SetSize(rightWidth, editorHeight)
	m.results.SetSize(rightWidth, resultsHeight)
	m.statusbar.SetWidth(m.width)
}

// Async commands

func (m Model) connectCmd(dsn string) tea.Cmd {
	service := m.service
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := service.Connect(ctx, dsn)
		return connectedMsg{err: err}
	}
}

func (m Model) loadSchemaCmd() tea.Cmd {
	service := m.service
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		tree, err := service.LoadSchemaTree(ctx)
		return schemaLoadedMsg{tree: tree, err: err}
	}
}

func (m Model) executeQueryCmd(query string) tea.Cmd {
	service := m.service
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := service.ExecuteQuery(ctx, query)
		return queryExecutedMsg{result: result, err: err}
	}
}

func (m Model) loadColumnsCmd(schema, table string) tea.Cmd {
	service := m.service
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		columns, err := service.LoadColumns(ctx, schema, table)
		return columnsLoadedMsg{schema: schema, table: table, columns: columns, err: err}
	}
}

// View renders the entire application.
func (m Model) View() string {
	if m.showHelp {
		return m.viewHelp()
	}

	if m.mode == ModeConnect {
		return m.viewConnect()
	}

	return m.viewMain()
}

func (m Model) viewConnect() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(theme.ColorPrimary).
		Bold(true).
		Padding(1, 0)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(theme.ColorMuted)

	title := titleStyle.Render("minadb")
	subtitle := subtitleStyle.Render("Fast. Private. Terminal-native.")

	promptStyle := lipgloss.NewStyle().
		Foreground(theme.ColorPrimary)
	prompt := promptStyle.Render("Enter connection string:")

	var errMsg string
	if m.err != nil {
		errMsg = "\n" + theme.StyleError.Render("  Error: "+m.err.Error())
	}

	hint := theme.StyleMuted.Render("  Press Enter to connect, Ctrl+C to quit")

	content := lipgloss.JoinVertical(lipgloss.Left,
		"",
		title,
		subtitle,
		"",
		prompt,
		"  "+m.connInput.View(),
		errMsg,
		"",
		hint,
	)

	// Center on screen
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m Model) viewMain() string {
	explorerBorder := theme.StyleBorder
	if m.activePane == PaneExplorer {
		explorerBorder = theme.StyleActiveBorder
	}

	explorerWidth := m.width / 4
	if explorerWidth < 22 {
		explorerWidth = 22
	}
	if explorerWidth > 35 {
		explorerWidth = 35
	}

	rightWidth := m.width - explorerWidth - 1

	statusHeight := 1
	availHeight := m.height - statusHeight - 2 // account for borders

	explorerView := explorerBorder.
		Width(explorerWidth - 2).
		Height(availHeight).
		Render(m.explorer.View())

	// Right pane: editor on top, results on bottom
	editorHeight := availHeight * 40 / 100
	if editorHeight < 5 {
		editorHeight = 5
	}
	resultsHeight := availHeight - editorHeight - 2

	editorBorder := theme.StyleBorder
	if m.activePane == PaneEditor {
		editorBorder = theme.StyleActiveBorder
	}
	editorView := editorBorder.
		Width(rightWidth - 2).
		Height(editorHeight).
		Render(m.editor.View())

	resultsBorder := theme.StyleBorder
	if m.activePane == PaneResults {
		resultsBorder = theme.StyleActiveBorder
	}
	resultsView := resultsBorder.
		Width(rightWidth - 2).
		Height(resultsHeight).
		Render(m.results.View())

	rightPane := lipgloss.JoinVertical(lipgloss.Left,
		editorView,
		resultsView,
	)

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top,
		explorerView,
		rightPane,
	)

	statusView := m.statusbar.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		mainArea,
		statusView,
	)
}

func (m Model) viewHelp() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(theme.ColorPrimary).
		Bold(true)

	sectionStyle := lipgloss.NewStyle().
		Foreground(theme.ColorHighlight).
		Bold(true)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	descStyle := lipgloss.NewStyle().
		Foreground(theme.ColorMuted)

	help := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("minadb - Keyboard Shortcuts"),
		"",
		sectionStyle.Render("Global"),
		keyStyle.Render("  q / Ctrl+C")+"    "+descStyle.Render("Quit application"),
		keyStyle.Render("  Tab")+"           "+descStyle.Render("Switch between panes"),
		keyStyle.Render("  Shift+Tab")+"     "+descStyle.Render("Switch panes (reverse)"),
		keyStyle.Render("  ?")+"             "+descStyle.Render("Toggle this help"),
		"",
		sectionStyle.Render("Explorer"),
		keyStyle.Render("  ↑/k  ↓/j")+"     "+descStyle.Render("Navigate up/down"),
		keyStyle.Render("  Enter/→/l")+"     "+descStyle.Render("Expand item"),
		keyStyle.Render("  ←/h")+"           "+descStyle.Render("Collapse item"),
		"",
		sectionStyle.Render("Editor"),
		keyStyle.Render("  Ctrl+E / F5")+"   "+descStyle.Render("Execute query"),
		keyStyle.Render("  Ctrl+K")+"        "+descStyle.Render("Clear editor"),
		"",
		sectionStyle.Render("Results"),
		keyStyle.Render("  ↑/k  ↓/j")+"     "+descStyle.Render("Scroll results"),
		keyStyle.Render("  PgUp/PgDn")+"     "+descStyle.Render("Page up/down"),
		"",
		theme.StyleMuted.Render("Press any key to close"),
	)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		help,
	)
}
