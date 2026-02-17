package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joacominatel/minadb/internal/app"
	"github.com/joacominatel/minadb/internal/config"
	"github.com/joacominatel/minadb/internal/database/postgres"
	"github.com/joacominatel/minadb/internal/tui"
)

func main() {
	dsn := flag.String("dsn", "", "PostgreSQL connection string (e.g. postgresql://user:pass@localhost/db)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = &config.Config{}
	}

	// Determine DSN: flag > config default (only if --dsn provided)
	connDSN := *dsn

	// Set up dependencies
	driver := postgres.New()
	service := app.NewService(driver)

	// Create and run TUI
	// Pass config so the TUI can show saved connections and save new ones
	model := tui.NewModel(service, cfg, connDSN)
	p := tea.NewProgram(model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	finalModel, err := p.Run()
	if err != nil {
		log.Fatalf("Error running program: %v", err)
	}

	// Graceful cleanup
	_ = service.Disconnect()
	_ = finalModel
}
