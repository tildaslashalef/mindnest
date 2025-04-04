package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/ulid"
)

var (
	// ErrWorkspaceNotFound is returned when a workspace is not found
	ErrWorkspaceNotFound = errors.New("workspace not found")

	// ErrWorkspaceAlreadyExists is returned when a workspace already exists with the same path
	ErrWorkspaceAlreadyExists = errors.New("workspace already exists")
)

// PaginationParams defines parameters for paginated queries
type PaginationParams struct {
	Page  int
	Limit int
}

// NewPaginationParams creates a new PaginationParams instance with default values
func NewPaginationParams(page, limit int) PaginationParams {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10 // Default to 10 items per page
	}
	if limit > 100 {
		limit = 100 // Cap at 100 items per page
	}
	return PaginationParams{
		Page:  page,
		Limit: limit,
	}
}

// Repository defines the interface for workspace persistence operations
type Repository interface {
	// Workspace operations
	CreateWorkspace(ctx context.Context, workspace *Workspace) error
	GetWorkspaceByID(ctx context.Context, id string) (*Workspace, error)
	GetWorkspaceByPath(ctx context.Context, path string) (*Workspace, error)
	ListWorkspaces(ctx context.Context) ([]*Workspace, error)
	ListWorkspacesWithPagination(ctx context.Context, params PaginationParams) ([]*Workspace, error)
	UpdateWorkspace(ctx context.Context, workspace *Workspace) error
	DeleteWorkspace(ctx context.Context, id string) error
	FindWorkspacesByName(ctx context.Context, searchTerm string) ([]*Workspace, error)
	DuplicateWorkspace(ctx context.Context, id string, newName string) (*Workspace, error)
	GetWorkspaceIssues(ctx context.Context, workspaceID string) ([]*Issue, error)

	// File operations
	SaveFile(ctx context.Context, file *File) error
	GetFileByID(ctx context.Context, fileID string) (*File, error)
	GetFileByPath(ctx context.Context, workspaceID, filePath string) (*File, error)
	UpdateFile(ctx context.Context, file *File) error
	DeleteFile(ctx context.Context, fileID string) error
	GetFilesByWorkspaceID(ctx context.Context, workspaceID string) ([]*File, error)

	// Chunk operations
	SaveChunk(ctx context.Context, chunk *Chunk) error
	GetChunkByID(ctx context.Context, chunkID string) (*Chunk, error)
	GetChunksByFileID(ctx context.Context, fileID string) ([]*Chunk, error)
	GetChunksByWorkspaceID(ctx context.Context, workspaceID string) ([]*Chunk, error)
	GetChunksByType(ctx context.Context, workspaceID string, chunkType ChunkType) ([]*Chunk, error)
	UpdateChunk(ctx context.Context, chunk *Chunk) error
	DeleteChunk(ctx context.Context, chunkID string) error
	DeleteChunksByFileID(ctx context.Context, fileID string) error
	SaveChunksForFile(ctx context.Context, file *File, chunks []*Chunk) error
}

// SQLRepository implements Repository using SQLite database
type SQLRepository struct {
	db      *sql.DB
	logger  *loggy.Logger
	builder sq.StatementBuilderType
}

// NewSQLRepository creates a new workspace SQL repository
func NewSQLRepository(db *sql.DB, logger *loggy.Logger) Repository {
	return &SQLRepository{
		db:      db,
		logger:  logger,
		builder: sq.StatementBuilder.PlaceholderFormat(sq.Question),
	}
}

