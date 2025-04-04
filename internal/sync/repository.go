package sync

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/ulid"
)

// Repository defines operations for managing sync logs in the database
type Repository interface {
	// CreateSyncLog creates a new sync log
	CreateSyncLog(ctx context.Context, log *SyncLog) error

	// GetSyncLogs retrieves sync logs with optional filtering
	GetSyncLogs(ctx context.Context, entityType EntityType, entityID string, limit, offset int) ([]*SyncLog, error)

	// GetLatestSyncLog retrieves the latest sync log for an entity
	GetLatestSyncLog(ctx context.Context, entityType EntityType, entityID string) (*SyncLog, error)

	// UpdateEntitySyncStatus updates the synced_at timestamp for an entity
	UpdateEntitySyncStatus(ctx context.Context, entityType EntityType, entityID string) error

	// GetUnsyncedWorkspaces retrieves workspaces that need to be synced
	GetUnsyncedWorkspaces(ctx context.Context, limit int) ([]string, error)

	// GetUnsyncedReviews retrieves reviews that need to be synced
	GetUnsyncedReviews(ctx context.Context, workspaceID string, limit int) ([]string, error)

	// GetUnsyncedReviewFiles retrieves review files that need to be synced
	GetUnsyncedReviewFiles(ctx context.Context, reviewID string, limit int) ([]string, error)

	// GetUnsyncedIssues retrieves issues that need to be synced
	GetUnsyncedIssues(ctx context.Context, reviewID string, limit int) ([]string, error)

	// GetUnsyncedFiles retrieves files that need to be synced
	GetUnsyncedFiles(ctx context.Context, workspaceID string, limit int) ([]string, error)

	// NeedsSync checks if an entity needs to be synced
	NeedsSync(ctx context.Context, entityType EntityType, entityID string) (bool, error)
}

// SQLRepository implements the Repository interface using a SQL database
type SQLRepository struct {
	db     *sql.DB
	logger *loggy.Logger
}

// NewSQLRepository creates a new SQL repository
func NewSQLRepository(db *sql.DB, logger *loggy.Logger) *SQLRepository {
	return &SQLRepository{
		db:     db,
		logger: logger,
	}
}

// CreateSyncLog creates a new sync log
func (r *SQLRepository) CreateSyncLog(ctx context.Context, log *SyncLog) error {
	if log.ID == "" {
		log.ID = ulid.SyncID()
	}

	q := squirrel.Insert("sync_logs").
		Columns("id", "sync_type", "entity_type", "entity_id", "success", "error_message", "items_synced", "started_at", "completed_at").
		Values(log.ID, log.SyncType, log.EntityType, log.EntityID, log.Success, log.ErrorMessage, log.ItemsSynced, log.StartedAt, log.CompletedAt)

	query, args, err := q.ToSql()
	if err != nil {
		return fmt.Errorf("building create sync log query: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("executing create sync log query: %w", err)
	}

	return nil
}

// GetSyncLogs retrieves sync logs with optional filtering
func (r *SQLRepository) GetSyncLogs(ctx context.Context, entityType EntityType, entityID string, limit, offset int) ([]*SyncLog, error) {
	q := squirrel.Select("id", "sync_type", "entity_type", "entity_id", "success", "error_message", "items_synced", "started_at", "completed_at").
		From("sync_logs").
		OrderBy("completed_at DESC")

	if entityType != "" {
		q = q.Where(squirrel.Eq{"entity_type": entityType})
	}

	if entityID != "" {
		q = q.Where(squirrel.Eq{"entity_id": entityID})
	}

	if limit > 0 {
		q = q.Limit(uint64(limit))
	}
	if offset > 0 {
		q = q.Offset(uint64(offset))
	}

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building get sync logs query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("executing get sync logs query: %w", err)
	}
	defer rows.Close()

	var logs []*SyncLog
	for rows.Next() {
		var log SyncLog
		err := rows.Scan(
			&log.ID,
			&log.SyncType,
			&log.EntityType,
			&log.EntityID,
			&log.Success,
			&log.ErrorMessage,
			&log.ItemsSynced,
			&log.StartedAt,
			&log.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning sync log row: %w", err)
		}
		logs = append(logs, &log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating sync log rows: %w", err)
	}

	return logs, nil
}

// GetLatestSyncLog retrieves the latest sync log for an entity
func (r *SQLRepository) GetLatestSyncLog(ctx context.Context, entityType EntityType, entityID string) (*SyncLog, error) {
	q := squirrel.Select("id", "sync_type", "entity_type", "entity_id", "success", "error_message", "items_synced", "started_at", "completed_at").
		From("sync_logs").
		Where(squirrel.Eq{"entity_type": entityType, "entity_id": entityID}).
		OrderBy("completed_at DESC").
		Limit(1)

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building get latest sync log query: %w", err)
	}

	var log SyncLog
	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&log.ID,
		&log.SyncType,
		&log.EntityType,
		&log.EntityID,
		&log.Success,
		&log.ErrorMessage,
		&log.ItemsSynced,
		&log.StartedAt,
		&log.CompletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No sync log found
		}
		return nil, fmt.Errorf("executing get latest sync log query: %w", err)
	}

	return &log, nil
}

