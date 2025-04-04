// Package workspace provides workspace management for the Mindnest application
package workspace

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
)

// --------------------------------------
// File Operations
// --------------------------------------

// SaveFile saves a file to the database
func (r *SQLRepository) SaveFile(ctx context.Context, file *File) error {
	query := sq.Insert("files").
		Columns(
			"id",
			"workspace_id",
			"path",
			"language",
			"last_parsed",
			"metadata",
			"created_at",
			"updated_at",
		).
		Values(
			file.ID,
			file.WorkspaceID,
			file.Path,
			file.Language,
			file.LastParsed,
			file.Metadata,
			file.CreatedAt,
			file.UpdatedAt,
		).
		Suffix("ON CONFLICT(workspace_id, path) DO UPDATE SET language = ?, last_parsed = ?, metadata = ?, updated_at = ?",
			file.Language,
			file.LastParsed,
			file.Metadata,
			file.UpdatedAt,
		)

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("generating SQL: %w", err)
	}

	_, err = r.db.ExecContext(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("executing query: %w", err)
	}

	return nil
}

// GetFileByID retrieves a file by its ID
func (r *SQLRepository) GetFileByID(ctx context.Context, fileID string) (*File, error) {
	query := sq.Select(
		"id",
		"workspace_id",
		"path",
		"language",
		"last_parsed",
		"metadata",
		"created_at",
		"updated_at",
	).
		From("files").
		Where(sq.Eq{"id": fileID})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("generating SQL: %w", err)
	}

	row := r.db.QueryRowContext(ctx, sql, args...)
	file, err := r.scanFile(row)
	if err != nil {
		// Treat not found errors as nil result
		return nil, fmt.Errorf("scanning file: %w", err)
	}

	return file, nil
}

// GetFileByPath retrieves a file by its path within a workspace
func (r *SQLRepository) GetFileByPath(ctx context.Context, workspaceID, filePath string) (*File, error) {
	query := sq.Select(
		"id",
		"workspace_id",
		"path",
		"language",
		"last_parsed",
		"metadata",
		"created_at",
		"updated_at",
	).
		From("files").
		Where(sq.Eq{"workspace_id": workspaceID, "path": filePath})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("generating SQL: %w", err)
	}

	row := r.db.QueryRowContext(ctx, sql, args...)
	file, err := r.scanFile(row)
	if err != nil {
		// Treat not found errors as nil result
		return nil, fmt.Errorf("scanning file: %w", err)
	}

	return file, nil
}

// UpdateFile updates a file in the database
func (r *SQLRepository) UpdateFile(ctx context.Context, file *File) error {
	query := sq.Update("files").
		Set("path", file.Path).
		Set("language", file.Language).
		Set("last_parsed", file.LastParsed).
		Set("metadata", file.Metadata).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": file.ID})

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("generating SQL: %w", err)
	}

	_, err = r.db.ExecContext(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("executing query: %w", err)
	}

	return nil
}

// DeleteFile deletes a file and its chunks
func (r *SQLRepository) DeleteFile(ctx context.Context, fileID string) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete chunks first (due to foreign key constraint)
	deleteChunksQuery := sq.Delete("chunks").Where(sq.Eq{"file_id": fileID})
	chunksSQL, chunksArgs, err := deleteChunksQuery.ToSql()
	if err != nil {
		return fmt.Errorf("generating chunks delete SQL: %w", err)
	}

	_, err = tx.ExecContext(ctx, chunksSQL, chunksArgs...)
	if err != nil {
		return fmt.Errorf("deleting chunks: %w", err)
	}

	// Now delete the file
	deleteFileQuery := sq.Delete("files").Where(sq.Eq{"id": fileID})
	fileSQL, fileArgs, err := deleteFileQuery.ToSql()
	if err != nil {
		return fmt.Errorf("generating file delete SQL: %w", err)
	}

	_, err = tx.ExecContext(ctx, fileSQL, fileArgs...)
	if err != nil {
		return fmt.Errorf("deleting file: %w", err)
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// GetFilesByWorkspaceID retrieves all files for a workspace
func (r *SQLRepository) GetFilesByWorkspaceID(ctx context.Context, workspaceID string) ([]*File, error) {
	query := sq.Select(
		"id",
		"workspace_id",
		"path",
		"language",
		"last_parsed",
		"metadata",
		"created_at",
		"updated_at",
	).
		From("files").
		Where(sq.Eq{"workspace_id": workspaceID}).
		OrderBy("path")

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("generating SQL: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		file, err := r.scanFile(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning file: %w", err)
		}
		files = append(files, file)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return files, nil
}

// scanFile scans a row into a File struct
func (r *SQLRepository) scanFile(scanner interface {
	Scan(dest ...interface{}) error
}) (*File, error) {
	var file File
	var lastParsed sql.NullTime
	var metadata sql.NullString // Use NullString to handle NULL values

	var err error
	switch s := scanner.(type) {
	case *sql.Row:
		err = s.Scan(
			&file.ID,
			&file.WorkspaceID,
			&file.Path,
			&file.Language,
			&lastParsed,
			&metadata, // Scan into NullString
			&file.CreatedAt,
			&file.UpdatedAt,
		)
	case *sql.Rows:
		err = s.Scan(
			&file.ID,
			&file.WorkspaceID,
			&file.Path,
			&file.Language,
			&lastParsed,
			&metadata, // Scan into NullString
			&file.CreatedAt,
			&file.UpdatedAt,
		)
	default:
		return nil, fmt.Errorf("unsupported scanner type")
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	// Set LastParsed if not NULL
	if lastParsed.Valid {
		parsedTime := lastParsed.Time
		file.LastParsed = &parsedTime
	}

	// Set Metadata if not NULL
	if metadata.Valid && metadata.String != "" {
		err = json.Unmarshal([]byte(metadata.String), &file.Metadata)
		if err != nil {
			r.logger.Warn("Failed to unmarshal file metadata", "file_id", file.ID, "error", err)
			// Initialize empty metadata rather than failing
			file.Metadata = json.RawMessage("{}")
		}
	} else {
		// Initialize empty metadata
		file.Metadata = json.RawMessage("{}")
	}

	return &file, nil
}
