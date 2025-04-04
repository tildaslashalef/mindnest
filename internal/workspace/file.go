// Package workspace provides workspace management for the Mindnest application
package workspace

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/tildaslashalef/mindnest/internal/ulid"
)

// File represents a file in the codebase
type File struct {
	ID          string          `json:"id"`           // Unique identifier for the file
	WorkspaceID string          `json:"workspace_id"` // Workspace the file belongs to
	Path        string          `json:"path"`         // Path to the file relative to the workspace root
	Language    string          `json:"language"`     // Programming language of the file
	LastParsed  *time.Time      `json:"last_parsed"`  // When the file was last parsed
	Metadata    json.RawMessage `json:"metadata"`     // Additional metadata about the file
	CreatedAt   time.Time       `json:"created_at"`   // When the file was created
	UpdatedAt   time.Time       `json:"updated_at"`   // When the file was last updated
	SyncedAt    *time.Time      `json:"synced_at"`    // When the file was last synced
}

// NewFile creates a new file
func NewFile(workspaceID, path, language string) *File {
	now := time.Now()
	return &File{
		ID:          ulid.FileID(),
		WorkspaceID: workspaceID,
		Path:        path,
		Language:    language,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// UpdateLastParsed updates the LastParsed timestamp to the current time
func (f *File) UpdateLastParsed() {
	now := time.Now()
	f.LastParsed = &now
	f.UpdatedAt = now
}

// SetMetadata sets the file's metadata
func (f *File) SetMetadata(metadata map[string]interface{}) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	f.Metadata = data
	return nil
}

// GetMetadata returns the file's metadata as a map
func (f *File) GetMetadata() (map[string]interface{}, error) {
	if f.Metadata == nil || len(f.Metadata) == 0 {
		return map[string]interface{}{}, nil
	}

	var result map[string]interface{}
	err := json.Unmarshal(f.Metadata, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Filename returns just the filename portion of the path
func (f *File) Filename() string {
	return filepath.Base(f.Path)
}

// Extension returns the file extension
func (f *File) Extension() string {
	return filepath.Ext(f.Path)
}

// Directory returns the directory portion of the path
func (f *File) Directory() string {
	return filepath.Dir(f.Path)
}
