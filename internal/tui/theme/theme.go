package theme

import "github.com/charmbracelet/lipgloss"

// Color palette — minimalist, terminal-friendly.
var (
	ColorPrimary   = lipgloss.Color("63")  // Purple
	ColorSecondary = lipgloss.Color("241") // Gray
	ColorSuccess   = lipgloss.Color("42")  // Green
	ColorError     = lipgloss.Color("196") // Red
	ColorBorder    = lipgloss.Color("238") // Dark gray
	ColorMuted     = lipgloss.Color("245") // Light gray
	ColorHighlight = lipgloss.Color("229") // Yellow
)

// Shared styles used across TUI components.
var (
	StyleBorder = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder)

	StyleActiveBorder = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary)

	StyleTitle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	StyleMuted = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleError = lipgloss.NewStyle().
			Foreground(ColorError)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	StyleStatusBar = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)
)
