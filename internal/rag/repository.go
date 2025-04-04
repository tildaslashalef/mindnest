package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// Repository defines the interface for vector storage and retrieval operations
type Repository interface {
	// Store a vector embedding and return its ID
	StoreVector(ctx context.Context, embedding []float32) (int64, error)

	// Retrieve a vector by its ID
	GetVector(ctx context.Context, id int64) (*Vector, error)

	// Find similar vectors based on cosine similarity
	FindSimilar(ctx context.Context, embedding []float32, limit int) ([]*SimilarVector, error)

	// Find similar vectors filtered by workspace ID
	FindSimilarByWorkspace(ctx context.Context, embedding []float32, workspaceID string, limit int) ([]*SimilarVector, error)

	// Delete a vector by its ID
	DeleteVector(ctx context.Context, id int64) error

	// Delete vectors by IDs
	DeleteVectors(ctx context.Context, ids []int64) error

	// Find chunks with their similarity scores to the given embedding in a single query
	// excludeFileID allows excluding chunks from a specific file from the results
	FindChunksWithSimilarity(
		ctx context.Context,
		embedding []float32,
		workspaceID string,
		chunkType string,
		excludeFileID string,
		limit int,
	) ([]*ChunkWithSimilarity, error)
}

// SQLRepository implements Repository using SQLite with sqlite-vec extension
type SQLRepository struct {
	db     *sql.DB
	logger *loggy.Logger
}

// NewSQLRepository creates a new vector SQL repository
func NewSQLRepository(db *sql.DB, logger *loggy.Logger) Repository {
	return &SQLRepository{
		db:     db,
		logger: logger,
	}
}

// StoreVector stores a vector embedding and returns its ID
func (r *SQLRepository) StoreVector(ctx context.Context, embedding []float32) (int64, error) {
	if len(embedding) == 0 {
		return 0, ErrInvalidEmbedding
	}

	// Use sqlite-vec.SerializeFloat32 to convert the embedding to a binary blob
	serializedEmbed, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return 0, fmt.Errorf("serializing embedding: %w", err)
	}

	// Insert into vectors table directly
	query := "INSERT INTO vectors (embedding) VALUES (?)"
	stmt, err := r.db.PrepareContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, serializedEmbed)
	if err != nil {
		return 0, fmt.Errorf("storing vector: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting vector id: %w", err)
	}

	return id, nil
}

// GetVector retrieves a vector by its ID
func (r *SQLRepository) GetVector(ctx context.Context, id int64) (*Vector, error) {
	query := "SELECT rowid, embedding FROM vectors WHERE rowid = ?"

	stmt, err := r.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	row := stmt.QueryRowContext(ctx, id)

	var vectorID int64
	var embedBlob []byte

	if err := row.Scan(&vectorID, &embedBlob); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrVectorNotFound
		}
		return nil, fmt.Errorf("scanning vector: %w", err)
	}

	embedding, err := deserializeFloat32(embedBlob)
	if err != nil {
		return nil, fmt.Errorf("deserializing embedding: %w", err)
	}

	return &Vector{
		ID:        vectorID,
		Embedding: embedding,
	}, nil
}

// calculateCosineSimilarity converts a cosine distance to a similarity score.
// The distance from sqlite-vec represents cosine distance between vectors,
// which measures dissimilarity (higher means more different).
// This function converts it to a similarity score in the range [0,1] where:
// - 1.0 means vectors are identical (distance = 0)
// - 0.0 means vectors are completely different (distance >= 1)
// The conversion uses a simple linear transformation: similarity = 1 - distance,
// clamped to ensure the result stays in [0,1] range.
func (r *SQLRepository) calculateCosineSimilarity(distance float64) float64 {
	similarity := 1.0 - distance
	if similarity < 0 {
		similarity = 0
	}
	return similarity
}

// FindSimilar finds similar vectors based on cosine similarity
func (r *SQLRepository) FindSimilar(ctx context.Context, embedding []float32, limit int) ([]*SimilarVector, error) {
	return r.FindSimilarByWorkspace(ctx, embedding, "", limit)
}

