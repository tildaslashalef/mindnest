package workspace

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/parser"
	"github.com/tildaslashalef/mindnest/internal/ulid"
)

// MockChunkRepository implements the Repository interface for testing
type MockChunkRepository struct {
	files  map[string]*File
	chunks map[string][]*Chunk

	// Tracking fields for test assertions
	SaveFileCalled   int
	SaveChunksCalled int
	SavedFiles       []*File
	SavedChunks      []*Chunk

	// Mock paths to return
	workspacePaths map[string]string
}

func NewMockChunkRepository() *MockChunkRepository {
	return &MockChunkRepository{
		files:          make(map[string]*File),
		chunks:         make(map[string][]*Chunk),
		SavedFiles:     make([]*File, 0),
		SavedChunks:    make([]*Chunk, 0),
		workspacePaths: make(map[string]string),
	}
}

func (m *MockChunkRepository) GetFileByPath(ctx context.Context, workspaceID, path string) (*File, error) {
	file, ok := m.files[path]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return file, nil
}

func (m *MockChunkRepository) GetFileByID(ctx context.Context, fileID string) (*File, error) {
	for _, file := range m.files {
		if file.ID == fileID {
			return file, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *MockChunkRepository) SaveFile(ctx context.Context, file *File) error {
	m.files[file.Path] = file
	m.SaveFileCalled++
	m.SavedFiles = append(m.SavedFiles, file)
	return nil
}

func (m *MockChunkRepository) UpdateFile(ctx context.Context, file *File) error {
	if _, ok := m.files[file.Path]; !ok {
		return sql.ErrNoRows
	}
	m.files[file.Path] = file
	return nil
}

func (m *MockChunkRepository) DeleteFile(ctx context.Context, fileID string) error {
	for path, file := range m.files {
		if file.ID == fileID {
			delete(m.files, path)
			delete(m.chunks, path)
			return nil
		}
	}
	return sql.ErrNoRows
}

func (m *MockChunkRepository) GetFilesByWorkspaceID(ctx context.Context, workspaceID string) ([]*File, error) {
	var files []*File
	for _, file := range m.files {
		if file.WorkspaceID == workspaceID {
			files = append(files, file)
		}
	}
	return files, nil
}

func (m *MockChunkRepository) GetChunksByFileID(ctx context.Context, fileID string) ([]*Chunk, error) {
	for _, file := range m.files {
		if file.ID == fileID {
			return m.chunks[file.Path], nil
		}
	}
	return nil, nil
}

func (m *MockChunkRepository) GetChunkByID(ctx context.Context, chunkID string) (*Chunk, error) {
	for _, chunks := range m.chunks {
		for _, chunk := range chunks {
			if chunk.ID == chunkID {
				return chunk, nil
			}
		}
	}
	return nil, sql.ErrNoRows
}

func (m *MockChunkRepository) SaveChunk(ctx context.Context, chunk *Chunk) error {
	var filePath string
	for path, file := range m.files {
		if file.ID == chunk.FileID {
			filePath = path
			break
		}
	}

	if filePath == "" {
		return sql.ErrNoRows
	}

	if _, ok := m.chunks[filePath]; !ok {
		m.chunks[filePath] = make([]*Chunk, 0)
	}
	m.chunks[filePath] = append(m.chunks[filePath], chunk)
	return nil
}

func (m *MockChunkRepository) SaveChunksForFile(ctx context.Context, file *File, chunks []*Chunk) error {
	path := file.Path
	m.chunks[path] = chunks
	m.SaveChunksCalled++
	m.SavedChunks = append(m.SavedChunks, chunks...)
	return nil
}

func (m *MockChunkRepository) UpdateChunk(ctx context.Context, chunk *Chunk) error {
	for filePath, chunks := range m.chunks {
		for i, c := range chunks {
			if c.ID == chunk.ID {
				m.chunks[filePath][i] = chunk
				return nil
			}
		}
	}
	return sql.ErrNoRows
}

func (m *MockChunkRepository) DeleteChunk(ctx context.Context, chunkID string) error {
	for filePath, chunks := range m.chunks {
		for i, chunk := range chunks {
			if chunk.ID == chunkID {
				m.chunks[filePath] = append(chunks[:i], chunks[i+1:]...)
				return nil
			}
		}
	}
	return sql.ErrNoRows
}

func (m *MockChunkRepository) DeleteChunksByFileID(ctx context.Context, fileID string) error {
	for path, file := range m.files {
		if file.ID == fileID {
			delete(m.chunks, path)
			return nil
		}
	}
	return sql.ErrNoRows
}

func (m *MockChunkRepository) GetChunksByWorkspaceID(ctx context.Context, workspaceID string) ([]*Chunk, error) {
	var allChunks []*Chunk
	for _, chunks := range m.chunks {
		for _, chunk := range chunks {
			if chunk.WorkspaceID == workspaceID {
				allChunks = append(allChunks, chunk)
			}
		}
	}
	return allChunks, nil
}

func (m *MockChunkRepository) GetChunksByType(ctx context.Context, workspaceID string, chunkType ChunkType) ([]*Chunk, error) {
	var filteredChunks []*Chunk
	for _, chunks := range m.chunks {
		for _, chunk := range chunks {
			if chunk.WorkspaceID == workspaceID && chunk.ChunkType == chunkType {
				filteredChunks = append(filteredChunks, chunk)
			}
		}
	}
	return filteredChunks, nil
}

func TestNewSQLRepository(t *testing.T) {
	// Skip if sqlite3 driver is not available
	_, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Skip("Skipping test because sqlite3 driver is not available")
	}

	logger := loggy.NewNoopLogger()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}
	defer db.Close()

	repo := NewSQLRepository(db, logger)
	assert.NotNil(t, repo, "NewSQLRepository should return a non-nil repository")
}

