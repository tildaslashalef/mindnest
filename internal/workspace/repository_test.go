package workspace

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	sq "github.com/Masterminds/squirrel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// testSQLRepository is a wrapper around SQLRepository for testing
type testSQLRepository struct {
	*SQLRepository
}

// NewTestSQLRepository creates a new test repository instance
func NewTestSQLRepository(db *sql.DB) *testSQLRepository {
	// Create a noop logger for testing
	logger := loggy.NewNoopLogger()

	return &testSQLRepository{
		SQLRepository: &SQLRepository{
			db:      db,
			logger:  logger,
			builder: sq.StatementBuilder.PlaceholderFormat(sq.Question),
		},
	}
}

// Helper function to create a proper raw message value that can be used with sqlmock
type rawMessageConverter struct{}

func (m rawMessageConverter) ConvertValue(v interface{}) (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func TestWorkspaceRepository(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "Failed to create mock database")
	defer db.Close()

	// Create test repository with the mock database
	repo := NewTestSQLRepository(db)

	// Sample workspace data with proper JSON marshaling for model config
	modelConfig := map[string]interface{}{
		"model": "test-model",
	}

	modelConfigJSON, err := json.Marshal(modelConfig)
	require.NoError(t, err, "Failed to marshal model config to JSON")

	sampleWorkspace := &Workspace{
		ID:          "ws_123456",
		Name:        "Test Workspace",
		Path:        "/path/to/workspace",
		GitRepoURL:  "https://github.com/example/repo",
		Description: "Test description",
		ModelConfig: modelConfigJSON,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Test CreateWorkspace
	t.Run("CreateWorkspace", func(t *testing.T) {
		// Setup expected query
		mock.ExpectQuery("SELECT .+ FROM workspaces WHERE path = ?").
			WithArgs(sampleWorkspace.Path).
			WillReturnError(sql.ErrNoRows)

		mock.ExpectExec("INSERT INTO workspaces").
			WithArgs(
				sampleWorkspace.ID,
				sampleWorkspace.Name,
				sampleWorkspace.Path,
				sampleWorkspace.GitRepoURL,
				sampleWorkspace.Description,
				sampleWorkspace.ModelConfig,
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Execute test
		err := repo.CreateWorkspace(context.Background(), sampleWorkspace)
		assert.NoError(t, err, "CreateWorkspace should not return an error")

		// Check all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	// Test GetWorkspaceByID
	t.Run("GetWorkspaceByID", func(t *testing.T) {
		// Setup expected query
		rows := sqlmock.NewRows([]string{
			"id", "name", "path", "git_repo_url", "description", "model_config", "created_at", "updated_at",
		}).AddRow(
			sampleWorkspace.ID,
			sampleWorkspace.Name,
			sampleWorkspace.Path,
			sampleWorkspace.GitRepoURL,
			sampleWorkspace.Description,
			modelConfigJSON,
			sampleWorkspace.CreatedAt.Format(time.RFC3339),
			sampleWorkspace.UpdatedAt.Format(time.RFC3339),
		)

		mock.ExpectQuery("SELECT .+ FROM workspaces WHERE id = ?").
			WithArgs(sampleWorkspace.ID).
			WillReturnRows(rows)

		// Execute test
		workspace, err := repo.GetWorkspaceByID(context.Background(), sampleWorkspace.ID)
		assert.NoError(t, err, "GetWorkspaceByID should not return an error")
		assert.NotNil(t, workspace, "Workspace should not be nil")
		assert.Equal(t, sampleWorkspace.ID, workspace.ID, "Workspace ID should match")
		assert.Equal(t, sampleWorkspace.Name, workspace.Name, "Workspace name should match")

		// Check all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	// Test GetWorkspaceByPath
	t.Run("GetWorkspaceByPath", func(t *testing.T) {
		// Setup expected query for filepath.Abs
		mock.ExpectQuery("SELECT .+ FROM workspaces WHERE path = ?").
			WithArgs(sampleWorkspace.Path).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "name", "path", "git_repo_url", "description", "model_config", "created_at", "updated_at",
			}).AddRow(
				sampleWorkspace.ID,
				sampleWorkspace.Name,
				sampleWorkspace.Path,
				sampleWorkspace.GitRepoURL,
				sampleWorkspace.Description,
				modelConfigJSON,
				sampleWorkspace.CreatedAt.Format(time.RFC3339),
				sampleWorkspace.UpdatedAt.Format(time.RFC3339),
			))

		// Execute test
		workspace, err := repo.GetWorkspaceByPath(context.Background(), sampleWorkspace.Path)
		assert.NoError(t, err, "GetWorkspaceByPath should not return an error")
		assert.NotNil(t, workspace, "Workspace should not be nil")
		assert.Equal(t, sampleWorkspace.ID, workspace.ID, "Workspace ID should match")
		assert.Equal(t, sampleWorkspace.Path, workspace.Path, "Workspace path should match")

		// Check all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	// Test ListWorkspaces
	t.Run("ListWorkspaces", func(t *testing.T) {
		// Setup expected query
		rows := sqlmock.NewRows([]string{
			"id", "name", "path", "git_repo_url", "description", "model_config", "created_at", "updated_at",
		}).AddRow(
			sampleWorkspace.ID,
			sampleWorkspace.Name,
			sampleWorkspace.Path,
			sampleWorkspace.GitRepoURL,
			sampleWorkspace.Description,
			modelConfigJSON,
			sampleWorkspace.CreatedAt.Format(time.RFC3339),
			sampleWorkspace.UpdatedAt.Format(time.RFC3339),
		)

		mock.ExpectQuery("SELECT .+ FROM workspaces").
			WillReturnRows(rows)

		// Execute test
		workspaces, err := repo.ListWorkspaces(context.Background())
		assert.NoError(t, err, "ListWorkspaces should not return an error")
		assert.Len(t, workspaces, 1, "Should return one workspace")
		assert.Equal(t, sampleWorkspace.ID, workspaces[0].ID, "Workspace ID should match")

		// Check all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	// Test ListWorkspacesWithPagination
	t.Run("ListWorkspacesWithPagination", func(t *testing.T) {
		paginationParams := PaginationParams{
			Page:  1,
			Limit: 10,
		}

		// Setup expected query
		rows := sqlmock.NewRows([]string{
			"id", "name", "path", "git_repo_url", "description", "model_config", "created_at", "updated_at",
		}).AddRow(
			sampleWorkspace.ID,
			sampleWorkspace.Name,
			sampleWorkspace.Path,
			sampleWorkspace.GitRepoURL,
			sampleWorkspace.Description,
			modelConfigJSON,
			sampleWorkspace.CreatedAt.Format(time.RFC3339),
			sampleWorkspace.UpdatedAt.Format(time.RFC3339),
		)

		mock.ExpectQuery("SELECT .+ FROM workspaces .+ LIMIT .+ OFFSET .+").
			WillReturnRows(rows)

		// Execute test
		workspaces, err := repo.ListWorkspacesWithPagination(context.Background(), paginationParams)
		assert.NoError(t, err, "ListWorkspacesWithPagination should not return an error")
		assert.Len(t, workspaces, 1, "Should return one workspace")
		assert.Equal(t, sampleWorkspace.ID, workspaces[0].ID, "Workspace ID should match")

		// Check all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	// Test UpdateWorkspace
	t.Run("UpdateWorkspace", func(t *testing.T) {
		// Setup expected query
		mock.ExpectExec("UPDATE workspaces SET").
			WithArgs(
				sampleWorkspace.Name,
				sampleWorkspace.Path,
				sampleWorkspace.GitRepoURL,
				sampleWorkspace.Description,
				sampleWorkspace.ModelConfig,
				sqlmock.AnyArg(),
				sampleWorkspace.ID,
			).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Execute test
		err := repo.UpdateWorkspace(context.Background(), sampleWorkspace)
		assert.NoError(t, err, "UpdateWorkspace should not return an error")

		// Check all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	// Test DeleteWorkspace
	t.Run("DeleteWorkspace", func(t *testing.T) {
		// Setup expected query with transaction
		mock.ExpectBegin()
		mock.ExpectExec("DELETE FROM workspaces WHERE id = ?").
			WithArgs(sampleWorkspace.ID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		// Execute test
		err := repo.DeleteWorkspace(context.Background(), sampleWorkspace.ID)
		assert.NoError(t, err, "DeleteWorkspace should not return an error")

		// Check all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	// Test FindWorkspacesByName
	t.Run("FindWorkspacesByName", func(t *testing.T) {
		searchTerm := "Test"

		// Setup expected query
		rows := sqlmock.NewRows([]string{
			"id", "name", "path", "git_repo_url", "description", "model_config", "created_at", "updated_at",
		}).AddRow(
			sampleWorkspace.ID,
			sampleWorkspace.Name,
			sampleWorkspace.Path,
			sampleWorkspace.GitRepoURL,
			sampleWorkspace.Description,
			modelConfigJSON,
			sampleWorkspace.CreatedAt.Format(time.RFC3339),
			sampleWorkspace.UpdatedAt.Format(time.RFC3339),
		)

		mock.ExpectQuery("SELECT .+ FROM workspaces WHERE name LIKE ?").
			WithArgs("%" + searchTerm + "%").
			WillReturnRows(rows)

		// Execute test
		workspaces, err := repo.FindWorkspacesByName(context.Background(), searchTerm)
		assert.NoError(t, err, "FindWorkspacesByName should not return an error")
		assert.Len(t, workspaces, 1, "Should return one workspace")
		assert.Equal(t, sampleWorkspace.ID, workspaces[0].ID, "Workspace ID should match")
		assert.Equal(t, sampleWorkspace.Name, workspaces[0].Name, "Workspace name should match")

		// Check all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	// Test DuplicateWorkspace
	t.Run("DuplicateWorkspace", func(t *testing.T) {
		newName := "Duplicated Workspace"

		// Mock begin transaction
		mock.ExpectBegin()

		// Setup expected queries for getting the original workspace
		rows := sqlmock.NewRows([]string{
			"id", "name", "path", "git_repo_url", "description", "model_config", "created_at", "updated_at",
		}).AddRow(
			sampleWorkspace.ID,
			sampleWorkspace.Name,
			sampleWorkspace.Path,
			sampleWorkspace.GitRepoURL,
			sampleWorkspace.Description,
			modelConfigJSON,
			sampleWorkspace.CreatedAt.Format(time.RFC3339),
			sampleWorkspace.UpdatedAt.Format(time.RFC3339),
		)

		mock.ExpectQuery("SELECT .+ FROM workspaces WHERE id = ?").
			WithArgs(sampleWorkspace.ID).
			WillReturnRows(rows)

		// Setup expected query for inserting the duplicate
		mock.ExpectExec("INSERT INTO workspaces").
			WithArgs(
				sqlmock.AnyArg(),
				newName,
				sampleWorkspace.Path+"_dup",
				sampleWorkspace.GitRepoURL,
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Setup commit expectation
		mock.ExpectCommit()

		// Execute test
		duplicated, err := repo.DuplicateWorkspace(context.Background(), sampleWorkspace.ID, newName)
		assert.NoError(t, err, "DuplicateWorkspace should not return an error")
		assert.NotNil(t, duplicated, "Duplicated workspace should not be nil")
		assert.Equal(t, newName, duplicated.Name, "Duplicated workspace name should match")
		assert.Equal(t, sampleWorkspace.Path+"_dup", duplicated.Path, "Duplicated workspace path should be appended with _dup")

		// Check all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
