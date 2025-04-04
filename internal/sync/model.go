// Package sync provides functionality for syncing local data to a remote server
package sync

import (
	"time"
)

// SyncType represents the type of sync operation
type SyncType string

const (
	// SyncTypeManual represents a manually initiated sync
	SyncTypeManual SyncType = "manual"
	// SyncTypePostReview represents a sync triggered after a review
	SyncTypePostReview SyncType = "post_review"
)

// EntityType represents the type of entity being synced
type EntityType string

const (
	// EntityTypeWorkspace represents a workspace entity
	EntityTypeWorkspace EntityType = "workspace"
	// EntityTypeReview represents a review entity
	EntityTypeReview EntityType = "review"
	// EntityTypeReviewFile represents a review file entity
	EntityTypeReviewFile EntityType = "review_file"
	// EntityTypeIssue represents an issue entity
	EntityTypeIssue EntityType = "issue"
	// EntityTypeFile represents a file entity
	EntityTypeFile EntityType = "file"
)

// SyncErrorType represents the type of error that occurred during sync
type SyncErrorType string

const (
	// SyncErrorTypeNetwork represents a network error
	SyncErrorTypeNetwork SyncErrorType = "network"
	// SyncErrorTypeAuth represents an authentication error
	SyncErrorTypeAuth SyncErrorType = "auth"
	// SyncErrorTypeServer represents a server error
	SyncErrorTypeServer SyncErrorType = "server"
	// SyncErrorTypeClient represents a client error
	SyncErrorTypeClient SyncErrorType = "client"
	// SyncErrorTypeUnknown represents an unknown error
	SyncErrorTypeUnknown SyncErrorType = "unknown"
)

// SyncLog represents a log entry for a sync operation
type SyncLog struct {
	ID           string        `json:"id"`
	SyncType     SyncType      `json:"sync_type"`
	EntityType   EntityType    `json:"entity_type"`
	EntityID     string        `json:"entity_id"`
	Success      bool          `json:"success"`
	ErrorType    SyncErrorType `json:"error_type,omitempty"`
	ErrorMessage string        `json:"error_message,omitempty"`
	ItemsSynced  int           `json:"items_synced"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  time.Time     `json:"completed_at"`
}

// NewSyncLog creates a new sync log entry
func NewSyncLog(syncType SyncType, entityType EntityType, entityID string) *SyncLog {
	now := time.Now()
	return &SyncLog{
		SyncType:    syncType,
		EntityType:  entityType,
		EntityID:    entityID,
		Success:     false, // Default to false, set to true when successful
		StartedAt:   now,
		CompletedAt: now, // Will be updated when the sync completes
	}
}

// MarkSuccessful marks the sync log as successful
func (l *SyncLog) MarkSuccessful(itemsSynced int) {
	l.Success = true
	l.ItemsSynced = itemsSynced
	l.CompletedAt = time.Now()
}

// MarkFailed marks the sync log as failed
func (l *SyncLog) MarkFailed(errorType SyncErrorType, errorMessage string) {
	l.Success = false
	l.ErrorType = errorType
	l.ErrorMessage = errorMessage
	l.CompletedAt = time.Now()
}

// SyncResult contains the results of a sync operation
type SyncResult struct {
	TotalItems      int
	SuccessItems    int
	FailedItems     int
	Success         bool
	ErrorType       SyncErrorType
	ErrorMessage    string
	Duration        time.Duration
	WorkspaceCount  int
	ReviewCount     int
	ReviewFileCount int
	IssueCount      int
	FileCount       int
}

// EntitySyncStatus represents the sync status of an entity
type EntitySyncStatus struct {
	EntityID   string        `json:"entity_id"`
	EntityType EntityType    `json:"entity_type"`
	SyncedAt   time.Time     `json:"synced_at"`
	Success    bool          `json:"success"`
	ErrorType  SyncErrorType `json:"error_type,omitempty"`
	Message    string        `json:"message,omitempty"`
}

// Syncable is an interface for entities that can be synced
type Syncable interface {
	GetID() string
	GetSyncedAt() *time.Time
	SetSyncedAt(time.Time)
	GetUpdatedAt() time.Time
	NeedsSync() bool
}