func TestChunkOperations(t *testing.T) {
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

	// Create the file
	err := repo.SaveFile(ctx, testFile)
	assert.NoError(t, err)

	// Create test chunks
	testChunks := []*Chunk{
		{
			ID:          ulid.ChunkID(),
			WorkspaceID: workspaceID,
			FileID:      testFile.ID,
			Content:     "func func1() {}",
			ChunkType:   ChunkTypeFunction,
			Name:        "func1",
			StartPos: parser.Position{
				Line:   1,
				Offset: 0,
			},
			EndPos: parser.Position{
				Line:   1,
				Offset: 15,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          ulid.ChunkID(),
			WorkspaceID: workspaceID,
			FileID:      testFile.ID,
			Content:     "func func2() {}",
			ChunkType:   ChunkTypeFunction,
			Name:        "func2",
			StartPos: parser.Position{
				Line:   2,
				Offset: 17,
			},
			EndPos: parser.Position{
				Line:   2,
				Offset: 32,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	// Test SaveChunksForFile
	err = repo.SaveChunksForFile(ctx, testFile, testChunks)
	assert.NoError(t, err, "SaveChunksForFile should not return an error")

	// Test GetChunksByFileID
	retrievedChunks, err := repo.GetChunksByFileID(ctx, testFile.ID)
	assert.NoError(t, err, "GetChunksByFileID should not return an error")
	assert.Len(t, retrievedChunks, 2, "Should retrieve 2 chunks")

	// Test GetChunkByID
	chunk1, err := repo.GetChunkByID(ctx, testChunks[0].ID)
	assert.NoError(t, err, "GetChunkByID should not return an error")
	assert.Equal(t, testChunks[0].ID, chunk1.ID, "Retrieved chunk ID should match")
	assert.Equal(t, "func1", chunk1.Name, "Retrieved chunk name should match")

	// Test UpdateChunk
	chunk1.Name = "updatedFunc1"
	err = repo.UpdateChunk(ctx, chunk1)
	assert.NoError(t, err, "UpdateChunk should not return an error")

	// Verify the update
	updatedChunk, err := repo.GetChunkByID(ctx, chunk1.ID)
	assert.NoError(t, err)
	assert.Equal(t, "updatedFunc1", updatedChunk.Name, "Chunk name should be updated")

	// Test GetChunksByType
	funcChunks, err := repo.GetChunksByType(ctx, workspaceID, ChunkTypeFunction)
	assert.NoError(t, err, "GetChunksByType should not return an error")
	assert.Len(t, funcChunks, 2, "Should retrieve 2 function chunks")

	// Test DeleteChunk
	err = repo.DeleteChunk(ctx, testChunks[1].ID)
	assert.NoError(t, err, "DeleteChunk should not return an error")

	// Verify deletion
	chunksAfterDelete, err := repo.GetChunksByFileID(ctx, testFile.ID)
	assert.NoError(t, err)
	assert.Len(t, chunksAfterDelete, 1, "Should have 1 remaining chunk after deletion")

	// Test DeleteChunksByFileID
	err = repo.DeleteChunksByFileID(ctx, testFile.ID)
	assert.NoError(t, err, "DeleteChunksByFileID should not return an error")

	// Verify chunks were deleted
	chunksAfterFileDelete, err := repo.GetChunksByFileID(ctx, testFile.ID)
	assert.NoError(t, err)
	assert.Empty(t, chunksAfterFileDelete, "Should retrieve 0 chunks after deletion by file ID")

	// Test GetChunksByWorkspaceID
	// First add a chunk for testing
	newChunk := &Chunk{
		ID:          ulid.ChunkID(),
		WorkspaceID: workspaceID,
		FileID:      testFile.ID,
		Content:     "func func3() {}",
		ChunkType:   ChunkTypeFunction,
		Name:        "func3",
		StartPos: parser.Position{
			Line:   3,
			Offset: 34,
		},
		EndPos: parser.Position{
			Line:   3,
			Offset: 49,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = repo.SaveChunk(ctx, newChunk)
	assert.NoError(t, err, "SaveChunk should not return an error")

	// Now test GetChunksByWorkspaceID
	workspaceChunks, err := repo.GetChunksByWorkspaceID(ctx, workspaceID)
	assert.NoError(t, err, "GetChunksByWorkspaceID should not return an error")
	assert.Len(t, workspaceChunks, 1, "Should retrieve 1 chunk for the workspace")
}