// FindSimilarByWorkspace finds similar vectors filtered by workspace ID
func (r *SQLRepository) FindSimilarByWorkspace(
	ctx context.Context,
	embedding []float32,
	workspaceID string,
	limit int,
) ([]*SimilarVector, error) {
	if len(embedding) == 0 {
		return nil, ErrInvalidEmbedding
	}

	if limit <= 0 {
		limit = 10 // Default limit if not specified
	}

	// Serialize the embedding to a binary blob using sqlite-vec
	serializedEmbed, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return nil, fmt.Errorf("serializing embedding: %w", err)
	}

	var query string
	var args []interface{}

	// Using the exact syntax from the sqlite-vec Go demo code
	if workspaceID != "" {
		// When workspace ID is provided, we JOIN with chunks to filter
		query = `
			SELECT v.rowid, distance
			FROM vectors v
			JOIN chunks c ON c.vector_id = v.rowid
			WHERE c.workspace_id = ?
			AND v.embedding MATCH ?
			AND k = ?
			ORDER BY distance
		`
		args = []interface{}{workspaceID, serializedEmbed, limit}
	} else {
		// When no workspace filter, use simple query
		query = `
			SELECT rowid, distance
			FROM vectors
			WHERE embedding MATCH ?
			AND k = ?
			ORDER BY distance
		`
		args = []interface{}{serializedEmbed, limit}
	}

	// Execute the query
	stmt, err := r.db.PrepareContext(ctx, query)
	if err != nil {
		r.logger.Warn("Vector search query failed, trying fallback to random selection", "error", err)

		// Fall back to a simple random selection
		if workspaceID != "" {
			query = `
				SELECT v.rowid, 0.5 as distance
				FROM vectors v
				JOIN chunks c ON c.vector_id = v.rowid
				WHERE c.workspace_id = ?
				GROUP BY v.rowid
				ORDER BY RANDOM()
				LIMIT ?
			`
			args = []interface{}{workspaceID, limit}
		} else {
			query = `
				SELECT rowid, 0.5 as distance
				FROM vectors
				ORDER BY RANDOM()
				LIMIT ?
			`
			args = []interface{}{limit}
		}

		stmt, err = r.db.PrepareContext(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("preparing fallback statement: %w", err)
		}
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, args...)
	if err != nil {
		// If the query fails, this could be because the extension isn't loaded correctly
		r.logger.Warn("Vector search query failed, falling back to random selection", "error", err)

		// Fall back to a simple random selection
		if workspaceID != "" {
			query = `
				SELECT 
					c.id, c.workspace_id, c.file_id, c.name, c.content,
					c.start_line, c.end_line, c.start_offset, c.end_offset,
					c.chunk_type, c.signature, c.parent_id, c.child_ids, c.metadata,
					c.vector_id, c.created_at, c.updated_at,
					0.5 as distance
				FROM chunks c
				JOIN vectors v ON c.vector_id = v.rowid
				WHERE c.workspace_id = ?
			`
			args = []interface{}{workspaceID}
		} else {
			query = `
				SELECT 
					c.id, c.workspace_id, c.file_id, c.name, c.content,
					c.start_line, c.end_line, c.start_offset, c.end_offset,
					c.chunk_type, c.signature, c.parent_id, c.child_ids, c.metadata,
					c.vector_id, c.created_at, c.updated_at,
					0.5 as distance
				FROM chunks c
				JOIN vectors v ON c.vector_id = v.rowid
			`
			args = []interface{}{}
		}

		// Add chunk type filter if provided
		if workspaceID != "" {
			query += " AND c.workspace_id = ?"
			args = append(args, workspaceID)
		}

		// Add order and limit
		query += " ORDER BY RANDOM() LIMIT ?"
		args = append(args, limit)

		stmt, err = r.db.PrepareContext(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("preparing fallback statement: %w", err)
		}
		defer stmt.Close()

		rows, err = stmt.QueryContext(ctx, args...)
		if err != nil {
			return nil, fmt.Errorf("executing fallback query: %w", err)
		}
	}
	defer rows.Close()

	var results []*SimilarVector

	for rows.Next() {
		var id int64
		var distance float64

		if err := rows.Scan(&id, &distance); err != nil {
			return nil, fmt.Errorf("scanning similar vector: %w", err)
		}

		results = append(results, &SimilarVector{
			ID:         id,
			Similarity: r.calculateCosineSimilarity(distance),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading similar vectors: %w", err)
	}

	return results, nil
}

// DeleteVector deletes a vector by its ID
func (r *SQLRepository) DeleteVector(ctx context.Context, id int64) error {
	query := "DELETE FROM vectors WHERE rowid = ?"

	stmt, err := r.db.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting vector: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking deleted rows: %w", err)
	}

	if affected == 0 {
		return ErrVectorNotFound
	}

	return nil
}

