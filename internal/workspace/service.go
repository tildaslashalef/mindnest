// Package workspace provides workspace management for the Mindnest application
package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/git"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/parser"
)

// Service provides workspace management operations
type Service struct {
	repo          Repository
	logger        *loggy.Logger
	gitService    *git.Service
	parserService *parser.Service
}

// NewService creates a new workspace service
func NewService(
	db *sql.DB,
	logger *loggy.Logger,
	gitService *git.Service,
	parserService *parser.Service,
) *Service {
	repo := NewSQLRepository(db, logger)

	return &Service{
		repo:          repo,
		logger:        logger,
		gitService:    gitService,
		parserService: parserService,
	}
}

// NewServiceWithRepository creates a service with a custom repository implementation (for testing)
func NewServiceWithRepository(
	repo Repository,
	logger *loggy.Logger,
	gitService *git.Service,
	parserService *parser.Service,
) *Service {
	return &Service{
		repo:          repo,
		logger:        logger,
		gitService:    gitService,
		parserService: parserService,
	}
}

// GetRepository returns the repository implementation
func (s *Service) GetRepository() Repository {
	return s.repo
}

// initGitRepo initializes the Git repository for the workspace path
func (s *Service) initGitRepo(path string) error {
	if err := s.gitService.InitRepo(path); err != nil {
		s.logger.Warn("Failed to initialize Git repository", "path", path, "error", err)
		return fmt.Errorf("initializing git repository: %w", err)
	}
	return nil
}

// CreateWorkspace creates a new workspace
func (s *Service) CreateWorkspace(ctx context.Context, path, name string, cfg *config.Config, description, gitRepoURL string) (*Workspace, error) {
	// Normalize path to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if workspace already exists for this path
	existingWorkspace, err := s.repo.GetWorkspaceByPath(ctx, absPath)
	if err != nil && !errors.Is(err, ErrWorkspaceNotFound) {
		return nil, fmt.Errorf("failed to check for existing workspace: %w", err)
	}

	if existingWorkspace != nil {
		return nil, ErrWorkspaceAlreadyExists
	}

	// Create a new workspace
	ws, err := New(absPath, name, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	// Initialize Git repository
	if err := s.initGitRepo(absPath); err != nil {
		s.logger.Warn("Failed to initialize Git repository, continuing without Git support",
			"path", absPath,
			"error", err)
	}

	// Set optional fields
	if description != "" {
		ws.SetDescription(description)
	}

	if gitRepoURL != "" {
		ws.SetGitRepoURL(gitRepoURL)
	} else if ws.HasGitRepo() {
		// Try to detect Git repository URL if not provided
		// This is a placeholder - in a real implementation, you'd use go-git to get the remote URL
		s.logger.Debug("Git repository detected but URL not provided", "path", absPath)
	}

	// Save the workspace
	if err := s.repo.CreateWorkspace(ctx, ws); err != nil {
		return nil, fmt.Errorf("failed to save workspace: %w", err)
	}

	s.logger.Info("Created new workspace",
		"id", ws.ID,
		"name", ws.Name,
		"path", ws.Path,
	)

	// Start a background task to index initial files
	go func() {
		// This would ideally scan for code files in the workspace and parse them
		s.logger.Info("Starting initial file indexing for workspace",
			"id", ws.ID,
			"path", ws.Path)

		// TODO: Implement full directory traversal for initial indexing
		// For now, just log that we would do this
		s.logger.Info("Initial file indexing completed", "id", ws.ID)
	}()

	return ws, nil
}

// GetWorkspace retrieves a workspace by ID
func (s *Service) GetWorkspace(ctx context.Context, id string) (*Workspace, error) {
	ws, err := s.repo.GetWorkspaceByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %w", err)
	}
	return ws, nil
}

func (s *Service) GetWorkspaceIssues(ctx context.Context, workspaceID string) ([]*Issue, error) {
	return s.repo.GetWorkspaceIssues(ctx, workspaceID)
}

// GetWorkspaceByPath retrieves a workspace by path
func (s *Service) GetWorkspaceByPath(ctx context.Context, path string) (*Workspace, error) {
	// Normalize path to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	ws, err := s.repo.GetWorkspaceByPath(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %w", err)
	}
	return ws, nil
}