// CreateWorkspace saves a new workspace to the database
func (r *SQLRepository) CreateWorkspace(ctx context.Context, workspace *Workspace) error {
	// Check if a workspace with the same path already exists
	existing, err := r.GetWorkspaceByPath(ctx, workspace.Path)
	if err != nil && !errors.Is(err, ErrWorkspaceNotFound) {
		return fmt.Errorf("checking for existing workspace: %w", err)
	}

	if existing != nil {
		return ErrWorkspaceAlreadyExists
	}

	now := time.Now()
	if workspace.CreatedAt.IsZero() {
		workspace.CreatedAt = now
	}
	if workspace.UpdatedAt.IsZero() {
		workspace.UpdatedAt = now
	}

	// Using Squirrel to build the insert query
	query, args, err := r.builder.
		Insert("workspaces").
		Columns(
			"id",
			"name",
			"path",
			"git_repo_url",
			"description",
			"model_config",
			"created_at",
			"updated_at",
		).
		Values(
			workspace.ID,
			workspace.Name,
			workspace.Path,
			workspace.GitRepoURL,
			workspace.Description,
			workspace.ModelConfig,
			workspace.CreatedAt,
			workspace.UpdatedAt,
		).
		ToSql()

	if err != nil {
		return fmt.Errorf("building insert query: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("inserting workspace: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no rows affected when creating workspace")
	}

	r.logger.Info("Created workspace", "id", workspace.ID, "name", workspace.Name, "path", workspace.Path)
	return nil
}

// GetWorkspaceByID retrieves a workspace by its ID
func (r *SQLRepository) GetWorkspaceByID(ctx context.Context, id string) (*Workspace, error) {
	// Using Squirrel to build the select query
	query, args, err := r.builder.
		Select(
			"id",
			"name",
			"path",
			"git_repo_url",
			"description",
			"model_config",
			"created_at",
			"updated_at",
		).
		From("workspaces").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("building select query: %w", err)
	}

	row := r.db.QueryRowContext(ctx, query, args...)
	workspace, err := scanWorkspace(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorkspaceNotFound
		}
		return nil, fmt.Errorf("scanning workspace: %w", err)
	}

	return workspace, nil
}

// GetWorkspaceByPath retrieves a workspace by its path
func (r *SQLRepository) GetWorkspaceByPath(ctx context.Context, path string) (*Workspace, error) {
	// Normalize path to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path: %w", err)
	}

	// Using Squirrel to build the select query
	query, args, err := r.builder.
		Select(
			"id",
			"name",
			"path",
			"git_repo_url",
			"description",
			"model_config",
			"created_at",
			"updated_at",
		).
		From("workspaces").
		Where(sq.Eq{"path": absPath}).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("building select query: %w", err)
	}

	row := r.db.QueryRowContext(ctx, query, args...)
	workspace, err := scanWorkspace(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorkspaceNotFound
		}
		return nil, fmt.Errorf("scanning workspace: %w", err)
	}

	return workspace, nil
}

// ListWorkspaces returns all workspaces
func (r *SQLRepository) ListWorkspaces(ctx context.Context) ([]*Workspace, error) {
	// Using Squirrel to build the select query with ordering
	query, args, err := r.builder.
		Select(
			"id",
			"name",
			"path",
			"git_repo_url",
			"description",
			"model_config",
			"created_at",
			"updated_at",
		).
		From("workspaces").
		OrderBy("updated_at DESC").
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("building select query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying for workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []*Workspace
	for rows.Next() {
		workspace, err := scanWorkspaceFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning workspace: %w", err)
		}
		workspaces = append(workspaces, workspace)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return workspaces, nil
}

// ListWorkspacesWithPagination returns workspaces with pagination
func (r *SQLRepository) ListWorkspacesWithPagination(ctx context.Context, params PaginationParams) ([]*Workspace, error) {
	// Create the base query
	query := r.builder.
		Select(
			"id",
			"name",
			"path",
			"git_repo_url",
			"description",
			"model_config",
			"created_at",
			"updated_at",
		).
		From("workspaces").
		OrderBy("updated_at DESC")

	// Add pagination
	if params.Limit > 0 {
		query = query.Limit(uint64(params.Limit))
		if params.Page > 0 {
			offset := uint64((params.Page - 1) * params.Limit)
			query = query.Offset(offset)
		}
	}

	// Convert to SQL
	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building paginated query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("executing paginated query: %w", err)
	}
	defer rows.Close()

	var workspaces []*Workspace
	for rows.Next() {
		workspace, err := scanWorkspaceFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning workspace: %w", err)
		}
		workspaces = append(workspaces, workspace)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return workspaces, nil
}

