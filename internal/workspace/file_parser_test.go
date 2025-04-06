package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tildaslashalef/mindnest/internal/git"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/parser"
	"github.com/tildaslashalef/mindnest/internal/ulid"
)

// ErrFileNotFound is returned when a file is not found
var ErrFileNotFound = errors.New("file not found")

// ParserService defines the interface for the parser service
type ParserService interface {
	DetectLanguage(filePath string) (string, error)
	ParseFile(filePath string) ([]*parser.RawChunk, string, error)
	IsCodeFile(filePath string) bool
}

// MockParserService mocks the parser.Service for testing
type MockParserService struct {
	mock.Mock
}

// DetectLanguage mocks the parser service DetectLanguage method
func (m *MockParserService) DetectLanguage(filePath string) (string, error) {
	args := m.Called(filePath)
	return args.String(0), args.Error(1)
}

// ParseFile mocks the parser service ParseFile method
func (m *MockParserService) ParseFile(filePath string) ([]*parser.RawChunk, string, error) {
	args := m.Called(filePath)
	return args.Get(0).([]*parser.RawChunk), args.String(1), args.Error(2)
}

// IsCodeFile mocks the parser service IsCodeFile method
func (m *MockParserService) IsCodeFile(filePath string) bool {
	args := m.Called(filePath)
	return args.Bool(0)
}

// simpleNewChunkFromRawChunk creates a new Chunk from a RawChunk for testing
func simpleNewChunkFromRawChunk(workspaceID, fileID string, rawChunk *parser.RawChunk) *Chunk {
	now := time.Now()
	return &Chunk{
		ID:          ulid.ChunkID(),
		WorkspaceID: workspaceID,
		FileID:      fileID,
		Name:        rawChunk.Name,
		Content:     rawChunk.Content,
		StartPos:    rawChunk.StartPos,
		EndPos:      rawChunk.EndPos,
		ChunkType:   ChunkType(rawChunk.Type),
		Signature:   rawChunk.Signature,
		ParentID:    rawChunk.ParentID,
		ChildIDs:    rawChunk.ChildIDs,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// TestService is a copy of the Service struct for testing
type TestService struct {
	repo          Repository
	logger        *loggy.Logger
	gitService    *git.Service
	parserService ParserService
}

// shouldParseFile determines if a file should be parsed
func (s *TestService) shouldParseFile(filePath string) bool {
	return s.parserService.IsCodeFile(filePath)
}

// ParseFile parses a single file and saves the chunks
func (s *TestService) ParseFile(ctx context.Context, workspaceID string, filePath string) (*File, []*Chunk, error) {
	// Get workspace
	workspace, err := s.repo.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}

	// Create absolute path by joining workspace path and file path
	absPath := filepath.Join(workspace.Path, filePath)

	// Detect language
	language, err := s.parserService.DetectLanguage(absPath)
	if err != nil {
		return nil, nil, err
	}

	// Get or create file record
	file, err := s.repo.GetFileByPath(ctx, workspaceID, filePath)
	if err != nil {
		// Create a new file if it doesn't exist
		file = NewFile(workspaceID, filePath, language)
	}

	// Parse file to get raw chunks
	rawChunks, _, err := s.parserService.ParseFile(absPath)
	if err != nil {
		return nil, nil, err
	}

	// Convert raw chunks to our model using two-pass approach
	chunks := make([]*Chunk, 0, len(rawChunks))
	idMap := make(map[string]string) // Map from temp parser IDs to real ULID chunk IDs

	// First pass: create chunks and build ID mapping
	for _, rawChunk := range rawChunks {
		chunk := simpleNewChunkFromRawChunk(workspaceID, file.ID, rawChunk)
		chunks = append(chunks, chunk)

		// Store the mapping from parser ID to chunk ID
		idMap[rawChunk.ID] = chunk.ID
	}

	// Second pass: update parent/child relationships with real ULIDs
	for i, chunk := range chunks {
		rawChunk := rawChunks[i]

		// Update parent ID if exists
		if rawChunk.ParentID != "" {
			if realParentID, exists := idMap[rawChunk.ParentID]; exists {
				chunk.ParentID = realParentID
			}
		}

		// Update child IDs if any
		if len(rawChunk.ChildIDs) > 0 {
			realChildIDs := make([]string, 0, len(rawChunk.ChildIDs))
			for _, tempChildID := range rawChunk.ChildIDs {
				if realChildID, exists := idMap[tempChildID]; exists {
					realChildIDs = append(realChildIDs, realChildID)
				}
			}
			chunk.ChildIDs = realChildIDs
		}
	}

	// Save file and its chunks
	if err := s.repo.SaveFile(ctx, file); err != nil {
		return nil, nil, err
	}

	// Save chunks
	err = s.repo.SaveChunksForFile(ctx, file, chunks)
	if err != nil {
		return nil, nil, err
	}

	return file, chunks, nil
}

// ParseFiles parses multiple files and returns all chunks
func (s *TestService) ParseFiles(ctx context.Context, workspaceID string, filePaths []string) ([]*Chunk, error) {
	allChunks := make([]*Chunk, 0)

	for _, filePath := range filePaths {
		// Skip files that shouldn't be parsed
		if !s.shouldParseFile(filePath) {
			continue
		}

		_, chunks, err := s.ParseFile(ctx, workspaceID, filePath)
		if err != nil {
			continue
		}

		allChunks = append(allChunks, chunks...)
	}

	return allChunks, nil
}

// ParseChangedFiles parses files in a diff
func (s *TestService) ParseChangedFiles(ctx context.Context, workspaceID string, diff *git.DiffResult) ([]string, error) {
	var fileIDs []string

	for _, file := range diff.Files {
		// Skip deleted files
		if file.ChangeType == git.ChangeTypeDeleted {
			continue
		}

		// Skip files that shouldn't be parsed
		if !s.shouldParseFile(file.Path) {
			continue
		}

		f, _, err := s.ParseFile(ctx, workspaceID, file.Path)
		if err != nil {
			continue
		}

		fileIDs = append(fileIDs, f.ID)
	}

	return fileIDs, nil
}

func TestParseFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "workspace_test_*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir)

	// Create a Go file to parse
	goFileName := "main.go"
	goFilePath := filepath.Join(tempDir, goFileName)
	goContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}
