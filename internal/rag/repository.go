package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// Repository defines the interface for vector operations in the RAG system
type Repository interface {
	// Basic vector operations
	StoreVector(ctx context.Context, embedding []float32) (int64, error)
	GetVector(ctx context.Context, id int64) (*Vector, error)
	DeleteVector(ctx context.Context, id int64) error
	DeleteVectors(ctx context.Context, ids []int64) error

	// Similarity search
	FindSimilar(ctx context.Context, embedding []float32, limit int) ([]*ScoredVector, error)
	FindSimilarByWorkspace(ctx context.Context, embedding []float32, workspaceID string, limit int) ([]*ScoredVector, error)
	FindChunksWithSimilarity(ctx context.Context, embedding []float32, opts *SearchOptions) ([]*ScoredChunk, error)
	FindSimilarInStore(ctx context.Context, embedding []float32, opts *SearchOptions) ([]*ScoredChunk, error)

	// Advanced vector operations
	StoreVectorWithOptions(ctx context.Context, embedding []float32, normalize bool, compressionType VectorType) (int64, error)
	StoreVectorWithMetadata(ctx context.Context, embedding []float32, chunkID string, workspaceID string, metadata map[string]interface{}) (int64, error)
	FindSimilarWithMetric(ctx context.Context, embedding []float32, workspaceID string, metric DistanceMetric, limit int) ([]*ScoredVector, error)
	StoreVectorWithDimensions(ctx context.Context, embedding []float32, dimensions int) (int64, error)
}

// SQLRepository implements Repository using SQLite with the sqlite-vec extension
type SQLRepository struct {
	db     *sql.DB
	logger *loggy.Logger
	vecOps *VecOps
}

// NewRepository creates a new vector repository
func NewRepository(db *sql.DB, logger *loggy.Logger) Repository {
	return &SQLRepository{
		db:     db,
		logger: logger,
		vecOps: NewVecOps(db, logger),
	}
}

// StoreVector stores a vector embedding and returns its ID
func (r *SQLRepository) StoreVector(ctx context.Context, embedding []float32) (int64, error) {
	if len(embedding) == 0 {
		return 0, ErrInvalidVector
	}

	// Serialize the vector to binary format for storage
	serialized, err := r.vecOps.SerializeVector(embedding, VectorTypeFloat32)
	if err != nil {
		return 0, fmt.Errorf("serializing vector: %w", err)
	}

	// Insert into vectors table
	query := "INSERT INTO vectors (embedding) VALUES (?)"
	res, err := r.db.ExecContext(ctx, query, serialized)
	if err != nil {
		return 0, fmt.Errorf("storing vector: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting vector ID: %w", err)
	}

	return id, nil
}

// GetVector retrieves a vector by its ID
func (r *SQLRepository) GetVector(ctx context.Context, id int64) (*Vector, error) {
	query := "SELECT rowid, embedding FROM vectors WHERE rowid = ?"
	row := r.db.QueryRowContext(ctx, query, id)

	var vectorID int64
	var blob []byte
	if err := row.Scan(&vectorID, &blob); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrVectorNotFound
		}
		return nil, fmt.Errorf("scanning vector: %w", err)
	}

	// Determine vector type (float32 is default for compatibility)
	vType := VectorTypeFloat32
	if detectedType, err := r.vecOps.GetVectorType(ctx, blob); err == nil {
		vType = detectedType
	}

	// Deserialize the binary blob back to a float32 vector
	embedding, err := r.vecOps.DeserializeVector(blob, vType)
	if err != nil {
		return nil, fmt.Errorf("deserializing vector: %w", err)
	}

	return &Vector{
		ID:        vectorID,
		Embedding: embedding,
		Type:      vType,
	}, nil
}

// DeleteVector deletes a vector by its ID
func (r *SQLRepository) DeleteVector(ctx context.Context, id int64) error {
	query := "DELETE FROM vectors WHERE rowid = ?"
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting vector: %w", err)
	}
	return nil
}

