package review

import (
	"github.com/tildaslashalef/mindnest/internal/review"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// statusChangeMsg is a message for status changes
type statusChangeMsg struct {
	newStatus Status // Status defined in types.go
	error     error
}

// workspaceMsg is a message for when workspace is loaded
type workspaceMsg struct {
	workspace *workspace.Workspace
	error     error
}

// reviewResultMsg is a message for when review is complete
type reviewResultMsg struct {
	review      *review.Review
	reviewFiles []*review.ReviewFile
	issues      []*review.Issue
	error       error
}

// fileProcessedMsg is a message for when a file is processed
type fileProcessedMsg struct {
	file            *workspace.File
	chunks          []*Chunk // Chunk alias defined in types.go
	progressCurrent int
	progressTotal   int
	error           error
}

// startReviewProcessMsg is a message to start the review process
type startReviewProcessMsg struct{}

// continueProcessingMsg is a message for continuing processing
type continueProcessingMsg struct {
	index int
}

// embedGenerationMsg is a message for embedding generation
type embedGenerationMsg struct{}

// reviewStartMsg is a message for starting code review
type reviewStartMsg struct{}

// issueAcceptedMsg is a message for when an issue is accepted/rejected
type issueAcceptedMsg struct {
	issueID string
	success bool
	error   error
}

// reviewSetupMsg is a message for setting up the review data
type reviewSetupMsg struct {
	fileIDs        []string
	filesToProcess []string
	totalFiles     int
	commitHash     string
	branchName     string
	baseBranchName string
}

// waitForInitToFinish is a helper message used internally during init sequence.
// Note: This was previously a function returning the message, now it's just the message struct.
// The command triggering the initial state transition will be handled separately.
// func waitForInitToFinish() tea.Msg {
// 	 return statusChangeMsg{
// 		 newStatus: StatusInit,
// 	 }
// }