`
	err = os.WriteFile(goFilePath, []byte(goContent), 0644)
	require.NoError(t, err, "Failed to write test Go file")

	// Setup mock repository
	mockRepo := new(MockRepository)

	// Mock workspace
	workspace := &Workspace{
		ID:   "workspace1",
		Path: tempDir,
		Name: "Test Workspace",
	}

	// Mock GetWorkspaceByID
	mockRepo.On("GetWorkspaceByID", mock.Anything, "workspace1").Return(workspace, nil)

	// Create mock parser service
	mockParserService := new(MockParserService)

	// Mock DetectLanguage
	mockParserService.On("DetectLanguage", filepath.Join(tempDir, goFileName)).Return("Go", nil)

	// Mock ParseFile
	mockParserService.On("ParseFile", filepath.Join(tempDir, goFileName)).Return([]*parser.RawChunk{
		{
			Type:     string(parser.ChunkTypeFunction),
			Name:     "main",
			Content:  "func main() {\n\tfmt.Println(\"Hello, world!\")\n}",
			Metadata: map[string]interface{}{"language": "Go"},
		},
	}, "Go", nil)

	// Mock file operations
	var _ *File // Using _ to explicitly ignore the variable

	// Simulate file not found to trigger creation
	mockRepo.On("GetFileByPath", mock.Anything, "workspace1", goFileName).Return(nil, ErrFileNotFound).Once()

	// Mock save file
	mockRepo.On("SaveFile", mock.Anything, mock.AnythingOfType("*workspace.File")).Return(nil).Run(func(args mock.Arguments) {
		// Not storing the saved file since we don't need it in this test
	})

	// Mock save chunks
	mockRepo.On("SaveChunksForFile", mock.Anything, mock.AnythingOfType("*workspace.File"), mock.AnythingOfType("[]*workspace.Chunk")).Return(nil)

	// Setup logger
	logger := loggy.NewNoopLogger()

	// Setup service with our mocks
	service := &TestService{
		repo:          mockRepo,
		logger:        logger,
		gitService:    nil,
		parserService: mockParserService,
	}

	// Test ParseFile
	ctx := context.Background()
	resultFile, chunks, err := service.ParseFile(ctx, "workspace1", goFileName)

	// Assertions
	assert.NoError(t, err, "ParseFile should not return an error")
	assert.NotNil(t, resultFile, "ParseFile should return a non-nil file")
	assert.Equal(t, "Go", resultFile.Language, "File language should be Go")
	assert.NotEmpty(t, chunks, "ParseFile should return non-empty chunks")

	// Verify we have a function chunk with name "main"
	var mainFuncFound bool
	for _, chunk := range chunks {
		if chunk.ChunkType == ChunkTypeFunction && chunk.Name == "main" {
			mainFuncFound = true
			assert.Contains(t, chunk.Content, "fmt.Println", "Main function should contain Println")
			break
		}
	}
	assert.True(t, mainFuncFound, "Should find a main function chunk")

	// Verify mock expectations
	mockRepo.AssertExpectations(t)
	mockParserService.AssertExpectations(t)
}

func TestRefreshFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "workspace_test_*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir)

	// Create a Go file to parse
	goFileName := "main.go"
	goFilePath := filepath.Join(tempDir, goFileName)
	goContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}
`
	err = os.WriteFile(goFilePath, []byte(goContent), 0644)
	require.NoError(t, err, "Failed to write test Go file")

	// Setup mock repository
	mockRepo := new(MockRepository)

	// Mock workspace
	workspace := &Workspace{
		ID:   "workspace1",
		Path: tempDir,
		Name: "Test Workspace",
	}

	// Mock GetWorkspaceByID
	mockRepo.On("GetWorkspaceByID", mock.Anything, "workspace1").Return(workspace, nil)

	// Mock parser service with real behavior
	logger := loggy.NewNoopLogger()
	parserService := parser.NewService(logger)

	// Mock file operations
	now := time.Now().Add(-1 * time.Hour) // File was parsed an hour ago
	file := &File{
		ID:          "file1",
		WorkspaceID: "workspace1",
		Path:        goFileName,
		Language:    "Go",
		LastParsed:  &now,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Mock getting existing file
	mockRepo.On("GetFileByPath", mock.Anything, "workspace1", goFileName).Return(file, nil)

	// Mock save file for the updated last parsed time
	mockRepo.On("SaveFile", mock.Anything, mock.AnythingOfType("*workspace.File")).Return(nil).Run(func(args mock.Arguments) {
		savedFile := args.Get(1).(*File)
		assert.NotNil(t, savedFile.LastParsed, "LastParsed should be updated")
		assert.True(t, savedFile.LastParsed.After(now), "LastParsed should be after the original time")
	})

	// Mock save chunks
	mockRepo.On("SaveChunksForFile", mock.Anything, mock.AnythingOfType("*workspace.File"), mock.AnythingOfType("[]*workspace.Chunk")).Return(nil)

	// Setup service
	service := NewServiceWithRepository(mockRepo, logger, nil, parserService)

	// Modify the file to trigger refresh
	time.Sleep(10 * time.Millisecond) // Ensure file mod time is different
	newContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, updated world!")
}

