package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joacominatel/minadb/internal/app"
	"github.com/joacominatel/minadb/internal/config"
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

// AppMode tracks the current UI state.
type AppMode int

const (
	ModeSelectConnection AppMode = iota // show saved connections list
	ModeConnect                         // manual DSN input
	ModeMain                            // main TUI
)

// Custom messages for async operations.
type (
	connectedMsg struct {
		dsn string
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
	connectionSavedMsg struct {
		err error
	}
)

// Model is the top-level bubbletea model orchestrating all components.
type Model struct {
	service    *app.Service
	cfg        *config.Config
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

	// Connection selection
	connCursor int
	connDSN    string // the DSN used for current connection (for saving)
}

// NewModel creates the top-level model.
func NewModel(service *app.Service, cfg *config.Config, dsn string) Model {
	ti := textinput.New()
	ti.Placeholder = "postgresql://user:password@localhost:5432/dbname?sslmode=disable"
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 70

	// Decide initial mode
	mode := ModeConnect
	if dsn == "" && len(cfg.Connections) > 0 {
		mode = ModeSelectConnection
	}

	m := Model{
		service:    service,
		cfg:        cfg,
		explorer:   explorer.New(),
		editor:     editor.New(),
		results:    results.New(),
		statusbar:  statusbar.New(),
		connInput:  ti,
		activePane: PaneExplorer,
		mode:       mode,
		initialDSN: dsn,
	}

	return m
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		textinput.Blink,
	}

	// If a DSN was provided via flag, connect immediately
	if m.initialDSN != "" {
		cmds = append(cmds, m.connectCmd(m.initialDSN))
	}

	return tea.Batch(cmds...)
}

// Update handles all messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Check for explorer column requests
	if schema, table, ok := explorer.IsRequestColumnsMsg(msg); ok {
		return m, m.loadColumnsCmd(schema, table)
	}

	// Handle quick query from explorer
	if qm, ok := msg.(explorer.QuickQueryMsg); ok {
		m.editor.SetQuery(qm.Query)
		m.results.SetLoading(true)
		m.statusbar.SetMessage("Executing query...")
		return m, m.executeQueryCmd(qm.Query)
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

		// Help toggle
		if msg.String() == "?" && m.mode == ModeMain && m.activePane != PaneEditor {
			m.showHelp = !m.showHelp
			return m, nil
		}

		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		// Mode-specific key handling
		switch m.mode {
		case ModeSelectConnection:
			return m.updateSelectConnection(msg)
		case ModeConnect:
			return m.updateConnect(msg)
		case ModeMain:
			return m.updateMain(msg)
		}

	case connectedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.statusbar.SetMessage("Connection failed: " + msg.err.Error())
			return m, nil
		}
		m.connDSN = msg.dsn
		m.mode = ModeMain
		m.err = nil
		m.explorer.SetLoading(true)
		m.statusbar.SetConnected(true, m.service.DatabaseName())
		m.setFocus(PaneExplorer)
		m.layout()

		// Save connection in background
		cmds := []tea.Cmd{m.loadSchemaCmd()}
		if msg.dsn != "" {
			cmds = append(cmds, m.saveConnectionCmd(msg.dsn))
		}
		return m, tea.Batch(cmds...)

	case connectionSavedMsg:
		if msg.err != nil {
			m.statusbar.SetMessage("Warning: could not save connection")
		}
		return m, nil

	case schemaLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.explorer.SetLoading(false)
			m.statusbar.SetMessage("Failed to load schema: " + msg.err.Error())
			return m, nil
		}
		m.explorer.SetTree(msg.tree)
		m.statusbar.SetMessage("")
		// Cache table names for editor autocompletion
		tableNames := m.service.AllTableNames(msg.tree)
		m.editor.SetTableNames(tableNames)
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

func (m Model) updateSelectConnection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	connCount := len(m.cfg.Connections)

	switch msg.String() {
	case "up", "k":
		if m.connCursor > 0 {
			m.connCursor--
		}
	case "down", "j":
		if m.connCursor < connCount { // connCount = last item is "New connection"
			m.connCursor++
		}
	case "enter":
		if m.connCursor < connCount {
			// Selected a saved connection
			conn := m.cfg.Connections[m.connCursor]
			m.statusbar.SetMessage("Connecting to " + conn.Name + "...")
			return m, m.connectCmd(conn.DSN())
		}
		// "New connection" selected
		m.mode = ModeConnect
		m.connInput.Focus()
		return m, nil
	case "n":
		m.mode = ModeConnect
		m.connInput.Focus()
		return m, nil
	case "q":
		return m, tea.Quit
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
	case "esc":
		if len(m.cfg.Connections) > 0 {
			m.mode = ModeSelectConnection
			return m, nil
		}
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
		if m.activePane != PaneEditor {
			return m, tea.Quit
		}
	case "tab":
		if m.activePane == PaneEditor && m.editor.CompletionActive() {
			return m.updateComponents(msg)
		}
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

	explorerWidth := m.width / 4
	if explorerWidth < 22 {
		explorerWidth = 22
	}
	if explorerWidth > 35 {
		explorerWidth = 35
	}

	rightWidth := m.width - explorerWidth - 1

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
		return connectedMsg{dsn: dsn, err: err}
	}
}

