package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/tildaslashalef/mindnest/internal/app"
	"github.com/tildaslashalef/mindnest/internal/commands"
)

// Version information - populated at build time
var (
	Version    = "dev"
	BuildTime  = "unknown"
	CommitHash = "unknown"
	Author     = "unknown"
	Email      = "unknown"
)

var (
	globalFlags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "staged",
			Aliases: []string{"s"},
			Usage:   "Review staged changes in git repository (default: true if no other review mode is specified)",
			Value:   false,
		},
		&cli.StringFlag{
			Name:    "commit",
			Aliases: []string{"c"},
			Usage:   "Review changes in a specific commit",
		},
		&cli.StringFlag{
			Name:    "branch",
			Aliases: []string{"b"},
			Usage:   "Target branch to review (compares changes from base-branch to this branch)",
		},
		&cli.StringFlag{
			Name:    "base-branch",
			Aliases: []string{"bb"},
			Usage:   "Source branch for comparison (the branch to compare against, default: 'main')",
			Value:   "main",
		},
	}
)

func main() {
	cliApp := &cli.App{
		Name:  "mindnest",
		Usage: "LLM-powered code review tool",
		Description: "Mindnest is an AI-powered code review tool that helps improve code quality.\n\n" +
			"When run without subcommands, Mindnest performs a code review (default action).\n" +
			"Additional subcommands provide workspace management and synchronization features.",
		Version: Version,
		Compiled: func() time.Time {
			t, err := time.Parse(time.RFC3339, BuildTime)
			if err != nil {
				return time.Now()
			}
			return t
		}(),
		Authors: []*cli.Author{
			{
				Name:  Author,
				Email: Email,
			},
		},
		Flags: globalFlags,
		Before: func(c *cli.Context) error {
			// Initialize the application
			application, err := app.New()
			if err != nil {
				return fmt.Errorf("failed to initialize application: %w", err)
			}

			// Store the app instance in the context for later use
			c.App.Metadata = map[string]interface{}{
				"app": application,
			}

			return nil
		},
		After: func(c *cli.Context) error {
			// Gracefully shutdown the application
			if app, ok := c.App.Metadata["app"].(*app.App); ok {
				return app.Shutdown()
			}
			return nil
		},
		Commands: []*cli.Command{
			commands.ReviewCommand(),
			commands.WorkspaceCommand(),
			commands.MigrateCommand(),
			commands.SyncCommand(),
		},
		Action: func(c *cli.Context) error {
			// Default action is to run the review command
			return commands.ReviewCommand().Action(c)
		},
	}

	if err := cliApp.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