func newFunc() {
	fmt.Println("New function!")
}
`
	err = os.WriteFile(goFilePath, []byte(newContent), 0644)
	require.NoError(t, err, "Failed to update test Go file")

	// Test RefreshFile
	ctx := context.Background()
	resultFile, chunks, err := service.RefreshFile(ctx, "workspace1", goFileName)

	// Assertions
	assert.NoError(t, err, "RefreshFile should not return an error")
	assert.NotNil(t, resultFile, "RefreshFile should return a non-nil file")
	assert.NotNil(t, resultFile.LastParsed, "File LastParsed should be updated")
	assert.True(t, resultFile.LastParsed.After(now), "LastParsed should be after the original time")
	assert.NotEmpty(t, chunks, "RefreshFile should return non-empty chunks")

	// Verify we have both functions
	var mainFuncFound, newFuncFound bool
	for _, chunk := range chunks {
		if chunk.ChunkType == ChunkTypeFunction && chunk.Name == "main" {
			mainFuncFound = true
			assert.Contains(t, chunk.Content, "updated world", "Main function should be updated")
		}
		if chunk.ChunkType == ChunkTypeFunction && chunk.Name == "newFunc" {
			newFuncFound = true
			assert.Contains(t, chunk.Content, "New function", "New function should be found")
		}
	}
	assert.True(t, mainFuncFound, "Should find the updated main function")
	assert.True(t, newFuncFound, "Should find the new function")

	// Verify mock expectations
	mockRepo.AssertExpectations(t)
}

func TestParseFiles(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "workspace_test_*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir)

	// Create two Go files to parse
	goFile1 := "main.go"
	goFile1Path := filepath.Join(tempDir, goFile1)
	goContent1 := `package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}
`
	err = os.WriteFile(goFile1Path, []byte(goContent1), 0644)
	require.NoError(t, err, "Failed to write first test Go file")

	goFile2 := "utils.go"
	goFile2Path := filepath.Join(tempDir, goFile2)
	goContent2 := `package main

