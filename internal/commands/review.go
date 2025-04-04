package commands

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/tildaslashalef/mindnest/internal/app"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/tui"
)

// ReviewCommand returns the CLI command for the TUI interface
func ReviewCommand() *cli.Command {
	return &cli.Command{
		Name:        "review",
		Usage:       "Start the interactive code review interface",
		Hidden:      true,
		Description: "Launch Mindnest in interactive TUI mode, for visual code review",
		Action:      reviewAction,
	}
}

// ReviewCommand is the main action function for the TUI command
func reviewAction(c *cli.Context) error {
	// Get application instance
	application, err := app.FromContext(c)
	if err != nil {
		return err
	}

	// Log that we're starting the TUI
	loggy.Info("Starting TUI mode")

	// Get flags from context
	staged := c.Bool("staged")
	commitHash := c.String("commit")
	branch := c.String("branch")
	baseBranch := c.String("base-branch")

	// Ensure staged is true by default when no other review mode is specified
	if commitHash == "" && branch == "" {
		staged = true
	}

	// Get current directory path
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Create TUI service
	tuiService := tui.NewService(application)

	// Set review options
	reviewOptions := tui.ReviewOptions{
		TargetDir:  cwd,
		Staged:     staged,
		CommitHash: commitHash,
		Branch:     branch,
		BaseBranch: baseBranch,
		AbsPath:    cwd, // Current directory is already absolute
	}

	// Run the TUI service with review options
	return tuiService.RunWithOptions(c.Context, reviewOptions)
}
