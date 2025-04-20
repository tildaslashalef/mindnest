package workspace

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tildaslashalef/mindnest/internal/github"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	ws "github.com/tildaslashalef/mindnest/internal/workspace"
)

// loadWorkspaceData creates a command to load issues for the current workspace.
func loadWorkspaceData(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background() // Or use m.ctx if context needs cancellation
		loggy.Debug("Loading workspace data command started", "workspace_id", m.workspace.ID)

		if m.workspace == nil {
			loggy.Error("Workspace is nil during data load")
			return LoadDataMsg{Error: fmt.Errorf("workspace not initialized")}
		}

		// Load workspace issues using the application service
		issues, err := m.app.Workspace.GetWorkspaceIssues(ctx, m.workspace.ID)
		if err != nil {
			loggy.Error("Failed to load workspace issues", "workspace_id", m.workspace.ID, "error", err)
			return LoadDataMsg{Error: fmt.Errorf("failed to load workspace issues: %w", err)}
		}

		loggy.Info("Workspace data loaded successfully", "workspace_id", m.workspace.ID, "issue_count", len(issues))
		return LoadDataMsg{
			Workspace: m.workspace, // Pass workspace back if needed, though model already has it
			Issues:    issues,
			Error:     nil,
		}
	}
}

// toggleIssueStatus creates a command to update the IsValid status of an issue.
func toggleIssueStatus(m Model, issueID string, isValid bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background() // Or use m.ctx
		loggy.Debug("Toggling issue status command started", "issue_id", issueID, "set_valid", isValid)

		var err error
		if isValid {
			// Mark as valid - NOTE: Using m.app.Review service. Is this correct?
			// This TUI is for workspace issues, but status update seems to use review service.
			// TODO: Verify if workspace issues should have their own status update mechanism
			//       or if they are inherently linked to review issues.
			//       Assuming link to review service for now based on original code.
			loggy.Info("Marking issue as valid via Review service", "issue_id", issueID)
			err = m.app.Review.MarkIssueAsValid(ctx, issueID) // Uses Review service
		} else {
			loggy.Info("Marking issue as invalid via Review service", "issue_id", issueID)
			err = m.app.Review.MarkIssueAsInvalid(ctx, issueID) // Uses Review service
		}

		if err != nil {
			loggy.Error("Failed to toggle issue status", "issue_id", issueID, "set_valid", isValid, "error", err)
		}

		return IssueStatusMsg{
			IssueID: issueID,
			IsValid: isValid,
			Success: err == nil,
			Error:   err,
		}
	}
}

// submitIssueToGitHub creates a command to submit the current issue details as a PR comment.
func submitIssueToGitHub(m Model, issueID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background() // Or use m.ctx
		loggy.Debug("Submitting issue to GitHub command started", "issue_id", issueID, "pr_number", m.prDetails.PRNumber)

		// Check if GitHub service is initialized
		if m.app.GitHub == nil {
			loggy.Error("GitHub service not initialized for PR submission")
			return GitHubPRSubmitMsg{
				IssueID: issueID,
				Success: false,
				Error:   fmt.Errorf("GitHub service not initialized"),
			}
		}

		// Get current issue details needed for the comment
		var issue *ws.Issue // Use alias ws from model.go's import if needed, or import directly
		found := false
		for _, iss := range m.issues {
			if iss.ID == issueID {
				issue = iss
				found = true
				break
			}
		}
		if !found || issue == nil {
			loggy.Error("Issue not found in model state during GitHub submission", "issue_id", issueID)
			return GitHubPRSubmitMsg{
				IssueID: issueID,
				Success: false,
				Error:   fmt.Errorf("issue %s not found", issueID),
			}
		}

		// Check if workspace has a GitRepoURL - fail early if not
		if m.workspace == nil || m.workspace.GitRepoURL == "" {
			wsName := "<unknown>"
			wsID := "<unknown>"
			if m.workspace != nil {
				wsName = m.workspace.Name
				wsID = m.workspace.ID
			}
			loggy.Error("Cannot submit GitHub PR comment: workspace missing or has no Git repository URL",
				"workspace_id", wsID,
				"workspace_name", wsName)
			return GitHubPRSubmitMsg{
				IssueID: issueID,
				Success: false,
				Error:   fmt.Errorf("workspace '%s' has no Git repository URL configured", wsName),
			}
		}

		// Create the PR comment payload
		prComment := &github.PRComment{
			WorkspaceID: m.workspace.ID, // Service uses this to find owner/repo
			PRNumber:    m.prDetails.PRNumber,
			FilePath:    getFilePath(issue), // Use helper from view.go?
			LineStart:   issue.LineStart,
			LineEnd:     issue.LineEnd,
			Commentary:  m.prDetails.ReviewText, // Use the text from the modal
		}

		loggy.Info("Submitting GitHub PR comment",
			"workspace_id", m.workspace.ID,
			"pr_number", prComment.PRNumber,
			"file", prComment.FilePath,
			"line_start", prComment.LineStart,
			"line_end", prComment.LineEnd)

		// Use the GitHub service to submit the comment
		err := m.app.GitHub.SubmitPRComment(ctx, prComment)
		if err != nil {
			loggy.Error("Failed to submit GitHub PR comment", "error", err)
		}

		return GitHubPRSubmitMsg{
			IssueID: issueID,
			Success: err == nil,
			Error:   err,
		}
	}
}
