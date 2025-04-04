package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/git"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/parser"
)

// MockRepository is a mock implementation of the Repository interface
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateWorkspace(ctx context.Context, workspace *Workspace) error {
	args := m.Called(ctx, workspace)
	return args.Error(0)
}

func (m *MockRepository) GetWorkspaceByID(ctx context.Context, id string) (*Workspace, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Workspace), args.Error(1)
}

func (m *MockRepository) GetWorkspaceByPath(ctx context.Context, path string) (*Workspace, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Workspace), args.Error(1)
}

func (m *MockRepository) ListWorkspaces(ctx context.Context) ([]*Workspace, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Workspace), args.Error(1)
}

func (m *MockRepository) ListWorkspacesWithPagination(ctx context.Context, params PaginationParams) ([]*Workspace, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Workspace), args.Error(1)
}

func (m *MockRepository) UpdateWorkspace(ctx context.Context, workspace *Workspace) error {
	args := m.Called(ctx, workspace)
	return args.Error(0)
}

func (m *MockRepository) DeleteWorkspace(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) FindWorkspacesByName(ctx context.Context, searchTerm string) ([]*Workspace, error) {
	args := m.Called(ctx, searchTerm)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Workspace), args.Error(1)
}

func (m *MockRepository) DuplicateWorkspace(ctx context.Context, id string, newName string) (*Workspace, error) {
	args := m.Called(ctx, id, newName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Workspace), args.Error(1)
}

func (m *MockRepository) GetWorkspaceIssues(ctx context.Context, workspaceID string) ([]*Issue, error) {
	args := m.Called(ctx, workspaceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Issue), args.Error(1)
}

func (m *MockRepository) DeleteChunk(ctx context.Context, chunkID string) error {
	args := m.Called(ctx, chunkID)
	return args.Error(0)
}

func (m *MockRepository) DeleteChunksByFileID(ctx context.Context, fileID string) error {
	args := m.Called(ctx, fileID)
	return args.Error(0)
}

func (m *MockRepository) SaveChunk(ctx context.Context, chunk *Chunk) error {
	args := m.Called(ctx, chunk)
	return args.Error(0)
}

func (m *MockRepository) GetChunkByID(ctx context.Context, chunkID string) (*Chunk, error) {
	args := m.Called(ctx, chunkID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Chunk), args.Error(1)
}

func (m *MockRepository) GetChunksByFileID(ctx context.Context, fileID string) ([]*Chunk, error) {
	args := m.Called(ctx, fileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Chunk), args.Error(1)
}

func (m *MockRepository) GetChunksByWorkspaceID(ctx context.Context, workspaceID string) ([]*Chunk, error) {
	args := m.Called(ctx, workspaceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Chunk), args.Error(1)
}

func (m *MockRepository) GetChunksByType(ctx context.Context, workspaceID string, chunkType ChunkType) ([]*Chunk, error) {
	args := m.Called(ctx, workspaceID, chunkType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Chunk), args.Error(1)
}

func (m *MockRepository) UpdateChunk(ctx context.Context, chunk *Chunk) error {
	args := m.Called(ctx, chunk)
	return args.Error(0)
}

func (m *MockRepository) SaveChunksForFile(ctx context.Context, file *File, chunks []*Chunk) error {
	args := m.Called(ctx, file, chunks)
	return args.Error(0)
}

func (m *MockRepository) SaveFile(ctx context.Context, file *File) error {
	args := m.Called(ctx, file)
	return args.Error(0)
}

func (m *MockRepository) GetFileByID(ctx context.Context, fileID string) (*File, error) {
	args := m.Called(ctx, fileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*File), args.Error(1)
}

func (m *MockRepository) GetFileByPath(ctx context.Context, workspaceID, filePath string) (*File, error) {
	args := m.Called(ctx, workspaceID, filePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*File), args.Error(1)
}