func helper() string {
	return "helper function"
}
`
	err = os.WriteFile(goFile2Path, []byte(goContent2), 0644)
	require.NoError(t, err, "Failed to write second test Go file")

	// Create a mock repository
	mockRepo := new(MockRepository)

	// Create a mock parser service
	mockParserService := new(MockParserService)

	// Mock the parser service IsCodeFile method to only return true for .go files
	mockParserService.On("IsCodeFile", mock.MatchedBy(func(path string) bool {
		return strings.HasSuffix(path, ".go")
	})).Return(true)

	// Setup workspace
	workspace := &Workspace{
		ID:   "workspace1",
		Path: tempDir,
		Name: "Test Workspace",
	}

	// Mock GetWorkspaceByID
	mockRepo.On("GetWorkspaceByID", mock.Anything, "workspace1").Return(workspace, nil).Times(2)

	// Mock DetectLanguage for both Go files
	mockParserService.On("DetectLanguage", filepath.Join(tempDir, goFile1)).Return("Go", nil)
	mockParserService.On("DetectLanguage", filepath.Join(tempDir, goFile2)).Return("Go", nil)

	// Mock ParseFile for both Go files
	mockParserService.On("ParseFile", filepath.Join(tempDir, goFile1)).Return([]*parser.RawChunk{
		{
			Type:     string(parser.ChunkTypeFunction),
			Name:     "main",
			Content:  "func main() {\n\tfmt.Println(\"Hello, world!\")\n}",
			Metadata: map[string]interface{}{"language": "Go"},
		},
	}, "Go", nil)

	mockParserService.On("ParseFile", filepath.Join(tempDir, goFile2)).Return([]*parser.RawChunk{
		{
			Type:     string(parser.ChunkTypeFunction),
			Name:     "helper",
			Content:  "func helper() string {\n\treturn \"helper function\"\n}",
			Metadata: map[string]interface{}{"language": "Go"},
		},
	}, "Go", nil)

	// Mock file operations for first file
	mockRepo.On("GetFileByPath", mock.Anything, "workspace1", goFile1).Return(nil, ErrFileNotFound).Once()
	mockRepo.On("SaveFile", mock.Anything, mock.MatchedBy(func(f *File) bool {
		return f.Path == goFile1
	})).Return(nil).Once()
	mockRepo.On("SaveChunksForFile", mock.Anything, mock.MatchedBy(func(f *File) bool {
		return f.Path == goFile1
	}), mock.AnythingOfType("[]*workspace.Chunk")).Return(nil).Once()

	// Mock file operations for second file
	mockRepo.On("GetFileByPath", mock.Anything, "workspace1", goFile2).Return(nil, ErrFileNotFound).Once()
	mockRepo.On("SaveFile", mock.Anything, mock.MatchedBy(func(f *File) bool {
		return f.Path == goFile2
	})).Return(nil).Once()
	mockRepo.On("SaveChunksForFile", mock.Anything, mock.MatchedBy(func(f *File) bool {
		return f.Path == goFile2
	}), mock.AnythingOfType("[]*workspace.Chunk")).Return(nil).Once()

	// Setup logger
	logger := loggy.NewNoopLogger()

	// Setup service with our mocks
	service := &TestService{
		repo:          mockRepo,
		logger:        logger,
		gitService:    nil,
		parserService: mockParserService,
	}

	// Test ParseFiles
	ctx := context.Background()
	filePaths := []string{goFile1, goFile2}
	chunks, err := service.ParseFiles(ctx, "workspace1", filePaths)

	// Assertions
	assert.NoError(t, err, "ParseFiles should not return an error")
	assert.NotEmpty(t, chunks, "ParseFiles should return non-empty chunks")

	// Verify we have functions from both files
	var mainFuncFound, helperFuncFound bool
	for _, chunk := range chunks {
		if chunk.ChunkType == ChunkTypeFunction && chunk.Name == "main" {
			mainFuncFound = true
		}
		if chunk.ChunkType == ChunkTypeFunction && chunk.Name == "helper" {
			helperFuncFound = true
		}
	}
	assert.True(t, mainFuncFound, "Should find the main function")
	assert.True(t, helperFuncFound, "Should find the helper function")

	// Verify mock expectations
	mockRepo.AssertExpectations(t)
	mockParserService.AssertExpectations(t)
}

func TestShouldParseFile(t *testing.T) {
	// Create mock services
	logger := loggy.NewNoopLogger()
	parserService := parser.NewService(logger)

	// Setup service
	service := NewService(nil, logger, nil, parserService)

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create test files for different types
	goFilePath := filepath.Join(tempDir, "main.go")
	mdFilePath := filepath.Join(tempDir, "README.md")
	vendorFilePath := filepath.Join(tempDir, "vendor", "package", "lib.go")

	// Create directories
	err := os.MkdirAll(filepath.Dir(vendorFilePath), 0755)
	require.NoError(t, err, "Failed to create vendor directory")

	// Write sample content to files
	err = os.WriteFile(goFilePath, []byte("package main\n\nfunc main() {}"), 0644)
	require.NoError(t, err, "Failed to write Go file")

	err = os.WriteFile(mdFilePath, []byte("# Test Project\n\nThis is a test project."), 0644)
	require.NoError(t, err, "Failed to write markdown file")

	err = os.WriteFile(vendorFilePath, []byte("package lib\n\nfunc Helper() {}"), 0644)
	require.NoError(t, err, "Failed to write vendor file")

	// Test cases
	testCases := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "Go file",
			filePath: goFilePath,
			want:     true,
		},
		{
			name:     "Markdown file",
			filePath: mdFilePath,
			want:     false,
		},
		{
			name:     "Vendor file",
			filePath: vendorFilePath,
			want:     false,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := service.shouldParseFile(tc.filePath)
			assert.Equal(t, tc.want, result, "shouldParseFile returned unexpected result")
		})
	}
}

func TestParseChangedFiles(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "workspace_test_*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir)

	// Create a test Go file
	goFileName := "main.go"
	goFilePath := filepath.Join(tempDir, goFileName)
	goContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}
`
	err = os.WriteFile(goFilePath, []byte(goContent), 0644)
	require.NoError(t, err, "Failed to write test Go file")

	// Setup mock repository
	mockRepo := new(MockRepository)

	// Create a mock parser service
	mockParserService := new(MockParserService)

	// Mock the parser service IsCodeFile method
	mockParserService.On("IsCodeFile", mock.MatchedBy(func(path string) bool {
		return path == goFileName
	})).Return(true)

	// Setup workspace
	workspace := &Workspace{
		ID:   "workspace1",
		Path: tempDir,
		Name: "Test Workspace",
	}

	// Mock GetWorkspaceByID
	mockRepo.On("GetWorkspaceByID", mock.Anything, "workspace1").Return(workspace, nil)

	// Mock DetectLanguage
	mockParserService.On("DetectLanguage", filepath.Join(tempDir, goFileName)).Return("Go", nil)

	// Mock ParseFile
	mockParserService.On("ParseFile", filepath.Join(tempDir, goFileName)).Return([]*parser.RawChunk{
		{
			Type:     string(parser.ChunkTypeFunction),
			Name:     "main",
			Content:  "func main() {\n\tfmt.Println(\"Hello, world!\")\n}",
			Metadata: map[string]interface{}{"language": "Go"},
		},
	}, "Go", nil)

	// Setup file
	file := &File{
		ID:          "file1",
		WorkspaceID: "workspace1",
		Path:        goFileName,
		Language:    "Go",
	}

	// Mock file operations
	mockRepo.On("GetFileByPath", mock.Anything, "workspace1", goFileName).Return(nil, ErrFileNotFound)
	mockRepo.On("SaveFile", mock.Anything, mock.AnythingOfType("*workspace.File")).Return(nil).Run(func(args mock.Arguments) {
		savedFile := args.Get(1).(*File)
		file.ID = savedFile.ID // Copy the ID from the saved file
	})
	mockRepo.On("SaveChunksForFile", mock.Anything, mock.AnythingOfType("*workspace.File"), mock.AnythingOfType("[]*workspace.Chunk")).Return(nil)

	// Setup logger
	logger := loggy.NewNoopLogger()

	// Setup service with our mocks
	service := &TestService{
		repo:          mockRepo,
		logger:        logger,
		gitService:    nil,
		parserService: mockParserService,
	}

	// Create a mock diff result
	diffResult := &git.DiffResult{
		Files: []git.ChangedFile{
			{
				Path:       goFileName,
				ChangeType: git.ChangeTypeModified,
			},
			{
				Path:       "deleted.go",
				ChangeType: git.ChangeTypeDeleted, // Should be skipped
			},
		},
	}

	// Test ParseChangedFiles
	ctx := context.Background()
	fileIDs, err := service.ParseChangedFiles(ctx, "workspace1", diffResult)

	// Assertions
	assert.NoError(t, err, "ParseChangedFiles should not return an error")
	assert.Len(t, fileIDs, 1, "ParseChangedFiles should return 1 file ID")
	assert.Equal(t, file.ID, fileIDs[0], "Returned file ID should match")

	// Verify mock expectations
	mockRepo.AssertExpectations(t)
	mockParserService.AssertExpectations(t)
}
