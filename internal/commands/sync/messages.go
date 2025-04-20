package sync

import "github.com/tildaslashalef/mindnest/internal/sync"

type (
	// SyncStartMsg is the initial message sent to the model
	SyncStartMsg struct{}

	// SyncProgressMsg is sent to update sync progress
	SyncProgressMsg struct {
		EntityType    sync.EntityType
		EntityID      string
		CurrentItem   int
		TotalItems    int
		ItemsSynced   int
		ItemsFailed   int
		ErrorMessage  string
		ProgressValue float64
	}

	// SyncCompleteMsg is sent when sync is complete
	SyncCompleteMsg struct {
		Result          *sync.SyncResult
		Error           error
		WorkspaceCount  int
		ReviewCount     int
		ReviewFileCount int
		IssueCount      int
		FileCount       int
	}

	// EntitiesToSyncMsg is sent after calculating what needs to be synced.
	EntitiesToSyncMsg struct {
		WorkspaceCount      int
		ReviewCount         int
		ReviewFileCount     int
		IssueCount          int
		FileCount           int
		TotalEntities       int
		EntityTypes         []sync.EntityType // Preserves sync order
		UnsyncedWorkspaces  []string
		UnsyncedReviews     []string
		UnsyncedReviewFiles []string
		UnsyncedIssues      []string
		UnsyncedFiles       []string
	}
)