func (m *MockRepository) UpdateFile(ctx context.Context, file *File) error {
	args := m.Called(ctx, file)
	return args.Error(0)
}

func (m *MockRepository) DeleteFile(ctx context.Context, fileID string) error {
	args := m.Called(ctx, fileID)
	return args.Error(0)
}

func (m *MockRepository) GetFilesByWorkspaceID(ctx context.Context, workspaceID string) ([]*File, error) {
	args := m.Called(ctx, workspaceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*File), args.Error(1)
}

func TestWorkspaceService(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "workspace_test_*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir) // Clean up after test

	// Create a mock repository
	mockRepo := new(MockRepository)

	// Create a logger and mock dependencies
	logger := loggy.NewNoopLogger()
	var gitService *git.Service
	var parserService *parser.Service

	// Create the service with the mock repository
	service := NewService(mockRepo, logger, gitService, parserService)

	// Sample workspace with path set to the temporary directory
	sampleWorkspace := &Workspace{
		ID:          "ws_123456",
		Name:        "Test Workspace",
		Path:        tempDir,
		GitRepoURL:  "https://github.com/example/repo",
		Description: "Test description",
	}

	// Test CreateWorkspace
	t.Run("CreateWorkspace", func(t *testing.T) {
		// Setup mock expectations
		mockRepo.On("GetWorkspaceByPath", mock.Anything, mock.AnythingOfType("string")).Return(nil, ErrWorkspaceNotFound).Once()
		mockRepo.On("CreateWorkspace", mock.Anything, mock.AnythingOfType("*workspace.Workspace")).Return(nil).Once()

		// Default config for testing
		testConfig := &config.Config{
			LLM: config.LLMConfig{
				DefaultProvider: "ollama",
				DefaultModel:    "deepseek-r1",
				MaxTokens:       1000,
				Temperature:     0.5,
			},
		}

		// Execute test - use the actual tempDir that exists
		workspace, err := service.CreateWorkspace(
			context.Background(),
			tempDir,
			"Test Workspace",
			testConfig,
			"Test description",
			"https://github.com/example/repo",
		)

		// Assertions
		assert.NoError(t, err, "CreateWorkspace should not return an error")
		assert.NotNil(t, workspace, "Workspace should not be nil")
		assert.Equal(t, "Test Workspace", workspace.Name, "Workspace name should match")
		// Path will be absolute in the result, so we check it contains our temp dir
		assert.Contains(t, workspace.Path, filepath.Base(tempDir), "Workspace path should contain the temp directory name")
		assert.Equal(t, "Test description", workspace.Description, "Workspace description should match")
		assert.Equal(t, "https://github.com/example/repo", workspace.GitRepoURL, "Workspace Git repo URL should match")

		// Verify mock expectations
		mockRepo.AssertExpectations(t)
	})

	// Test CreateWorkspace when workspace already exists
	t.Run("CreateWorkspace_AlreadyExists", func(t *testing.T) {
		// Setup mock expectations - make sure to pass the actual temp dir path
		mockRepo.On("GetWorkspaceByPath", mock.Anything, mock.AnythingOfType("string")).Return(sampleWorkspace, nil).Once()

		// Default config for testing
		testConfig := &config.Config{
			LLM: config.LLMConfig{
				DefaultProvider: "ollama",
				DefaultModel:    "deepseek-r1",
				MaxTokens:       1000,
				Temperature:     0.5,
			},
		}

		// Execute test
		workspace, err := service.CreateWorkspace(
			context.Background(),
			tempDir,
			"Test Workspace",
			testConfig,
			"Test description",
			"https://github.com/example/repo",
		)

		// Assertions
		assert.Error(t, err, "CreateWorkspace should return an error")
		assert.True(t, errors.Is(err, ErrWorkspaceAlreadyExists), "Error should be ErrWorkspaceAlreadyExists")
		assert.Nil(t, workspace, "Workspace should be nil")

		// Verify mock expectations
		mockRepo.AssertExpectations(t)
	})

	// Test GetWorkspace
	t.Run("GetWorkspace", func(t *testing.T) {
		// Setup mock expectations
		mockRepo.On("GetWorkspaceByID", mock.Anything, "ws_123456").Return(sampleWorkspace, nil).Once()

		// Execute test
		workspace, err := service.GetWorkspace(context.Background(), "ws_123456")

		// Assertions
		assert.NoError(t, err, "GetWorkspace should not return an error")
		assert.NotNil(t, workspace, "Workspace should not be nil")
		assert.Equal(t, sampleWorkspace.ID, workspace.ID, "Workspace ID should match")

		// Verify mock expectations
		mockRepo.AssertExpectations(t)
	})

	// Test GetWorkspace when workspace is not found
	t.Run("GetWorkspace_NotFound", func(t *testing.T) {
		// Setup mock expectations
		mockRepo.On("GetWorkspaceByID", mock.Anything, "ws_nonexistent").Return(nil, ErrWorkspaceNotFound).Once()

		// Execute test
		workspace, err := service.GetWorkspace(context.Background(), "ws_nonexistent")

		// Assertions
		assert.Error(t, err, "GetWorkspace should return an error")
		assert.Nil(t, workspace, "Workspace should be nil")

		// Verify mock expectations
		mockRepo.AssertExpectations(t)
	})

	// Test ListWorkspaces
	t.Run("ListWorkspaces", func(t *testing.T) {
		// Setup mock expectations
		workspaces := []*Workspace{sampleWorkspace}
		mockRepo.On("ListWorkspaces", mock.Anything).Return(workspaces, nil).Once()

		// Execute test
		result, err := service.ListWorkspaces(context.Background())

		// Assertions
		assert.NoError(t, err, "ListWorkspaces should not return an error")
		assert.Len(t, result, 1, "Should return one workspace")
		assert.Equal(t, sampleWorkspace.ID, result[0].ID, "Workspace ID should match")

		// Verify mock expectations
		mockRepo.AssertExpectations(t)
	})

	// Test UpdateWorkspace
	t.Run("UpdateWorkspace", func(t *testing.T) {
		// Setup mock expectations
		mockRepo.On("UpdateWorkspace", mock.Anything, sampleWorkspace).Return(nil).Once()

		// Execute test
		err := service.UpdateWorkspace(context.Background(), sampleWorkspace)

		// Assertions
		assert.NoError(t, err, "UpdateWorkspace should not return an error")

		// Verify mock expectations
		mockRepo.AssertExpectations(t)
	})

	// Test DeleteWorkspace
	t.Run("DeleteWorkspace", func(t *testing.T) {
		// Setup mock expectations
		mockRepo.On("DeleteWorkspace", mock.Anything, "ws_123456").Return(nil).Once()

		// Execute test
		err := service.DeleteWorkspace(context.Background(), "ws_123456")

		// Assertions
		assert.NoError(t, err, "DeleteWorkspace should not return an error")

		// Verify mock expectations
		mockRepo.AssertExpectations(t)
	})

	// Test GetWorkspaceByPath
	t.Run("GetWorkspaceByPath", func(t *testing.T) {
		// Setup mock expectations
		mockRepo.On("GetWorkspaceByPath", mock.Anything, mock.AnythingOfType("string")).Return(sampleWorkspace, nil).Once()

		// Execute test - use the actual temporary directory
		workspace, err := service.GetWorkspaceByPath(context.Background(), tempDir)

		// Assertions
		assert.NoError(t, err, "GetWorkspaceByPath should not return an error")
		assert.NotNil(t, workspace, "Workspace should not be nil")
		assert.Equal(t, sampleWorkspace.ID, workspace.ID, "Workspace ID should match")

		// Verify mock expectations
		mockRepo.AssertExpectations(t)
	})
}
