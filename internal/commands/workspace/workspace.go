package workspace

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urfave/cli/v2"

	"github.com/tildaslashalef/mindnest/internal/app"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/utils"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// WorkspaceCommand returns the workspace command
func WorkspaceCommand() *cli.Command {
	return &cli.Command{
		Name:    "workspace",
		Aliases: []string{"ws"},
		Usage:   "Show workspace details in an interactive UI",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "Name of the workspace to show (defaults to workspace in current directory)",
			},
			&cli.StringFlag{
				Name:    "github-url",
				Aliases: []string{"g"},
				Usage:   "Set or update GitHub repository URL for the workspace (e.g., https://github.com/owner/repo)",
			},
			&cli.BoolFlag{
				Name:    "tui",
				Aliases: []string{"t"},
				Usage:   "Show the interactive TUI after command execution (default: true)",
				Value:   true,
			},
		},
		Action: workspaceShowAction,
	}
}

// workspaceShowAction handles the workspace show command logic
// It now delegates the TUI part to the workspace package
func workspaceShowAction(c *cli.Context) error {
	application, err := app.FromContext(c)
	if err != nil {
		return fmt.Errorf("failed to get application from context: %w", err)
	}

	ctx := context.Background()
	name := c.String("name")
	githubURL := c.String("github-url")
	showTUI := c.Bool("tui")
	var ws *workspace.Workspace

	if name != "" {
		workspaces, listErr := application.Workspace.ListWorkspaces(ctx)
		if listErr != nil {
			return fmt.Errorf("failed to list workspaces: %w", listErr)
		}
		var matchingWorkspaces []*workspace.Workspace
		for _, w := range workspaces {
			if strings.Contains(strings.ToLower(w.Name), strings.ToLower(name)) {
				matchingWorkspaces = append(matchingWorkspaces, w)
			}
			if w.Name == name {
				ws = w
				break
			}
		}
		if ws == nil && len(matchingWorkspaces) > 0 {
			ws = matchingWorkspaces[0]
		}
		if ws == nil {
			utils.PrintError(fmt.Sprintf("No workspace found matching name: %s", name))
			return fmt.Errorf("no workspace found matching name: %s", name)
		}
	} else {
		currentDir, dirErr := os.Getwd()
		if dirErr != nil {
			return fmt.Errorf("failed to get current directory: %w", dirErr)
		}
		ws, err = application.Workspace.GetWorkspaceByPath(ctx, currentDir)
		if err != nil {
			utils.PrintError(fmt.Sprintf("Failed to get workspace for current directory: %v", err))
			return fmt.Errorf("failed to get workspace for current directory: %w", err)
		}
	}

	if ws == nil {
		// This case should ideally be caught above, but added for safety
		utils.PrintError("No workspace found.")
		return fmt.Errorf("no workspace found")
	}

	loggy.Info("Found workspace", "id", ws.ID, "name", ws.Name, "path", ws.Path)

	if githubURL != "" {
		if !strings.HasPrefix(githubURL, "http://") && !strings.HasPrefix(githubURL, "https://") && !strings.HasPrefix(githubURL, "git@") {
			msg := fmt.Sprintf("Invalid GitHub URL format: %s (should start with http://, https://, or git@)", githubURL)
			utils.PrintError(msg)
			return fmt.Errorf("%s", msg)
		}
		if _, _, urlErr := application.GitHub.ExtractRepoDetailsFromURL(githubURL); urlErr != nil {
			msg := fmt.Sprintf("Unable to extract repository details from URL: %v", urlErr)
			utils.PrintError(msg)
			return fmt.Errorf("%s", msg)
		}
		ws.SetGitRepoURL(githubURL)
		if updateErr := application.Workspace.UpdateWorkspace(ctx, ws); updateErr != nil {
			msg := fmt.Sprintf("Failed to update workspace with new GitHub URL: %v", updateErr)
			utils.PrintError(msg)
			return fmt.Errorf("%s", msg)
		}
		utils.PrintSuccess(fmt.Sprintf("Updated workspace '%s' with GitHub URL: %s", ws.Name, githubURL))
		// If ONLY updating URL and TUI not explicitly requested, exit.
		if !showTUI {
			loggy.Debug("GitHub URL updated and TUI not requested, exiting.")
			return nil
		}
	}

	if !showTUI {
		// If TUI is explicitly disabled (even if URL wasn't updated), just print info.
		// TODO: Implement a simple non-TUI printout of workspace info?
		utils.PrintInfo(fmt.Sprintf("Workspace Name: %s", ws.Name))
		utils.PrintInfo(fmt.Sprintf("Workspace Path: %s", ws.Path))
		utils.PrintInfo(fmt.Sprintf("GitHub URL: %s", ws.GitRepoURL))
		utils.PrintInfo(fmt.Sprintf("Workspace ID: %s", ws.ID))
		loggy.Debug("TUI disabled, exiting after printing info.")
		return nil
	}

	// Create the TUI model from the new workspace package
	model := NewModel(application, ws)

	// Create the Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen()) // Add mouse options if needed

	loggy.Debug("Starting workspace TUI", "workspace_id", ws.ID, "name", ws.Name)

	// Run the TUI
	if _, runErr := p.Run(); runErr != nil {
		// Log the error from the TUI run itself
		loggy.Error("Error running workspace TUI", "error", runErr)
		return fmt.Errorf("failed to run workspace UI: %w", runErr)
	}

	loggy.Debug("Workspace TUI finished.")
	return nil
}
