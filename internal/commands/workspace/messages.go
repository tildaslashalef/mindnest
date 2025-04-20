package workspace

import (
	ws "github.com/tildaslashalef/mindnest/internal/workspace"
)

// Message types used within the workspace TUI
type (
	// InitMsg is the initial message sent to the model
	// (Not strictly necessary as Init() returns a Cmd, but kept for consistency if needed)
	InitMsg struct{}

	// LoadDataMsg is sent when workspace data (issues) has been loaded
	LoadDataMsg struct {
		Workspace *ws.Workspace
		Issues    []*ws.Issue
		Error     error
	}

	// IssueStatusMsg is sent when an issue's status has been updated asynchronously
	IssueStatusMsg struct {
		IssueID string
		IsValid bool
		Success bool
		Error   error
	}

	// GitHubPRSubmitMsg is sent after attempting to submit an issue to GitHub PR
	GitHubPRSubmitMsg struct {
		IssueID string
		Success bool
		Error   error
	}
)

// GitHubPRDetails contains the details collected for submitting to a GitHub PR
// (Could also be considered a type rather than a message, but placed here for now)
type GitHubPRDetails struct {
	PRNumber   int
	ReviewText string // Combined text with description, suggestion, and code snippet
}
