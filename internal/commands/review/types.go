package review

import (
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// Chunk is an alias for workspace.Chunk to avoid import issues
type Chunk = workspace.Chunk

// Status represents the current status of the TUI
type Status int

const (
	// StatusInitializing is the initial status
	StatusInitializing Status = iota
	// StatusInit is the status when the TUI is initializing
	StatusInit
	// StatusWorkspaceInit is the status when the workspace is being initialized
	StatusWorkspaceInit
	// StatusReady is the status when the TUI is ready for user input
	StatusReady
	// StatusReviewing is the status when a review is in progress
	StatusReviewing
	// StatusProcessingFiles is the status when processing files
	StatusProcessingFiles
	// StatusGeneratingEmbeddings is the status when generating embeddings
	StatusGeneratingEmbeddings
	// StatusAnalyzingCode is the status when the LLM is analyzing code
	StatusAnalyzingCode
	// StatusViewingReview is the status when viewing a review
	StatusViewingReview
	// StatusError is the status when an error occurred
	StatusError
)

// ReviewOptions contains options for performing a review
type ReviewOptions struct {
	TargetDir   string
	Staged      bool
	CommitHash  string
	Branch      string
	BaseBranch  string
	AbsPath     string
	WorkspaceID string
}

// Add other type definitions extracted from model.go if any
