package workspace

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tildaslashalef/mindnest/internal/ulid"
)

func TestFileOperations(t *testing.T) {
	// Create a mock repository for testing
	repo := NewMockChunkRepository()

	// Context for operations
	ctx := context.Background()

	// Create a test file
	workspaceID := "workspace1"
	testFile := &File{
		ID:          ulid.FileID(),
		WorkspaceID: workspaceID,
		Path:        "test/file.go",
		Language:    "Go",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Test SaveFile
	err := repo.SaveFile(ctx, testFile)
	assert.NoError(t, err, "SaveFile should not return an error")

	// Test GetFileByPath
	retrievedFile, err := repo.GetFileByPath(ctx, workspaceID, testFile.Path)
	assert.NoError(t, err, "GetFileByPath should not return an error for existing file")
	assert.Equal(t, testFile, retrievedFile, "Retrieved file should match created file")

	// Test GetFileByPath for non-existent file
	_, err = repo.GetFileByPath(ctx, workspaceID, "non/existent/file.go")
	assert.Equal(t, sql.ErrNoRows, err, "GetFileByPath should return ErrNoRows for non-existent file")

	// Test UpdateFile
	testFile.Language = "JavaScript"
	err = repo.UpdateFile(ctx, testFile)
	assert.NoError(t, err, "UpdateFile should not return an error for existing file")

	// Verify the update
	retrievedFile, err = repo.GetFileByPath(ctx, workspaceID, testFile.Path)
	assert.NoError(t, err)
	assert.Equal(t, "JavaScript", retrievedFile.Language, "File language should be updated")

	// Test GetFileByID
	retrievedFileByID, err := repo.GetFileByID(ctx, testFile.ID)
	assert.NoError(t, err, "GetFileByID should not return an error for existing file")
	assert.Equal(t, testFile, retrievedFileByID, "Retrieved file should match created file")

	// Create a second file for testing GetFilesByWorkspaceID
	testFile2 := &File{
		ID:          ulid.FileID(),
		WorkspaceID: workspaceID,
		Path:        "test/another.js",
		Language:    "JavaScript",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = repo.SaveFile(ctx, testFile2)
	assert.NoError(t, err, "SaveFile should not return an error for second file")

	// Test GetFilesByWorkspaceID
	files, err := repo.GetFilesByWorkspaceID(ctx, workspaceID)
	assert.NoError(t, err, "GetFilesByWorkspaceID should not return an error")
	assert.Len(t, files, 2, "Should retrieve 2 files")

	// Test DeleteFile
	err = repo.DeleteFile(ctx, testFile.ID)
	assert.NoError(t, err, "DeleteFile should not return an error for existing file")

	// Verify the file was deleted
	_, err = repo.GetFileByPath(ctx, workspaceID, testFile.Path)
	assert.Equal(t, sql.ErrNoRows, err, "GetFileByPath should return ErrNoRows after file is deleted")

	// Test GetFilesByWorkspaceID after deletion
	filesAfterDelete, err := repo.GetFilesByWorkspaceID(ctx, workspaceID)
	assert.NoError(t, err, "GetFilesByWorkspaceID should not return an error after deletion")
	assert.Len(t, filesAfterDelete, 1, "Should retrieve 1 file after deletion")
	assert.Equal(t, testFile2.ID, filesAfterDelete[0].ID, "Remaining file should be the second one")
}
