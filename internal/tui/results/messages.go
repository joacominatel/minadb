package results

// SetEditorQueryMsg tells the app to put a query in the editor pane
type SetEditorQueryMsg struct {
	Query string
}

// StatusNotifyMsg tells the app to show a message in the status bar
type StatusNotifyMsg struct {
	Message string
}