// DeleteVectors deletes multiple vectors by their IDs
func (r *SQLRepository) DeleteVectors(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	// Use a transaction for better performance with multiple deletes
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	query := "DELETE FROM vectors WHERE rowid = ?"
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	for _, id := range ids {
		if _, err = stmt.ExecContext(ctx, id); err != nil {
			return fmt.Errorf("deleting vector %d: %w", id, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// FindSimilar finds vectors similar to the query vector
func (r *SQLRepository) FindSimilar(ctx context.Context, embedding []float32, limit int) ([]*ScoredVector, error) {
	return r.FindSimilarByWorkspace(ctx, embedding, "", limit)
}

// FindSimilarByWorkspace finds vectors similar to the query vector, filtered by workspace
func (r *SQLRepository) FindSimilarByWorkspace(
	ctx context.Context,
	embedding []float32,
	workspaceID string,
	limit int,
) ([]*ScoredVector, error) {
	if len(embedding) == 0 {
		return nil, ErrInvalidVector
	}

	if limit <= 0 {
		limit = 10 // Default limit
	}

	// Use optimized KNN search
	var additionalWhere string
	var args []interface{}

	if workspaceID != "" {
		additionalWhere = "EXISTS (SELECT 1 FROM chunks c WHERE c.vector_id = vectors.rowid AND c.workspace_id = ?)"
		args = []interface{}{workspaceID}
	}

	// Find the k-nearest neighbors
	vectorIDs, distances, err := r.vecOps.FindKNN(ctx, embedding, limit, additionalWhere, args...)
	if err != nil {
		return nil, fmt.Errorf("KNN search: %w", err)
	}

	// Convert to ScoredVector results
	scoredVectors := make([]*ScoredVector, len(vectorIDs))
	for i, id := range vectorIDs {
		similarity := r.vecOps.DistanceToSimilarity(distances[i], DistanceMetricCosine)
		scoredVectors[i] = &ScoredVector{
			ID:         id,
			Similarity: similarity,
			Distance:   distances[i],
		}
	}

	return scoredVectors, nil
}

// FindChunksWithSimilarity finds chunks with similarity scores to the query vector
func (r *SQLRepository) FindChunksWithSimilarity(
	ctx context.Context,
	embedding []float32,
	opts *SearchOptions,
) ([]*ScoredChunk, error) {
	if len(embedding) == 0 {
		return nil, ErrInvalidVector
	}

	if opts == nil {
		opts = NewSearchOptions()
	}

	// We need to use the newer approach with vector_store, as the old approach depended on VectorID
	// which is no longer available in the Chunk struct
	serialized, err := r.vecOps.SerializeVector(embedding, VectorTypeFloat32)
	if err != nil {
		return nil, fmt.Errorf("serializing embedding: %w", err)
	}

	// Build query to join chunks and vector_store
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		WITH vector_matches AS (
			SELECT v.rowid, distance
			FROM vectors v
			WHERE v.embedding MATCH ?
			AND k = ?
		)
		SELECT 
			c.id, c.workspace_id, c.file_id, c.content, c.name,
			c.start_line, c.end_line, c.chunk_type, c.signature,
			c.parent_id, vs.id as vector_store_id, vs.metadata,
			vm.distance
		FROM vector_matches vm
		JOIN vector_store vs ON vs.id = ('vec_' || vm.rowid)
		JOIN chunks c ON c.id = vs.chunk_id
		WHERE 1=1
	`)

	// Prepare query arguments
	queryArgs := []interface{}{serialized, opts.Limit}

	// Add workspace filter if provided
	if opts.WorkspaceID != "" || (opts.Filter != nil && opts.Filter.WorkspaceID != "") {
		workspaceID := opts.WorkspaceID
		if workspaceID == "" && opts.Filter != nil {
			workspaceID = opts.Filter.WorkspaceID
		}
		queryBuilder.WriteString(" AND c.workspace_id = ?")
		queryArgs = append(queryArgs, workspaceID)
	}

	// Add chunk type filter if provided
	if opts.ChunkType != "" {
		queryBuilder.WriteString(" AND c.chunk_type = ?")
		queryArgs = append(queryArgs, string(opts.ChunkType))
	}

	// Add file filter if provided
	if opts.Filter != nil && opts.Filter.FileID != "" {
		queryBuilder.WriteString(" AND c.file_id = ?")
		queryArgs = append(queryArgs, opts.Filter.FileID)
	}

	// Add file exclusion if provided
	if opts.ExcludeFileID != "" {
		queryBuilder.WriteString(" AND c.file_id != ?")
		queryArgs = append(queryArgs, opts.ExcludeFileID)
	}

	// Add multiple file filter if provided
	if opts.Filter != nil && len(opts.Filter.FileIDs) > 0 {
		placeholders := make([]string, len(opts.Filter.FileIDs))
		for i := range placeholders {
			placeholders[i] = "?"
			queryArgs = append(queryArgs, opts.Filter.FileIDs[i])
		}
		queryBuilder.WriteString(" AND c.file_id IN (" + strings.Join(placeholders, ",") + ")")
	}

	// Add chunk ID exclusion if provided
	if opts.Filter != nil && len(opts.Filter.ExcludeIDs) > 0 {
		placeholders := make([]string, len(opts.Filter.ExcludeIDs))
		for i := range placeholders {
			placeholders[i] = "?"
			queryArgs = append(queryArgs, opts.Filter.ExcludeIDs[i])
		}
		queryBuilder.WriteString(" AND c.id NOT IN (" + strings.Join(placeholders, ",") + ")")
	}

	// Add similarity threshold if provided
	if opts.MinSimilarity > 0 {
		// Convert similarity threshold to distance threshold
		// For cosine: distance = 2 * (1 - similarity)
		var distanceThreshold float64
		switch opts.Metric {
		case DistanceMetricCosine:
			distanceThreshold = 2.0 * (1.0 - opts.MinSimilarity)
		case DistanceMetricL2:
			// For L2: approx. distance threshold is (1/similarity) - 1
			distanceThreshold = (1.0 / opts.MinSimilarity) - 1.0
		default:
			// Default threshold (conservative)
			distanceThreshold = 1.0 - opts.MinSimilarity
		}
		queryBuilder.WriteString(" AND vm.distance <= ?")
		queryArgs = append(queryArgs, distanceThreshold)
	}

	// Add order by and limit
	queryBuilder.WriteString(" ORDER BY vm.distance LIMIT ?")
	queryArgs = append(queryArgs, opts.Limit)

	// Execute the query
	rows, err := r.db.QueryContext(ctx, queryBuilder.String(), queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()

	// Process results
	var results []*ScoredChunk
	for rows.Next() {
		var chunk workspace.Chunk
		var distance float64
		var metadataJSON []byte
		var startLine, endLine int
		var vectorStoreID string

		err := rows.Scan(
			&chunk.ID, &chunk.WorkspaceID, &chunk.FileID, &chunk.Content, &chunk.Name,
			&startLine, &endLine, &chunk.ChunkType, &chunk.Signature,
			&chunk.ParentID, &vectorStoreID, &metadataJSON,
			&distance,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning chunk: %w", err)
		}

		// Set position values
		chunk.StartPos.Line = startLine
		chunk.EndPos.Line = endLine

		// Parse metadata if present
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &chunk.Metadata); err != nil {
				return nil, fmt.Errorf("parsing metadata: %w", err)
			}
		}

		// Convert distance to similarity
		similarity := r.vecOps.DistanceToSimilarity(distance, opts.Metric)

		// Create result with or without distance based on options
		result := &ScoredChunk{
			Chunk:      &chunk,
			Similarity: similarity,
		}

		if opts.IncludeDistance {
			result.Distance = distance
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating results: %w", err)
	}

	return results, nil
}

// StoreVectorWithOptions stores a vector with advanced options like normalization and compression
func (r *SQLRepository) StoreVectorWithOptions(
	ctx context.Context,
	embedding []float32,
	normalize bool,
	compressionType VectorType,
) (int64, error) {
	if len(embedding) == 0 {
		return 0, ErrInvalidVector
	}

	// Apply normalization if requested
	if normalize {
		normalized, err := r.vecOps.NormalizeVector(ctx, embedding)
		if err != nil {
			return 0, fmt.Errorf("normalizing vector: %w", err)
		}
		embedding = normalized
	}

	// Apply compression if requested
	var serialized []byte
	var err error

	switch compressionType {
	case VectorTypeFloat32:
		// Standard storage
		serialized, err = r.vecOps.SerializeVector(embedding, VectorTypeFloat32)
	case VectorTypeInt8:
		// Int8 quantization (4x smaller)
		serialized, err = r.vecOps.QuantizeInt8(ctx, embedding, -1.0, 1.0)
	case VectorTypeBit, VectorTypeBinary:
		// Binary quantization (32x smaller)
		serialized, err = r.vecOps.QuantizeBinary(ctx, embedding)
	default:
		return 0, fmt.Errorf("unsupported vector type: %s", compressionType)
	}

	if err != nil {
		return 0, fmt.Errorf("preparing vector: %w", err)
	}

	// Store the vector
	query := "INSERT INTO vectors (embedding) VALUES (?)"
	res, err := r.db.ExecContext(ctx, query, serialized)
	if err != nil {
		return 0, fmt.Errorf("storing vector: %w", err)
	}

	return res.LastInsertId()
}

// StoreVectorWithMetadata stores a vector with associated metadata
func (r *SQLRepository) StoreVectorWithMetadata(
	ctx context.Context,
	embedding []float32,
	chunkID string,
	workspaceID string,
	metadata map[string]interface{},
) (int64, error) {
	if len(embedding) == 0 {
		return 0, fmt.Errorf("empty embedding: %w", ErrInvalidVector)
	}

	// Normalize for better search results
	normalized, err := r.vecOps.NormalizeVector(ctx, embedding)
	if err != nil {
		r.logger.Warn("Failed to normalize vector", "error", err)
		normalized = embedding // Fall back to original
	}

	// Serialize the vector
	serialized, err := r.vecOps.SerializeVector(normalized, VectorTypeFloat32)
	if err != nil {
		return 0, fmt.Errorf("serializing vector: %w", err)
	}

	// Convert metadata to JSON if present
	var metadataJSON []byte
	if len(metadata) > 0 {
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			return 0, fmt.Errorf("serializing metadata: %w", err)
		}
	}

	// Calculate dimensions
	dimensions := len(embedding)

	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Insert into vector_store table with metadata
	query := `
		INSERT INTO vector_store 
		(id, chunk_id, workspace_id, vector, vector_type, dimensions, metadata) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(chunk_id) DO UPDATE SET
		vector = excluded.vector,
		vector_type = excluded.vector_type,
		dimensions = excluded.dimensions,
		metadata = excluded.metadata
	`

	// Generate unique ID for the vector
	vectorID := fmt.Sprintf("vec_%s", chunkID)

	_, err = tx.ExecContext(
		ctx,
		query,
		vectorID,
		chunkID,
		workspaceID,
		serialized,
		string(VectorTypeFloat32),
		dimensions,
		metadataJSON,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting vector with metadata: %w", err)
	}

	// Also insert into virtual table for search
	// First delete any existing entry to avoid duplicates
	_, err = tx.ExecContext(
		ctx,
		"DELETE FROM vector_index WHERE id = ?",
		vectorID,
	)
	if err != nil {
		r.logger.Warn("Failed to delete from vector_index", "error", err)
		// Continue anyway, as this might just be the first insertion
	}

	// Now insert
	_, err = tx.ExecContext(
		ctx,
		"INSERT INTO vector_index (id, embedding, workspace_id) VALUES (?, ?, ?)",
		vectorID,
		serialized,
		workspaceID,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting into vector index: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}

	// Return the ID as int64 for compatibility
	return 0, nil // Return 0 since the actual ID is a string
}

// FindSimilarWithMetric finds similar vectors using a specific distance metric
func (r *SQLRepository) FindSimilarWithMetric(
	ctx context.Context,
	embedding []float32,
	workspaceID string,
	metric DistanceMetric,
	limit int,
) ([]*ScoredVector, error) {
	if len(embedding) == 0 {
		return nil, ErrInvalidVector
	}

	if limit <= 0 {
		limit = 10 // Default limit
	}

	// Prepare WHERE clause based on workspace filter
	var additionalWhere string
	var args []interface{}

	if workspaceID != "" {
		additionalWhere = "EXISTS (SELECT 1 FROM chunks c WHERE c.vector_id = vectors.rowid AND c.workspace_id = ?)"
		args = []interface{}{workspaceID}
	}

	// Find KNN vectors
	vectorIDs, distances, err := r.vecOps.FindKNN(ctx, embedding, limit, additionalWhere, args...)
	if err != nil {
		return nil, fmt.Errorf("KNN search: %w", err)
	}

	// Convert to ScoredVector results
	results := make([]*ScoredVector, len(vectorIDs))
	for i, id := range vectorIDs {
		// Convert distance to similarity score using the specified metric
		similarity := r.vecOps.DistanceToSimilarity(distances[i], metric)
		results[i] = &ScoredVector{
			ID:         id,
			Similarity: similarity,
			Distance:   distances[i],
		}
	}

	return results, nil
}

// StoreVectorWithDimensions stores a vector with variable dimensions
// This is useful for adaptive embedding sizes based on content complexity
func (r *SQLRepository) StoreVectorWithDimensions(
	ctx context.Context,
	embedding []float32,
	dimensions int,
) (int64, error) {
	if len(embedding) == 0 {
		return 0, ErrInvalidVector
	}

	if dimensions <= 0 || dimensions > len(embedding) {
		dimensions = len(embedding)
	}

	// Slice the vector to the requested dimensions
	sliced, err := r.vecOps.SliceVector(ctx, embedding, 0, dimensions)
	if err != nil {
		return 0, fmt.Errorf("slicing vector: %w", err)
	}

	// Normalize for better similarity
	normalized, err := r.vecOps.NormalizeVector(ctx, sliced)
	if err != nil {
		return 0, fmt.Errorf("normalizing vector: %w", err)
	}

	// Store the vector
	return r.StoreVector(ctx, normalized)
}

// FindSimilarInStore finds vectors in the new vector_store that are similar to the query vector
func (r *SQLRepository) FindSimilarInStore(
	ctx context.Context,
	embedding []float32,
	opts *SearchOptions,
) ([]*ScoredChunk, error) {
	if len(embedding) == 0 {
		return nil, ErrInvalidVector
	}

	if opts == nil {
		opts = NewSearchOptions()
	}

	// Ensure we have a valid limit
	if opts.Limit <= 0 {
		opts.Limit = 10 // Default limit if not set
	}

	// Normalize the query vector if requested
	if opts.Normalization {
		normalized, err := r.vecOps.NormalizeVector(ctx, embedding)
		if err != nil {
			r.logger.Warn("Failed to normalize query vector", "error", err)
		} else {
			embedding = normalized
		}
	}

	// Serialize the query vector
	serialized, err := r.vecOps.SerializeVector(embedding, VectorTypeFloat32)
	if err != nil {
		return nil, fmt.Errorf("serializing query vector: %w", err)
	}

	// Build query and arguments
	query, args := r.buildVectorStoreQuery(serialized, opts)

	// Execute the query
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("executing vector store query: %w", err)
	}
	defer rows.Close()

	// Process results
	return r.processVectorStoreResults(rows, opts)
}

// prepareVectorWithDistanceType prepares a vector for search with the specified distance type
// This converts the vector to the format needed by sqlite-vec's MATCH operator
func (r *SQLRepository) prepareVectorWithDistanceType(vector []byte, distanceType string) ([]byte, error) {
	// Based on the documentation, we need to provide just the vector
	// and specify the distance type in the virtual table definition or query
	// For now, let's just return the vector as is since we can't modify the query structure easily
	return vector, nil
}

// buildVectorStoreQuery constructs the SQL query and arguments for vector store search
func (r *SQLRepository) buildVectorStoreQuery(serializedVector []byte, opts *SearchOptions) (string, []interface{}) {
	// Build the base query
	queryBuilder := strings.Builder{}

	// We're using the distance_metric defined in the table creation,
	// so we don't need to specify it in the query
	queryBuilder.WriteString(`
		WITH vector_matches AS (
			SELECT vi.id, vi.workspace_id, distance
			FROM vector_index vi
			WHERE vi.embedding MATCH ? 
			AND k = ?
		)
		SELECT 
			vs.id, vs.chunk_id, vs.workspace_id, 
			vs.vector_type, vs.dimensions, vs.metadata,
			c.content, c.name, c.start_line, c.end_line, 
			c.chunk_type, c.signature,
			vm.distance
		FROM vector_matches vm
		JOIN vector_store vs ON vm.id = vs.id
		JOIN chunks c ON vs.chunk_id = c.id
		WHERE 1=1
	`)

	// Prepare query arguments
	queryArgs := []interface{}{serializedVector, opts.Limit}

	// Add workspace filter if provided
	if opts.WorkspaceID != "" || (opts.Filter != nil && opts.Filter.WorkspaceID != "") {
		workspaceID := opts.WorkspaceID
		if workspaceID == "" && opts.Filter != nil {
			workspaceID = opts.Filter.WorkspaceID
		}
		queryBuilder.WriteString(" AND vs.workspace_id = ?")
		queryArgs = append(queryArgs, workspaceID)
	}

	// Add chunk type filter if provided
	if opts.ChunkType != "" {
		queryBuilder.WriteString(" AND c.chunk_type = ?")
		queryArgs = append(queryArgs, string(opts.ChunkType))
	}

	// Add file filter if provided
	if opts.Filter != nil && opts.Filter.FileID != "" {
		queryBuilder.WriteString(" AND c.file_id = ?")
		queryArgs = append(queryArgs, opts.Filter.FileID)
	}

	// Add file exclusion if provided
	if opts.ExcludeFileID != "" {
		queryBuilder.WriteString(" AND c.file_id != ?")
		queryArgs = append(queryArgs, opts.ExcludeFileID)
	}

	queryArgs = r.addMultipleFileFilters(&queryBuilder, opts, queryArgs)
	queryArgs = r.addChunkExclusionFilters(&queryBuilder, opts, queryArgs)
	queryArgs = r.addSimilarityThreshold(&queryBuilder, opts, queryArgs)

	// Order by similarity and add limit
	queryBuilder.WriteString(" ORDER BY vm.distance LIMIT ?")
	queryArgs = append(queryArgs, opts.Limit)

	return queryBuilder.String(), queryArgs
}

// addMultipleFileFilters adds file ID filters to the query
func (r *SQLRepository) addMultipleFileFilters(queryBuilder *strings.Builder, opts *SearchOptions, queryArgs []interface{}) []interface{} {
	if opts.Filter != nil && len(opts.Filter.FileIDs) > 0 {
		placeholders := make([]string, len(opts.Filter.FileIDs))
		for i := range placeholders {
			placeholders[i] = "?"
			queryArgs = append(queryArgs, opts.Filter.FileIDs[i])
		}
		queryBuilder.WriteString(" AND c.file_id IN (" + strings.Join(placeholders, ",") + ")")
	}
	return queryArgs
}

// addChunkExclusionFilters adds chunk exclusion filters to the query
func (r *SQLRepository) addChunkExclusionFilters(queryBuilder *strings.Builder, opts *SearchOptions, queryArgs []interface{}) []interface{} {
	if opts.Filter != nil && len(opts.Filter.ExcludeIDs) > 0 {
		placeholders := make([]string, len(opts.Filter.ExcludeIDs))
		for i := range placeholders {
			placeholders[i] = "?"
			queryArgs = append(queryArgs, opts.Filter.ExcludeIDs[i])
		}
		queryBuilder.WriteString(" AND c.id NOT IN (" + strings.Join(placeholders, ",") + ")")
	}
	return queryArgs
}

// addSimilarityThreshold adds similarity threshold filter to the query
func (r *SQLRepository) addSimilarityThreshold(queryBuilder *strings.Builder, opts *SearchOptions, queryArgs []interface{}) []interface{} {
	if opts.MinSimilarity > 0 {
		// Convert similarity threshold to distance threshold
		var distanceThreshold float64
		switch opts.Metric {
		case DistanceMetricCosine:
			distanceThreshold = 2.0 * (1.0 - opts.MinSimilarity)
		case DistanceMetricL2:
			distanceThreshold = (1.0 / opts.MinSimilarity) - 1.0
		default:
			distanceThreshold = 1.0 - opts.MinSimilarity
		}
		queryBuilder.WriteString(" AND vm.distance <= ?")
		queryArgs = append(queryArgs, distanceThreshold)
	}
	return queryArgs
}

// processVectorStoreResults processes the database rows into ScoredChunk objects
func (r *SQLRepository) processVectorStoreResults(rows *sql.Rows, opts *SearchOptions) ([]*ScoredChunk, error) {
	var results []*ScoredChunk
	for rows.Next() {
		var vsID, chunkID, vsWorkspaceID string
		var vectorType string
		var dimensions int
		var metadataJSON []byte
		var content, name, chunkType, signature string
		var startLine, endLine int
		var distance float64

		err := rows.Scan(
			&vsID, &chunkID, &vsWorkspaceID,
			&vectorType, &dimensions, &metadataJSON,
			&content, &name, &startLine, &endLine,
			&chunkType, &signature,
			&distance,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning vector store result: %w", err)
		}

		// Create chunk
		chunk := &workspace.Chunk{
			ID:          chunkID,
			WorkspaceID: vsWorkspaceID,
			Name:        name,
			Content:     content,
			ChunkType:   workspace.ChunkType(chunkType),
			Signature:   signature,
		}

		// Set position values
		chunk.StartPos.Line = startLine
		chunk.EndPos.Line = endLine

		// Parse metadata if present
		var metadata map[string]interface{}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
				r.logger.Warn("Error parsing metadata", "error", err, "chunk_id", chunkID)
			}
		}

		// Convert distance to similarity
		similarity := r.vecOps.DistanceToSimilarity(distance, opts.Metric)

		// Create result
		result := &ScoredChunk{
			Chunk:      chunk,
			Similarity: similarity,
			Metadata:   metadata,
		}

		if opts.IncludeDistance {
			result.Distance = distance
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating vector store results: %w", err)
	}

	return results, nil
}
