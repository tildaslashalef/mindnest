package tui

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tildaslashalef/mindnest/internal/app"
)

// Service is the main service for the TUI
type Service struct {
	app *app.App
}

// NewService creates a new TUI service
func NewService(application *app.App) *Service {
	return &Service{
		app: application,
	}
}

// Run starts the TUI with default options
func (s *Service) Run(ctx context.Context) error {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Use default options (staged changes, current directory)
	return s.RunWithOptions(ctx, ReviewOptions{
		TargetDir: cwd,
		AbsPath:   cwd,
		Staged:    false,
	})
}

// RunWithOptions starts the TUI with specific options
func (s *Service) RunWithOptions(ctx context.Context, options ReviewOptions) error {
	// Ensure staged is true by default if no other review mode is specified
	if options.CommitHash == "" && options.Branch == "" {
		options.Staged = true
	}
	// Create model with the application
	model := NewModel(s.app)

	// Set review options
	model.SetReviewOptions(options)

	// Create the program with the model with full mouse support
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Enable mouse motion tracking for text selection
		tea.WithMouseAllMotion(),  // Enable all mouse motion events
	)

	// Start the program
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}