// UpdateEntitySyncStatus updates the synced_at timestamp for an entity
func (r *SQLRepository) UpdateEntitySyncStatus(ctx context.Context, entityType EntityType, entityID string) error {
	var table string
	switch entityType {
	case EntityTypeWorkspace:
		table = "workspaces"
	case EntityTypeReview:
		table = "reviews"
	case EntityTypeReviewFile:
		table = "review_files"
	case EntityTypeIssue:
		table = "issues"
	case EntityTypeFile:
		table = "files"
	default:
		return fmt.Errorf("unknown entity type: %s", entityType)
	}

	now := time.Now()
	q := squirrel.Update(table).
		Set("synced_at", now).
		Where(squirrel.Eq{"id": entityID})

	query, args, err := q.ToSql()
	if err != nil {
		return fmt.Errorf("building update entity sync status query: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("executing update entity sync status query: %w", err)
	}

	return nil
}

// GetUnsyncedWorkspaces retrieves workspaces that need to be synced
func (r *SQLRepository) GetUnsyncedWorkspaces(ctx context.Context, limit int) ([]string, error) {
	q := squirrel.Select("id").
		From("workspaces").
		Where(squirrel.Or{
			squirrel.Eq{"synced_at": nil},
			squirrel.Expr("synced_at < updated_at"),
		}).
		OrderBy("updated_at DESC")

	if limit > 0 {
		q = q.Limit(uint64(limit))
	}

	return r.getUnsyncedEntities(ctx, q)
}

// GetUnsyncedReviews retrieves reviews that need to be synced
func (r *SQLRepository) GetUnsyncedReviews(ctx context.Context, workspaceID string, limit int) ([]string, error) {
	q := squirrel.Select("id").
		From("reviews").
		Where(squirrel.Or{
			squirrel.Eq{"synced_at": nil},
			squirrel.Expr("synced_at < updated_at"),
		})

	if workspaceID != "" {
		q = q.Where(squirrel.Eq{"workspace_id": workspaceID})
	}

	q = q.OrderBy("updated_at DESC")

	if limit > 0 {
		q = q.Limit(uint64(limit))
	}

	return r.getUnsyncedEntities(ctx, q)
}

// GetUnsyncedReviewFiles retrieves review files that need to be synced
func (r *SQLRepository) GetUnsyncedReviewFiles(ctx context.Context, reviewID string, limit int) ([]string, error) {
	q := squirrel.Select("id").
		From("review_files").
		Where(squirrel.Or{
			squirrel.Eq{"synced_at": nil},
			squirrel.Expr("synced_at < updated_at"),
		})

	if reviewID != "" {
		q = q.Where(squirrel.Eq{"review_id": reviewID})
	}

	q = q.OrderBy("updated_at DESC")

	if limit > 0 {
		q = q.Limit(uint64(limit))
	}

	return r.getUnsyncedEntities(ctx, q)
}

// GetUnsyncedIssues retrieves issues that need to be synced
func (r *SQLRepository) GetUnsyncedIssues(ctx context.Context, reviewID string, limit int) ([]string, error) {
	q := squirrel.Select("id").
		From("issues").
		Where(squirrel.Or{
			squirrel.Eq{"synced_at": nil},
			squirrel.Expr("synced_at < updated_at"),
		})

	if reviewID != "" {
		q = q.Where(squirrel.Eq{"review_id": reviewID})
	}

	q = q.OrderBy("updated_at DESC")

	if limit > 0 {
		q = q.Limit(uint64(limit))
	}

	return r.getUnsyncedEntities(ctx, q)
}

// GetUnsyncedFiles retrieves files that need to be synced
func (r *SQLRepository) GetUnsyncedFiles(ctx context.Context, workspaceID string, limit int) ([]string, error) {
	q := squirrel.Select("id").
		From("files").
		Where(squirrel.Or{
			squirrel.Eq{"synced_at": nil},
			squirrel.Expr("synced_at < updated_at"),
		})

	if workspaceID != "" {
		q = q.Where(squirrel.Eq{"workspace_id": workspaceID})
	}

	q = q.OrderBy("updated_at DESC")

	if limit > 0 {
		q = q.Limit(uint64(limit))
	}

	return r.getUnsyncedEntities(ctx, q)
}

// getUnsyncedEntities is a helper function to execute queries for unsynced entities
func (r *SQLRepository) getUnsyncedEntities(ctx context.Context, q squirrel.SelectBuilder) ([]string, error) {
	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building get unsynced entities query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("executing get unsynced entities query: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("scanning entity ID: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating entity rows: %w", err)
	}

	return ids, nil
}

// NeedsSync checks if an entity needs to be synced
func (r *SQLRepository) NeedsSync(ctx context.Context, entityType EntityType, entityID string) (bool, error) {
	var table string
	var idColumn string
	var updatedColumn string
	var syncedColumn string

	switch entityType {
	case EntityTypeWorkspace:
		table = "workspaces"
		idColumn = "id"
		updatedColumn = "updated_at"
		syncedColumn = "synced_at"
	case EntityTypeReview:
		table = "reviews"
		idColumn = "id"
		updatedColumn = "updated_at"
		syncedColumn = "synced_at"
	case EntityTypeReviewFile:
		table = "review_files"
		idColumn = "id"
		updatedColumn = "updated_at"
		syncedColumn = "synced_at"
	case EntityTypeIssue:
		table = "issues"
		idColumn = "id"
		updatedColumn = "updated_at"
		syncedColumn = "synced_at"
	case EntityTypeFile:
		table = "files"
		idColumn = "id"
		updatedColumn = "updated_at"
		syncedColumn = "synced_at"
	default:
		return false, fmt.Errorf("unsupported entity type: %s", entityType)
	}

	q := squirrel.Select("COUNT(*)").
		From(table).
		Where(squirrel.And{
			squirrel.Eq{idColumn: entityID},
			squirrel.Or{
				squirrel.Eq{syncedColumn: nil},
				squirrel.Expr(fmt.Sprintf("%s < %s", syncedColumn, updatedColumn)),
			},
		})

	var count int
	err := q.RunWith(r.db).QueryRowContext(ctx).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check sync status: %w", err)
	}

	return count > 0, nil
}