// ListWorkspaces returns all workspaces
func (s *Service) ListWorkspaces(ctx context.Context) ([]*Workspace, error) {
	workspaces, err := s.repo.ListWorkspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspaces: %w", err)
	}
	return workspaces, nil
}

// UpdateWorkspace updates an existing workspace
func (s *Service) UpdateWorkspace(ctx context.Context, workspace *Workspace) error {
	if err := s.repo.UpdateWorkspace(ctx, workspace); err != nil {
		return fmt.Errorf("failed to update workspace: %w", err)
	}

	s.logger.Info("Updated workspace", "id", workspace.ID, "name", workspace.Name)
	return nil
}

// DeleteWorkspace deletes a workspace by ID
func (s *Service) DeleteWorkspace(ctx context.Context, id string) error {
	if err := s.repo.DeleteWorkspace(ctx, id); err != nil {
		return fmt.Errorf("failed to delete workspace: %w", err)
	}

	s.logger.Info("Deleted workspace", "id", id)
	return nil
}

// GetCurrentWorkspace retrieves or creates a workspace for the current directory
func (s *Service) GetCurrentWorkspace(ctx context.Context, cfg *config.Config) (*Workspace, error) {
	// Get current directory
	currentDir, err := filepath.Abs(".")
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Try to find workspace for current directory
	workspace, err := s.repo.GetWorkspaceByPath(ctx, currentDir)
	if err != nil {
		if !errors.Is(err, ErrWorkspaceNotFound) {
			return nil, fmt.Errorf("failed to check for workspace: %w", err)
		}

		// If workspace doesn't exist and auto-create is enabled, create it
		if cfg.Workspace.AutoCreate {
			s.logger.Info("No workspace found for current directory, creating one", "path", currentDir)

			// Use directory name as workspace name
			dirName := filepath.Base(currentDir)

			workspace, err = s.CreateWorkspace(ctx, currentDir, dirName, cfg, "", "")
			if err != nil {
				return nil, fmt.Errorf("failed to create workspace: %w", err)
			}

			return workspace, nil
		}

		return nil, fmt.Errorf("no workspace found for directory %s (auto-create disabled)", currentDir)
	}

	// Initialize Git repository for existing workspace
	if err := s.initGitRepo(workspace.Path); err != nil {
		s.logger.Warn("Failed to initialize Git repository for existing workspace",
			"path", workspace.Path,
			"error", err)
	}

	s.logger.Info("Using workspace", "id", workspace.ID, "name", workspace.Name)
	return workspace, nil
}

// UpdateModelConfig updates the model configuration for a workspace
func (s *Service) UpdateModelConfig(ctx context.Context, workspaceID string, cfg *config.Config) error {
	workspace, err := s.repo.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get workspace: %w", err)
	}

	if err := workspace.UpdateModelConfig(cfg); err != nil {
		return fmt.Errorf("failed to update model config: %w", err)
	}

	if err := s.repo.UpdateWorkspace(ctx, workspace); err != nil {
		return fmt.Errorf("failed to save workspace: %w", err)
	}

	s.logger.Info("Updated model configuration for workspace", "id", workspace.ID)
	return nil
}

// ------------------------------------------
// File and Chunk Operations
// ------------------------------------------

// GetFile gets a file by its ID
func (s *Service) GetFile(ctx context.Context, fileID string) (*File, error) {
	return s.repo.GetFileByID(ctx, fileID)
}

// GetFileByPath gets a file by its path within a workspace
func (s *Service) GetFileByPath(ctx context.Context, workspaceID, filePath string) (*File, error) {
	return s.repo.GetFileByPath(ctx, workspaceID, filePath)
}

// ListFiles lists all files in a workspace
func (s *Service) ListFiles(ctx context.Context, workspaceID string) ([]*File, error) {
	return s.repo.GetFilesByWorkspaceID(ctx, workspaceID)
}

// DeleteFile deletes a file and its chunks
func (s *Service) DeleteFile(ctx context.Context, fileID string) error {
	return s.repo.DeleteFile(ctx, fileID)
}

// GetChunk gets a chunk by its ID
func (s *Service) GetChunk(ctx context.Context, chunkID string) (*Chunk, error) {
	return s.repo.GetChunkByID(ctx, chunkID)
}

