// Package workspace provides workspace management for the Mindnest application
package workspace

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/tildaslashalef/mindnest/internal/parser"
)

// --------------------------------------
// Chunk Operations
// --------------------------------------

// SaveChunk saves a chunk to the database
func (r *SQLRepository) SaveChunk(ctx context.Context, chunk *Chunk) error {
	// Prepare values for child_ids
	var childIDsJSON []byte
	if len(chunk.ChildIDs) > 0 {
		var err error
		childIDsJSON, err = json.Marshal(chunk.ChildIDs)
		if err != nil {
			return fmt.Errorf("marshaling child_ids: %w", err)
		}
	}

	query := sq.Insert("chunks").
		Columns(
			"id",
			"workspace_id",
			"file_id",
			"name",
			"content",
			"start_line",
			"end_line",
			"start_offset",
			"end_offset",
			"chunk_type",
			"signature",
			"parent_id",
			"child_ids",
			"metadata",
			"vector_id",
			"created_at",
			"updated_at",
		).
		Values(
			chunk.ID,
			chunk.WorkspaceID,
			chunk.FileID,
			chunk.Name,
			chunk.Content,
			chunk.StartPos.Line,
			chunk.EndPos.Line,
			chunk.StartPos.Offset,
			chunk.EndPos.Offset,
			chunk.ChunkType,
			chunk.Signature,
			chunk.ParentID,
			childIDsJSON,
			chunk.Metadata,
			chunk.VectorID,
			chunk.CreatedAt,
			chunk.UpdatedAt,
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

// GetChunkByID retrieves a chunk by its ID
func (r *SQLRepository) GetChunkByID(ctx context.Context, chunkID string) (*Chunk, error) {
	query := sq.Select(
		"id",
		"workspace_id",
		"file_id",
		"name",
		"content",
		"start_line",
		"end_line",
		"start_offset",
		"end_offset",
		"chunk_type",
		"signature",
		"parent_id",
		"child_ids",
		"metadata",
		"vector_id",
		"created_at",
		"updated_at",
	).
		From("chunks").
		Where(sq.Eq{"id": chunkID})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("generating SQL: %w", err)
	}

	row := r.db.QueryRowContext(ctx, sql, args...)
	chunk, err := r.scanChunk(row)
	if err != nil {
		return nil, fmt.Errorf("scanning chunk: %w", err)
	}

	return chunk, nil
}

// GetChunksByFileID retrieves all chunks for a file
func (r *SQLRepository) GetChunksByFileID(ctx context.Context, fileID string) ([]*Chunk, error) {
	query := sq.Select(
		"id",
		"workspace_id",
		"file_id",
		"name",
		"content",
		"start_line",
		"end_line",
		"start_offset",
		"end_offset",
		"chunk_type",
		"signature",
		"parent_id",
		"child_ids",
		"metadata",
		"vector_id",
		"created_at",
		"updated_at",
	).
		From("chunks").
		Where(sq.Eq{"file_id": fileID}).
		OrderBy("start_line")

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("generating SQL: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()

	var chunks []*Chunk
	for rows.Next() {
		chunk, err := r.scanChunk(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning chunk: %w", err)
		}
		chunks = append(chunks, chunk)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return chunks, nil
}

// GetChunksByWorkspaceID retrieves all chunks for a workspace
func (r *SQLRepository) GetChunksByWorkspaceID(ctx context.Context, workspaceID string) ([]*Chunk, error) {
	query := sq.Select(
		"id",
		"workspace_id",
		"file_id",
		"name",
		"content",
		"start_line",
		"end_line",
		"start_offset",
		"end_offset",
		"chunk_type",
		"signature",
		"parent_id",
		"child_ids",
		"metadata",
		"vector_id",
		"created_at",
		"updated_at",
	).
		From("chunks").
		Where(sq.Eq{"workspace_id": workspaceID})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("generating SQL: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()

	var chunks []*Chunk
	for rows.Next() {
		chunk, err := r.scanChunk(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning chunk: %w", err)
		}
		chunks = append(chunks, chunk)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return chunks, nil
}

// GetChunksByType retrieves all chunks of a specific type for a workspace
func (r *SQLRepository) GetChunksByType(ctx context.Context, workspaceID string, chunkType ChunkType) ([]*Chunk, error) {
	query := sq.Select(
		"id",
		"workspace_id",
		"file_id",
		"name",
		"content",
		"start_line",
		"end_line",
		"start_offset",
		"end_offset",
		"chunk_type",
		"signature",
		"parent_id",
		"child_ids",
		"metadata",
		"vector_id",
		"created_at",
		"updated_at",
	).
		From("chunks").
		Where(sq.Eq{"workspace_id": workspaceID, "chunk_type": chunkType})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("generating SQL: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()

	var chunks []*Chunk
	for rows.Next() {
		chunk, err := r.scanChunk(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning chunk: %w", err)
		}
		chunks = append(chunks, chunk)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return chunks, nil
}

// UpdateChunk updates a chunk in the database
func (r *SQLRepository) UpdateChunk(ctx context.Context, chunk *Chunk) error {
	// Prepare values for child_ids
	var childIDsJSON []byte
	if len(chunk.ChildIDs) > 0 {
		var err error
		childIDsJSON, err = json.Marshal(chunk.ChildIDs)
		if err != nil {
			return fmt.Errorf("marshaling child_ids: %w", err)
		}
	}

	query := sq.Update("chunks").
		Set("name", chunk.Name).
		Set("content", chunk.Content).
		Set("start_line", chunk.StartPos.Line).
		Set("end_line", chunk.EndPos.Line).
		Set("start_offset", chunk.StartPos.Offset).
		Set("end_offset", chunk.EndPos.Offset).
		Set("chunk_type", chunk.ChunkType).
		Set("signature", chunk.Signature).
		Set("parent_id", chunk.ParentID).
		Set("child_ids", childIDsJSON).
		Set("metadata", chunk.Metadata).
		Set("vector_id", chunk.VectorID).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": chunk.ID})

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

// DeleteChunk deletes a chunk
func (r *SQLRepository) DeleteChunk(ctx context.Context, chunkID string) error {
	query := sq.Delete("chunks").
		Where(sq.Eq{"id": chunkID})

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

// DeleteChunksByFileID deletes all chunks for a file
func (r *SQLRepository) DeleteChunksByFileID(ctx context.Context, fileID string) error {
	query := sq.Delete("chunks").
		Where(sq.Eq{"file_id": fileID})

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

// SaveChunksForFile saves multiple chunks for a file to the database
func (r *SQLRepository) SaveChunksForFile(ctx context.Context, file *File, chunks []*Chunk) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing chunks for this file
	deleteQuery := sq.Delete("chunks").Where(sq.Eq{"file_id": file.ID})
	deleteSQL, deleteArgs, err := deleteQuery.ToSql()
	if err != nil {
		return fmt.Errorf("generating delete SQL: %w", err)
	}

	_, err = tx.ExecContext(ctx, deleteSQL, deleteArgs...)
	if err != nil {
		return fmt.Errorf("deleting existing chunks: %w", err)
	}

	// Insert each chunk
	for _, chunk := range chunks {
		// Ensure workspace ID and file ID are set
		chunk.WorkspaceID = file.WorkspaceID
		chunk.FileID = file.ID

		// Prepare values for child_ids
		var childIDsJSON []byte
		if len(chunk.ChildIDs) > 0 {
			childIDsJSON, err = json.Marshal(chunk.ChildIDs)
			if err != nil {
				return fmt.Errorf("marshaling child_ids: %w", err)
			}
		}

		query := sq.Insert("chunks").
			Columns(
				"id",
				"workspace_id",
				"file_id",
				"name",
				"content",
				"start_line",
				"end_line",
				"start_offset",
				"end_offset",
				"chunk_type",
				"signature",
				"parent_id",
				"child_ids",
				"metadata",
				"vector_id",
				"created_at",
				"updated_at",
			).
			Values(
				chunk.ID,
				chunk.WorkspaceID,
				chunk.FileID,
				chunk.Name,
				chunk.Content,
				chunk.StartPos.Line,
				chunk.EndPos.Line,
				chunk.StartPos.Offset,
				chunk.EndPos.Offset,
				chunk.ChunkType,
				chunk.Signature,
				chunk.ParentID,
				childIDsJSON,
				chunk.Metadata,
				chunk.VectorID,
				chunk.CreatedAt,
				chunk.UpdatedAt,
			)

		sql, args, err := query.ToSql()
		if err != nil {
			return fmt.Errorf("generating SQL: %w", err)
		}

		_, err = tx.ExecContext(ctx, sql, args...)
		if err != nil {
			return fmt.Errorf("executing query: %w", err)
		}
	}

	// Update the file's last_parsed time
	file.UpdateLastParsed()
	updateQuery := sq.Update("files").
		Set("last_parsed", file.LastParsed).
		Set("updated_at", file.UpdatedAt).
		Where(sq.Eq{"id": file.ID})

	updateSQL, updateArgs, err := updateQuery.ToSql()
	if err != nil {
		return fmt.Errorf("generating update SQL: %w", err)
	}

	_, err = tx.ExecContext(ctx, updateSQL, updateArgs...)
	if err != nil {
		return fmt.Errorf("updating file: %w", err)
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// scanChunk scans a row into a Chunk struct
func (r *SQLRepository) scanChunk(scanner interface {
	Scan(dest ...interface{}) error
}) (*Chunk, error) {
	var chunk Chunk
	var startLine, endLine, startOffset, endOffset int
	var childIDsJSON []byte
	var vectorID sql.NullInt64
	var metadata sql.NullString // Use NullString to handle NULL metadata values

	var err error
	switch s := scanner.(type) {
	case *sql.Row:
		err = s.Scan(
			&chunk.ID,
			&chunk.WorkspaceID,
			&chunk.FileID,
			&chunk.Name,
			&chunk.Content,
			&startLine,
			&endLine,
			&startOffset,
			&endOffset,
			&chunk.ChunkType,
			&chunk.Signature,
			&chunk.ParentID,
			&childIDsJSON,
			&metadata, // Use NullString for metadata
			&vectorID,
			&chunk.CreatedAt,
			&chunk.UpdatedAt,
		)
	case *sql.Rows:
		err = s.Scan(
			&chunk.ID,
			&chunk.WorkspaceID,
			&chunk.FileID,
			&chunk.Name,
			&chunk.Content,
			&startLine,
			&endLine,
			&startOffset,
			&endOffset,
			&chunk.ChunkType,
			&chunk.Signature,
			&chunk.ParentID,
			&childIDsJSON,
			&metadata, // Use NullString for metadata
			&vectorID,
			&chunk.CreatedAt,
			&chunk.UpdatedAt,
		)
	default:
		return nil, fmt.Errorf("unsupported scanner type")
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows // Return standard error for better compatibility
		}
		return nil, err
	}

	// Set position information
	chunk.StartPos = parser.Position{
		Filename: chunk.FileID,
		Line:     startLine,
		Column:   0,
		Offset:   startOffset,
	}

	chunk.EndPos = parser.Position{
		Filename: chunk.FileID,
		Line:     endLine,
		Column:   0,
		Offset:   endOffset,
	}

	// Parse child IDs
	if len(childIDsJSON) > 0 {
		err = json.Unmarshal(childIDsJSON, &chunk.ChildIDs)
		if err != nil {
			r.logger.Warn("Failed to unmarshal child_ids", "error", err)
			chunk.ChildIDs = []string{}
		}
	} else {
		chunk.ChildIDs = []string{}
	}

	// Set Metadata if not NULL
	if metadata.Valid && metadata.String != "" {
		err = json.Unmarshal([]byte(metadata.String), &chunk.Metadata)
		if err != nil {
			r.logger.Warn("Failed to unmarshal chunk metadata", "chunk_id", chunk.ID, "error", err)
			// Initialize empty metadata rather than failing
			chunk.Metadata = json.RawMessage("{}")
		}
	} else {
		// Initialize empty metadata
		chunk.Metadata = json.RawMessage("{}")
	}

	// Set vector ID if valid
	if vectorID.Valid {
		chunk.VectorID = vectorID
	}

	return &chunk, nil
}