// DeleteVectors deletes vectors by their IDs
func (r *SQLRepository) DeleteVectors(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	// Using a transaction for bulk deletion
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	// Defer a rollback in case of error
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// For each ID, execute a delete statement
	for _, id := range ids {
		_, err = tx.ExecContext(ctx, "DELETE FROM vectors WHERE rowid = ?", id)
		if err != nil {
			return fmt.Errorf("deleting vector %d: %w", id, err)
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// FindChunksWithSimilarity finds chunks with their similarity scores to the given embedding
func (r *SQLRepository) FindChunksWithSimilarity(
	ctx context.Context,
	embedding []float32,
	workspaceID string,
	chunkType string,
	excludeFileID string,
	limit int,
) ([]*ChunkWithSimilarity, error) {
	if len(embedding) == 0 {
		return nil, ErrInvalidEmbedding
	}

	if limit <= 0 {
		limit = 10 // Default limit if not specified
	}

	// Serialize the embedding to a binary blob using sqlite-vec
	serializedEmbed, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return nil, fmt.Errorf("serializing embedding: %w", err)
	}

	var query string
	var args []interface{}

	// Base query with vector similarity search and chunk data
	query = `
		WITH vector_matches AS (
			SELECT v.rowid, distance
			FROM vectors v
			WHERE v.embedding MATCH ?
			AND k = ?
		)
		SELECT 
			c.id, c.workspace_id, c.file_id, c.name, c.content,
			c.start_line, c.end_line, c.start_offset, c.end_offset,
			c.chunk_type, c.signature, c.parent_id, c.child_ids, c.metadata,
			c.vector_id, c.created_at, c.updated_at,
			vm.distance
		FROM vector_matches vm
		JOIN chunks c ON c.vector_id = vm.rowid
		WHERE 1=1
	`
	args = []interface{}{serializedEmbed, limit}

	// Add workspace filter if provided
	if workspaceID != "" {
		query += " AND c.workspace_id = ?"
		args = append(args, workspaceID)
	}

	// Add chunk type filter if provided
	if chunkType != "" {
		query += " AND c.chunk_type = ?"
		args = append(args, chunkType)
	}

	// Add file exclusion if provided
	if excludeFileID != "" {
		query += " AND c.file_id != ?"
		args = append(args, excludeFileID)
	}

	query += " ORDER BY vm.distance"

	// Execute the query
	stmt, err := r.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()

	var results []*ChunkWithSimilarity

	for rows.Next() {
		var chunk workspace.Chunk
		var distance float64
		var childIDsJSON, metadataJSON []byte
		var startLine, endLine, startOffset, endOffset int

		err := rows.Scan(
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
			&metadataJSON,
			&chunk.VectorID,
			&chunk.CreatedAt,
			&chunk.UpdatedAt,
			&distance,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning chunk: %w", err)
		}

		// Set the position values
		chunk.StartPos.Line = startLine
		chunk.EndPos.Line = endLine
		chunk.StartPos.Offset = startOffset
		chunk.EndPos.Offset = endOffset

		// Parse child IDs JSON
		if len(childIDsJSON) > 0 {
			if err := json.Unmarshal(childIDsJSON, &chunk.ChildIDs); err != nil {
				return nil, fmt.Errorf("parsing child IDs: %w", err)
			}
		}

		// Parse metadata JSON
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &chunk.Metadata); err != nil {
				return nil, fmt.Errorf("parsing metadata: %w", err)
			}
		}

		// Convert distance to similarity score
		similarity := 1.0 - distance
		if similarity < 0 {
			similarity = 0
		}

		results = append(results, &ChunkWithSimilarity{
			Chunk:      &chunk,
			Similarity: similarity,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading chunks: %w", err)
	}

	return results, nil
}

// deserializeFloat32 converts a binary blob back to a float32 slice
func deserializeFloat32(blob []byte) ([]float32, error) {
	if len(blob) < 4 { // Need at least 4 bytes for a float32
		return nil, fmt.Errorf("blob too small to contain float32 data")
	}

	// Each float32 is 4 bytes
	count := len(blob) / 4
	result := make([]float32, count)

	// Convert each 4-byte chunk to a float32
	for i := 0; i < count; i++ {
		// Get 4 bytes for this float32
		start := i * 4
		bits := uint32(blob[start]) | uint32(blob[start+1])<<8 | uint32(blob[start+2])<<16 | uint32(blob[start+3])<<24
		result[i] = math.Float32frombits(bits)
	}

	return result, nil
}