// GetChunksByFile gets all chunks for a file
func (s *Service) GetChunksByFile(ctx context.Context, fileID string) ([]*Chunk, error) {
	return s.repo.GetChunksByFileID(ctx, fileID)
}

// GetChunksByWorkspace gets all chunks for a workspace
func (s *Service) GetChunksByWorkspace(ctx context.Context, workspaceID string) ([]*Chunk, error) {
	return s.repo.GetChunksByWorkspaceID(ctx, workspaceID)
}

// GetChunksByType gets all chunks of a specific type for a workspace
func (s *Service) GetChunksByType(ctx context.Context, workspaceID string, chunkType ChunkType) ([]*Chunk, error) {
	return s.repo.GetChunksByType(ctx, workspaceID, chunkType)
}

// GetChunksByReviewFile gets all chunks for a review file
func (s *Service) GetChunkIDs(ctx context.Context, chunks []*Chunk) []string {
	chunkIDs := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkIDs[i] = chunk.ID
	}
	return chunkIDs
}

// shouldParseFile determines if a file should be parsed
func (s *Service) shouldParseFile(filePath string) bool {
	// Check if file exists and is readable
	if _, err := os.Stat(filePath); err != nil {
		s.logger.Debug("File does not exist or cannot be accessed", "path", filePath, "error", err)
		return false
	}

	// Check if it's a code file
	result := s.parserService.IsCodeFile(filePath)
	return result
}

// ParseFile parses a single file and saves the chunks
func (s *Service) ParseFile(ctx context.Context, workspaceID string, filePath string) (*File, []*Chunk, error) {
	// Get workspace
	workspace, err := s.repo.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("getting workspace: %w", err)
	}

	// Create absolute path by joining workspace path and file path
	absPath := filepath.Join(workspace.Path, filePath)

	// Detect language
	language, err := s.parserService.DetectLanguage(absPath)
	if err != nil {
		return nil, nil, fmt.Errorf("detecting language: %w", err)
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
		return nil, nil, fmt.Errorf("parsing file: %w", err)
	}

	// Convert raw chunks to our model using two-pass approach
	chunks := make([]*Chunk, 0, len(rawChunks))
	idMap := make(map[string]string) // Map from temp parser IDs to real ULID chunk IDs

	// First pass: create chunks and build ID mapping
	for _, rawChunk := range rawChunks {
		chunk := NewChunkFromRawChunk(workspaceID, file.ID, rawChunk)
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
		return nil, nil, fmt.Errorf("saving file: %w", err)
	}

	// Save chunks
	err = s.repo.SaveChunksForFile(ctx, file, chunks)
	if err != nil {
		return nil, nil, fmt.Errorf("saving chunks: %w", err)
	}

	return file, chunks, nil
}

// ParseFiles parses multiple files and returns all chunks
func (s *Service) ParseFiles(ctx context.Context, workspaceID string, filePaths []string) ([]*Chunk, error) {
	allChunks := make([]*Chunk, 0)

	for _, filePath := range filePaths {
		// Skip files that shouldn't be parsed
		if !s.shouldParseFile(filePath) {
			s.logger.Debug("Skipping file", "path", filePath)
			continue
		}

		_, chunks, err := s.ParseFile(ctx, workspaceID, filePath)
		if err != nil {
			s.logger.Warn("Error parsing file", "path", filePath, "error", err)
			continue
		}

		allChunks = append(allChunks, chunks...)
	}

	return allChunks, nil
}