// UpdateWorkspace updates an existing workspace
func (r *SQLRepository) UpdateWorkspace(ctx context.Context, workspace *Workspace) error {
	// Ensure timestamp is in UTC format
	updatedAt := workspace.UpdatedAt.UTC().Format(time.RFC3339)

	// Using Squirrel to build the update query
	query, args, err := r.builder.
		Update("workspaces").
		Set("name", workspace.Name).
		Set("path", workspace.Path).
		Set("git_repo_url", workspace.GitRepoURL).
		Set("description", workspace.Description).
		Set("model_config", workspace.ModelConfig).
		Set("updated_at", updatedAt).
		Where(sq.Eq{"id": workspace.ID}).
		ToSql()

	if err != nil {
		return fmt.Errorf("building update query: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating workspace: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrWorkspaceNotFound
	}

	r.logger.Info("Updated workspace", "id", workspace.ID, "name", workspace.Name)
	return nil
}

// DeleteWorkspace deletes a workspace and all associated data by its ID
func (r *SQLRepository) DeleteWorkspace(ctx context.Context, id string) error {
	// Start a transaction to ensure atomicity
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Using Squirrel to build the delete query
	query, args, err := r.builder.
		Delete("workspaces").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return fmt.Errorf("building delete query: %w", err)
	}

	// Execute within transaction
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("deleting workspace: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrWorkspaceNotFound
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	r.logger.Info("Deleted workspace and all associated data",
		"id", id,
		"cascade_deleted", "files, chunks, reviews, issues, review_files")
	return nil
}

// FindWorkspacesByName searches for workspaces with names containing the search term
func (r *SQLRepository) FindWorkspacesByName(ctx context.Context, searchTerm string) ([]*Workspace, error) {
	// Using Squirrel to build a query with a LIKE condition
	query, args, err := r.builder.
		Select(
			"id",
			"name",
			"path",
			"git_repo_url",
			"description",
			"model_config",
			"created_at",
			"updated_at",
		).
		From("workspaces").
		Where(sq.Like{"name": "%" + searchTerm + "%"}).
		OrderBy("updated_at DESC").
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("building search query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("searching for workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []*Workspace
	for rows.Next() {
		workspace, err := scanWorkspaceFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning workspace: %w", err)
		}
		workspaces = append(workspaces, workspace)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return workspaces, nil
}

// DuplicateWorkspace creates a duplicate of an existing workspace
// This operation uses a transaction to ensure atomicity
func (r *SQLRepository) DuplicateWorkspace(ctx context.Context, id string, newName string) (*Workspace, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var duplicatedWorkspace *Workspace

	// First, get the original workspace
	original, err := r.GetWorkspaceByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting original workspace: %w", err)
	}

	// Create a new workspace with the same properties but a new ID and name
	duplicatedWorkspace = &Workspace{
		ID:          ulid.WorkspaceID(),
		Name:        newName,
		Path:        original.Path + "_dup", // Modify path to avoid conflicts
		GitRepoURL:  original.GitRepoURL,
		Description: "Duplicated from " + original.Name,
		ModelConfig: original.ModelConfig,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Insert the new workspace
	createdAt := duplicatedWorkspace.CreatedAt.UTC().Format(time.RFC3339)
	updatedAt := duplicatedWorkspace.UpdatedAt.UTC().Format(time.RFC3339)

	query, args, err := r.builder.
		Insert("workspaces").
		Columns(
			"id",
			"name",
			"path",
			"git_repo_url",
			"description",
			"model_config",
			"created_at",
			"updated_at",
		).
		Values(
			duplicatedWorkspace.ID,
			duplicatedWorkspace.Name,
			duplicatedWorkspace.Path,
			duplicatedWorkspace.GitRepoURL,
			duplicatedWorkspace.Description,
			duplicatedWorkspace.ModelConfig,
			createdAt,
			updatedAt,
		).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("building insert query: %w", err)
	}

	// Execute within the transaction
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("inserting duplicated workspace: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("no rows affected when creating duplicated workspace")
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	r.logger.Info("Duplicated workspace",
		"original_id", id,
		"new_id", duplicatedWorkspace.ID,
		"new_name", duplicatedWorkspace.Name)

	return duplicatedWorkspace, nil
}

// GetWorkspaceIssues retrieves all issues for a workspace
func (r *SQLRepository) GetWorkspaceIssues(ctx context.Context, workspaceID string) ([]*Issue, error) {
	// Join issues with reviews to get issues for the workspace
	query, args, err := r.builder.
		Select(
			"i.id",
			"i.review_id",
			"i.file_id",
			"i.type",
			"i.severity",
			"i.title",
			"i.description",
			"i.line_start",
			"i.line_end",
			"i.suggestion",
			"i.affected_code",
			"i.code_snippet",
			"i.is_valid",
			"i.created_at",
			"i.updated_at",
			"f.path AS file_path", // Join with files to get the file path
		).
		From("issues i").
		Join("reviews r ON i.review_id = r.id").
		Join("files f ON i.file_id = f.id").
		Where(sq.Eq{"r.workspace_id": workspaceID}).
		OrderBy("i.created_at DESC").
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("building query for workspace issues: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("executing query for workspace issues: %w", err)
	}
	defer rows.Close()

	var issues []*Issue
	for rows.Next() {
		var issue Issue
		var filePath string
		var createdAt, updatedAt string

		err := rows.Scan(
			&issue.ID,
			&issue.ReviewID,
			&issue.FileID,
			&issue.Type,
			&issue.Severity,
			&issue.Title,
			&issue.Description,
			&issue.LineStart,
			&issue.LineEnd,
			&issue.Suggestion,
			&issue.AffectedCode,
			&issue.CodeSnippet,
			&issue.IsValid,
			&createdAt,
			&updatedAt,
			&filePath,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning issue row: %w", err)
		}

		// Parse timestamps
		if issue.CreatedAt, err = time.Parse(time.RFC3339, createdAt); err != nil {
			return nil, fmt.Errorf("parsing created_at: %w", err)
		}
		if issue.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt); err != nil {
			return nil, fmt.Errorf("parsing updated_at: %w", err)
		}

		// Initialize metadata and store file path
		if issue.Metadata == nil {
			issue.Metadata = make(map[string]interface{})
		}
		issue.Metadata["file_path"] = filePath

		issues = append(issues, &issue)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating issue rows: %w", err)
	}

	return issues, nil
}

// Private helper functions

// scanWorkspace scans a workspace from a row
func scanWorkspace(row *sql.Row) (*Workspace, error) {
	var workspace Workspace
	var createdAtStr, updatedAtStr string

	err := row.Scan(
		&workspace.ID,
		&workspace.Name,
		&workspace.Path,
		&workspace.GitRepoURL,
		&workspace.Description,
		&workspace.ModelConfig,
		&createdAtStr,
		&updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	// Parse the time strings
	workspace.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}

	workspace.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", err)
	}

	return &workspace, nil
}

// scanWorkspaceFromRows scans a workspace from a rows object
func scanWorkspaceFromRows(rows *sql.Rows) (*Workspace, error) {
	var workspace Workspace
	var createdAtStr, updatedAtStr string

	err := rows.Scan(
		&workspace.ID,
		&workspace.Name,
		&workspace.Path,
		&workspace.GitRepoURL,
		&workspace.Description,
		&workspace.ModelConfig,
		&createdAtStr,
		&updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	// Parse the time strings
	workspace.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}

	workspace.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", err)
	}

	return &workspace, nil
}
