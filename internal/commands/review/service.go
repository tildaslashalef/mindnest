package review

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
	// Note: ReviewOptions and NewModel are defined within this package now
	return s.RunWithOptions(ctx, ReviewOptions{
		TargetDir: cwd,
		AbsPath:   cwd,
		Staged:    false, // Default behavior clarified in RunWithOptions
	})
}

// RunWithOptions starts the TUI with specific options
func (s *Service) RunWithOptions(ctx context.Context, options ReviewOptions) error {
	// Ensure staged is true by default if no other review mode is specified
	// This logic seems slightly redundant if the default in Run is false,
	// but keeping original behavior. Might need review later.
	if options.CommitHash == "" && options.Branch == "" && !options.Staged {
		options.Staged = true
	}

	// Create model with the application
	model := NewModel(s.app) // NewModel is defined in model.go

	// Set review options
	model.SetReviewOptions(options) // SetReviewOptions is defined in model.go

	// Create the program with the model with full mouse support
	p := tea.NewProgram(
		model, // model is a tea.Model
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Enable mouse motion tracking for text selection
		//tea.WithMouseAllMotion(),  // Enable all mouse motion events
	)

	// Start the program
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}