// RefreshFile re-parses a file if needed
func (s *Service) RefreshFile(ctx context.Context, workspaceID, filePath string) (*File, []*Chunk, error) {
	// Get workspace
	workspace, err := s.repo.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("getting workspace: %w", err)
	}

	// Create absolute path
	absPath := filepath.Join(workspace.Path, filePath)

	// Get or create the file
	file, err := s.repo.GetFileByPath(ctx, workspaceID, filePath)
	if err != nil {
		// Create a new file
		language, err := s.parserService.DetectLanguage(absPath)
		if err != nil {
			return nil, nil, fmt.Errorf("detecting language: %w", err)
		}
		file = NewFile(workspaceID, filePath, language)
	}

	// Check if we need to reparse (no LastParsed or file was updated since last parse)
	needsReparsing := file.LastParsed == nil
	if !needsReparsing {
		// Check if file on disk is newer than last parsed time
		fileInfo, err := os.Stat(absPath)
		if err == nil && fileInfo.ModTime().After(*file.LastParsed) {
			needsReparsing = true
		}
	}

	var chunks []*Chunk
	if needsReparsing {
		// Parse the file using the parser service
		rawChunks, _, err := s.parserService.ParseFile(absPath)
		if err != nil {
			return file, nil, fmt.Errorf("parsing file: %w", err)
		}

		// Set the LastParsed time to now
		now := time.Now()
		file.LastParsed = &now

		// Save the file with updated LastParsed time
		err = s.repo.SaveFile(ctx, file)
		if err != nil {
			return file, nil, fmt.Errorf("saving file: %w", err)
		}

		// Convert raw chunks to our model using two-pass approach
		chunks = make([]*Chunk, 0, len(rawChunks))
		idMap := make(map[string]string) // Map from temp parser IDs to real ULID chunk IDs

		// First pass: create chunks and build ID mapping
		for _, rawChunk := range rawChunks {
			chunk := NewChunkFromRawChunk(file.WorkspaceID, file.ID, rawChunk)
			chunks = append(chunks, chunk)

			// Store the mapping from parser ID to chunk ID
			idMap[rawChunk.ID] = chunk.ID
		}

		// Second pass: update parent/child relationships with real ULIDs
		s.logger.Debug("Starting second pass in RefreshFile: updating parent-child relationships", "chunks_count", len(chunks))
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

		// Save the chunks
		err = s.repo.SaveChunksForFile(ctx, file, chunks)
		if err != nil {
			return file, nil, fmt.Errorf("saving chunks: %w", err)
		}
	} else {
		// Get existing chunks
		chunks, err = s.repo.GetChunksByFileID(ctx, file.ID)
		if err != nil {
			return file, nil, fmt.Errorf("getting chunks: %w", err)
		}
	}

	return file, chunks, nil
}

// ParseChangedFiles parses the files in a diff and returns file IDs
func (s *Service) ParseChangedFiles(ctx context.Context, workspaceID string, diffResult *git.DiffResult) ([]string, error) {
	var fileIDs []string

	// Get the workspace
	workspace, err := s.repo.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		s.logger.Debug("Error getting workspace", "error", err)
		return nil, fmt.Errorf("getting workspace: %w", err)
	}

	// Process each file in the diff
	for i, file := range diffResult.Files {
		// Debug log
		s.logger.Debug("Processing file",
			"index", i,
			"path", file.Path,
			"changeType", file.ChangeType)

		// Skip deleted files
		if file.ChangeType == git.ChangeTypeDeleted {
			s.logger.Debug("Skipping deleted file", "path", file.Path)
			continue
		}

		// Determine if path is absolute or relative
		var absFilePath string
		var relFilePath string

		if filepath.IsAbs(file.Path) {
			// File path is already absolute
			absFilePath = file.Path
			// Calculate relative path from workspace
			relPath, err := filepath.Rel(workspace.Path, file.Path)
			if err != nil {
				s.logger.Warn("Failed to get relative path",
					"abs_path", file.Path,
					"workspace_path", workspace.Path,
					"error", err)
				// Use the basename as a fallback
				relFilePath = filepath.Base(file.Path)
			} else {
				relFilePath = relPath
			}
		} else {
			// Build absolute path from relative
			absFilePath = filepath.Join(workspace.Path, file.Path)
			relFilePath = file.Path
		}

		// Skip files that shouldn't be parsed
		shouldParse := s.shouldParseFile(absFilePath)

		if !shouldParse {
			s.logger.Debug("Skipping non-code file", "path", file.Path)
			continue
		}

		// Parse the file
		parsedFile, _, err := s.ParseFile(ctx, workspaceID, relFilePath)
		if err != nil {
			s.logger.Warn("Failed to parse file",
				"path", file.Path,
				"rel_path", relFilePath,
				"abs_path", absFilePath,
				"error", err)
			continue
		}

		fileIDs = append(fileIDs, parsedFile.ID)
	}

	s.logger.Debug("Finished parsing files",
		"total_processed", len(fileIDs),
		"file_ids", fileIDs)
	return fileIDs, nil
}