func (m Model) saveConnectionCmd(dsn string) tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		conn, err := config.ParseDSN(dsn)
		if err != nil {
			return connectionSavedMsg{err: err}
		}
		err = config.SaveConnection(cfg, conn)
		return connectionSavedMsg{err: err}
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

	switch m.mode {
	case ModeSelectConnection:
		return m.viewSelectConnection()
	case ModeConnect:
		return m.viewConnect()
	default:
		return m.viewMain()
	}
}

func (m Model) viewSelectConnection() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(theme.ColorPrimary).
		Bold(true).
		Padding(1, 0)
	subtitleStyle := lipgloss.NewStyle().Foreground(theme.ColorMuted)

	title := titleStyle.Render("minadb")
	subtitle := subtitleStyle.Render("Fast. Private. Terminal-native.")

	sectionTitle := lipgloss.NewStyle().
		Foreground(theme.ColorPrimary).
		Bold(true).
		Render("Saved Connections")

	var items []string
	for i, conn := range m.cfg.Connections {
		label := fmt.Sprintf("  %s (%s)", conn.Name, conn.DisplayString())
		if i == m.connCursor {
			label = lipgloss.NewStyle().
				Foreground(theme.ColorHighlight).
				Bold(true).
				Render("> " + conn.Name + " (" + conn.DisplayString() + ")")
		}
		items = append(items, label)
	}

	// "New connection" option
	newLabel := "  [New Connection]"
	if m.connCursor == len(m.cfg.Connections) {
		newLabel = lipgloss.NewStyle().
			Foreground(theme.ColorHighlight).
			Bold(true).
			Render("> [New Connection]")
	}
	items = append(items, "")
	items = append(items, newLabel)

	var errMsg string
	if m.err != nil {
		errMsg = "\n" + theme.StyleError.Render("  Error: "+m.err.Error())
	}

	hints := theme.StyleMuted.Render("  ↑/↓: Navigate  Enter: Connect  n: New  q: Quit")

	parts := []string{
		"",
		title,
		subtitle,
		"",
		sectionTitle,
	}
	parts = append(parts, items...)
	if errMsg != "" {
		parts = append(parts, errMsg)
	}
	parts = append(parts, "", hints)

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
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

	backHint := ""
	if len(m.cfg.Connections) > 0 {
		backHint = "  Esc: Back │ "
	}
	hint := theme.StyleMuted.Render("  " + backHint + "Enter: Connect │ Ctrl+C: Quit")

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
	availHeight := m.height - statusHeight - 2

	explorerView := explorerBorder.
		Width(explorerWidth - 2).
		Height(availHeight).
		Render(m.explorer.View())

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
		keyStyle.Render("  s")+"             "+descStyle.Render("Quick SELECT * LIMIT 100"),
		keyStyle.Render("  d")+"             "+descStyle.Render("Count rows"),
		"",
		sectionStyle.Render("Editor"),
		keyStyle.Render("  Ctrl+E / F5")+"   "+descStyle.Render("Execute query"),
		keyStyle.Render("  Ctrl+K")+"        "+descStyle.Render("Clear editor"),
		keyStyle.Render("  Ctrl+L")+"        "+descStyle.Render("Format query (uppercase keywords)"),
		keyStyle.Render("  Auto")+"          "+descStyle.Render("Keywords uppercase on space/newline/;"),
		keyStyle.Render("  Ctrl+Space")+"    "+descStyle.Render("Open autocomplete"),
		keyStyle.Render("  ↑/↓ Enter Tab")+" "+descStyle.Render("Navigate/accept completion"),
		keyStyle.Render("  Esc")+"           "+descStyle.Render("Cancel completion"),
		"",
		sectionStyle.Render("Results"),
		keyStyle.Render("  ↑/k  ↓/j")+"     "+descStyle.Render("Scroll results"),
		keyStyle.Render("  ←/h  →/l")+"     "+descStyle.Render("Scroll visible columns"),
		keyStyle.Render("  PgUp/PgDn")+"     "+descStyle.Render("Page up/down"),
		"",
		theme.StyleMuted.Render("Press any key to close"),
	)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		help,
	)
}