// ParseStagedChanges parses staged changes in a Git repository and returns file IDs
func (s *Service) ParseStagedChanges(ctx context.Context, workspaceID string, repoPath string) ([]string, error) {
	// Initialize Git repository before getting staged changes
	if err := s.initGitRepo(repoPath); err != nil {
		return nil, fmt.Errorf("initializing git repository: %w", err)
	}

	// Get staged changes from git service
	diffResult, err := s.gitService.GetDiff(git.DiffRequest{
		RepoPath: repoPath,
		DiffType: git.DiffTypeStaged,
	})

	if err != nil {
		s.logger.Debug("Error getting staged changes", "error", err)
		return nil, fmt.Errorf("getting staged changes: %w", err)
	}

	// Check if we have any changes
	if len(diffResult.Files) == 0 {
		s.logger.Debug("No staged changes found")
		return []string{}, nil
	}

	// Update workspace path to use the repo path for correct abs path construction
	workspace, err := s.repo.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		s.logger.Debug("Error getting workspace", "error", err)
		return nil, fmt.Errorf("getting workspace: %w", err)
	}

	// Use a clone of the diffResult with modified file paths
	modifiedDiff := &git.DiffResult{
		Files: make([]git.ChangedFile, len(diffResult.Files)),
	}

	// Get the workspace path and compare with repoPath
	if workspace.Path != repoPath {
		// Clone the diff result with absolute paths
		for i, file := range diffResult.Files {
			modifiedDiff.Files[i] = file

			// Create absolute path using repoPath instead of workspace.Path
			absPath := filepath.Join(repoPath, file.Path)
			_, err := os.Stat(absPath)
			fileExists := err == nil
			s.logger.Debug("Using absolute path from target repo",
				"file", file.Path,
				"abs_path", absPath,
				"file_exists", fileExists)

			// Update file path to use absolute path
			modifiedDiff.Files[i].Path = absPath
		}

		// Parse the files in the diff using the modified diff
		return s.ParseChangedFiles(ctx, workspaceID, modifiedDiff)
	}

	// Parse the files in the diff
	return s.ParseChangedFiles(ctx, workspaceID, diffResult)
}

// ParseCommitChanges parses changes in a specific Git commit and returns file IDs
func (s *Service) ParseCommitChanges(ctx context.Context, workspaceID string, repoPath string, commitHash string) ([]string, error) {
	// Initialize Git repository before getting commit changes
	if err := s.initGitRepo(repoPath); err != nil {
		return nil, fmt.Errorf("initializing git repository: %w", err)
	}

	// Get commit changes from git service
	diffResult, err := s.gitService.GetDiff(git.DiffRequest{
		RepoPath: repoPath,
		DiffType: git.DiffTypeCommit,
		CommitID: commitHash,
	})
	if err != nil {
		return nil, fmt.Errorf("getting commit changes: %w", err)
	}

	// Parse the files in the diff
	return s.ParseChangedFiles(ctx, workspaceID, diffResult)
}

// ParseBranchChanges parses changes between two Git branches and returns file IDs
func (s *Service) ParseBranchChanges(ctx context.Context, workspaceID string, repoPath string, baseBranch string, compareBranch string) ([]string, error) {
	// Initialize Git repository before getting branch changes
	if err := s.initGitRepo(repoPath); err != nil {
		return nil, fmt.Errorf("initializing git repository: %w", err)
	}

	// Get branch changes from git service
	diffResult, err := s.gitService.GetDiff(git.DiffRequest{
		RepoPath:  repoPath,
		DiffType:  git.DiffTypeBranch,
		BranchOne: baseBranch,
		BranchTwo: compareBranch,
	})
	if err != nil {
		return nil, fmt.Errorf("getting branch diff: %w", err)
	}

	// Parse the files in the diff
	return s.ParseChangedFiles(ctx, workspaceID, diffResult)
}

// HasGitRepo checks if the provided path contains a valid Git repository
func (s *Service) HasGitRepo(path string) bool {
	// Try to initialize the Git repository
	err := s.initGitRepo(path)
	return err == nil
}
