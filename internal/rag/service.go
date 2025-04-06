package rag

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/llm"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// Service provides advanced vector operations for code similarity search and RAG functionality
type Service struct {
	workspace *workspace.Service
	repo      Repository
	llmClient llm.Client
	config    *config.Config
	logger    *loggy.Logger
	db        *sql.DB
}

// NewService creates a new RAG2 service
func NewService(
	workspaceService *workspace.Service,
	db *sql.DB,
	llmClient llm.Client,
	cfg *config.Config,
	logger *loggy.Logger,
) *Service {
	repo := NewRepository(db, logger)
	vecOps := NewVecOps(db, logger)

	// Create service
	service := &Service{
		workspace: workspaceService,
		repo:      repo,
		llmClient: llmClient,
		config:    cfg,
		logger:    logger,
		db:        db,
	}

	// Check vector extension capabilities
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Get extension version
		version, err := vecOps.IsExtensionLoaded(ctx)
		if err != nil {
			logger.Error("SQLite-vec extension not loaded", "error", err)
			return
		}
		logger.Info("SQLite-vec extension loaded", "version", version)

		// Verify capabilities
		capabilities, err := vecOps.VerifyVectorExtensionCapabilities(ctx)
		if err != nil {
			logger.Error("Failed to verify vector extension capabilities", "error", err)
			return
		}

		// Log capabilities
		supportedMetrics := []string{}
		for metric, supported := range capabilities {
			if supported && (strings.HasSuffix(metric, "_distance") || metric == "dot_product") {
				supportedMetrics = append(supportedMetrics, metric)
			}
		}
		logger.Info("Vector search capabilities",
			"metrics", strings.Join(supportedMetrics, ", "),
			"vec0", capabilities["vec0"])
	}()

	return service
}

// GenerateEmbedding generates an embedding for a single piece of text
func (s *Service) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text: %w", ErrInvalidVector)
	}

	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not initialized: %w", ErrEmbeddingFailed)
	}

	embeddings, err := s.GenerateBatchEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("generating embedding: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding generated: %w", ErrEmbeddingFailed)
	}

	return embeddings[0], nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (s *Service) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("empty text batch: %w", ErrInvalidVector)
	}

	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not initialized: %w", ErrEmbeddingFailed)
	}

	// Prepare embedding requests
	reqs := make([]llm.EmbeddingRequest, len(texts))
	for i, text := range texts {
		reqs[i] = llm.EmbeddingRequest{
			Text: text,
		}
	}

	// Generate embeddings
	return s.llmClient.BatchEmbeddings(ctx, reqs)
}

// ProcessChunk processes a single chunk, generating an embedding and storing it in the vector repository
// Deprecated: Use StoreVectorWithMetadata instead
func (s *Service) ProcessChunk(ctx context.Context, chunk *workspace.Chunk) error {
	if chunk == nil {
		return fmt.Errorf("nil chunk")
	}

	// Check if the chunk already has a vector in the vector_store
	exists, err := s.chunkHasVector(ctx, chunk.ID)
	if err != nil {
		return fmt.Errorf("checking if chunk has vector: %w", err)
	}

	if exists {
		s.logger.Debug("Chunk already has a vector in store, skipping",
			"chunk_id", chunk.ID)
		return nil
	}

	// Generate embedding is now handled in StoreVectorWithMetadata
	return s.StoreVectorWithMetadata(ctx, chunk, nil)
}

// chunkHasVector checks if a chunk already has a vector in vector_store
func (s *Service) chunkHasVector(ctx context.Context, chunkID string) (bool, error) {
	query := "SELECT 1 FROM vector_store WHERE chunk_id = ? LIMIT 1"
	row := s.db.QueryRowContext(ctx, query, chunkID)

	var exists int
	err := row.Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// updateChunk updates a chunk in the workspace repository
func (s *Service) updateChunk(ctx context.Context, chunk *workspace.Chunk) error {
	repo := s.workspace.GetRepository()
	return repo.UpdateChunk(ctx, chunk)
}

// ProcessChunks processes multiple chunks in batches
// Deprecated: Use ProcessChunksWithMetadata instead
func (s *Service) ProcessChunks(ctx context.Context, chunks []*workspace.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Convert to using ProcessChunksWithMetadata
	return s.ProcessChunksWithMetadata(ctx, chunks, nil)
}

// ProcessChunksWithMetadata processes multiple chunks with metadata in batches
func (s *Service) ProcessChunksWithMetadata(ctx context.Context, chunks []*workspace.Chunk, metadataFn func(*workspace.Chunk) map[string]interface{}) error {
	if len(chunks) == 0 {
		return nil
	}

	// Get batch size from config
	batchSize := GetBatchSize(s.config)
	s.logger.Info("Processing chunks with metadata", "count", len(chunks), "batch_size", batchSize)

	// Process chunks in batches
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		if err := s.processBatchWithMetadata(ctx, batch, metadataFn); err != nil {
			return fmt.Errorf("processing batch %d-%d: %w", i, end, err)
		}

		s.logger.Debug("Processed batch with metadata", "from", i, "to", end)
	}

	return nil
}

// processBatchWithMetadata processes a batch of chunks with metadata
func (s *Service) processBatchWithMetadata(ctx context.Context, chunks []*workspace.Chunk, metadataFn func(*workspace.Chunk) map[string]interface{}) error {
	if len(chunks) == 0 {
		return nil
	}

	// Extract text from chunks
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}

	// Generate embeddings for all texts in the batch
	embeddings, err := s.GenerateBatchEmbeddings(ctx, texts)
	if err != nil {
		return fmt.Errorf("generating batch embeddings: %w", err)
	}

	// Store each embedding with its metadata
	for i, c := range chunks {
		// Generate metadata if a function is provided
		var metadata map[string]interface{}
		if metadataFn != nil {
			metadata = metadataFn(c)
		}

		// Store the embedding with metadata
		_, err := s.repo.StoreVectorWithMetadata(
			ctx,
			embeddings[i],
			c.ID,
			c.WorkspaceID,
			metadata,
		)
		if err != nil {
			return fmt.Errorf("storing vector with metadata for chunk %s: %w", c.ID, err)
		}
	}

	return nil
}

// ProcessPendingChunks processes all chunks in a workspace that don't have vectors
func (s *Service) ProcessPendingChunks(ctx context.Context, workspaceID string) error {
	// Get all chunks for the workspace
	chunks, err := s.workspace.GetChunksByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("fetching chunks: %w", err)
	}

	// Get chunks that already have vectors
	chunkIDs := make([]string, len(chunks))
	for i, c := range chunks {
		chunkIDs[i] = c.ID
	}

	chunksWithVectors, err := s.getChunksWithVectors(ctx, chunkIDs)
	if err != nil {
		return fmt.Errorf("checking for existing vectors: %w", err)
	}

	// Filter chunks without vectors
	var pendingChunks []*workspace.Chunk
	for _, c := range chunks {
		if !chunksWithVectors[c.ID] {
			pendingChunks = append(pendingChunks, c)
		}
	}

	if len(pendingChunks) == 0 {
		s.logger.Info("No pending chunks to process in workspace", "workspace_id", workspaceID)
		return nil
	}

	// Process the pending chunks
	s.logger.Info("Processing pending chunks", "count", len(pendingChunks), "workspace_id", workspaceID)
	return s.ProcessChunks(ctx, pendingChunks)
}

// getChunksWithVectors returns a map of chunk IDs that already have vectors
func (s *Service) getChunksWithVectors(ctx context.Context, chunkIDs []string) (map[string]bool, error) {
	if len(chunkIDs) == 0 {
		return make(map[string]bool), nil
	}

	// Build query with placeholders for the IN clause
	placeholders := make([]string, len(chunkIDs))
	args := make([]interface{}, len(chunkIDs))
	for i, id := range chunkIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("SELECT chunk_id FROM vector_store WHERE chunk_id IN (%s)",
		strings.Join(placeholders, ","))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Create map of chunk IDs that have vectors
	result := make(map[string]bool)
	for rows.Next() {
		var chunkID string
		if err := rows.Scan(&chunkID); err != nil {
			return nil, err
		}
		result[chunkID] = true
	}

	return result, rows.Err()
}

// FindSimilarChunks finds chunks similar to the given text
// Deprecated: Use FindSimilarUsingStore instead
func (s *Service) FindSimilarChunks(ctx context.Context, text string, opts *SearchOptions) ([]*ScoredChunk, error) {
	// Forward to the new store-based implementation
	return s.FindSimilarUsingStore(ctx, text, opts)
}

// FindSimilarByDocuments finds chunks similar to the given text, but only in specific documents/files
func (s *Service) FindSimilarByDocuments(ctx context.Context, text string, workspaceID string, fileIDs []string, limit int) ([]*ScoredChunk, error) {
	opts := NewSearchOptions().
		WithWorkspace(workspaceID).
		WithFiles(fileIDs).
		WithLimit(limit)

	return s.FindSimilarChunks(ctx, text, opts)
}

// Context represents a collection of chunks that form a context window for LLM operations
type Context struct {
	Chunks      []*ScoredChunk `json:"chunks"`
	TotalTokens int            `json:"total_tokens"`
}

// BuildContext builds a context window from chunks similar to a query text
func (s *Service) BuildContext(ctx context.Context, queryText string, opts *SearchOptions, maxTokens int) (*Context, error) {
	if maxTokens <= 0 {
		maxTokens = 2048 // Default token limit
	}

	// Find similar chunks
	chunks, err := s.FindSimilarChunks(ctx, queryText, opts)
	if err != nil {
		return nil, fmt.Errorf("finding similar chunks: %w", err)
	}

	if len(chunks) == 0 {
		return &Context{
			Chunks:      []*ScoredChunk{},
			TotalTokens: 0,
		}, nil
	}

	// Calculate total tokens and filter chunks to fit within token limit
	context := &Context{
		Chunks:      []*ScoredChunk{},
		TotalTokens: 0,
	}

	// Sort by similarity (highest first)
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Similarity > chunks[j].Similarity
	})

	// Add chunks until we reach the token limit
	for _, chunk := range chunks {
		tokens := EstimateTokens(chunk.Chunk.Content)
		if context.TotalTokens+tokens > maxTokens {
			// Skip this chunk if it would exceed the token limit
			continue
		}

		context.Chunks = append(context.Chunks, chunk)
		context.TotalTokens += tokens

		// Stop if we're close to the token limit
		if context.TotalTokens >= int(float64(maxTokens)*0.9) {
			break
		}
	}

	s.logger.Debug("Built context",
		"chunks", len(context.Chunks),
		"tokens", context.TotalTokens,
		"max_tokens", maxTokens)

	return context, nil
}

// GetContext is a convenience method to build a context window for a file
func (s *Service) GetContext(ctx context.Context, fileID string, content string, maxTokens int) (*Context, error) {
	// Create search options with defaults from config
	opts := NewSearchOptions().WithConfigDefaults(s.config)

	if fileID != "" {
		opts.WithExcludeFile(fileID) // Exclude the current file from search results
	}

	return s.BuildContext(ctx, content, opts, maxTokens)
}

// ProcessChunkWithOptions processes a chunk with advanced vector options
// Deprecated: Use StoreVectorWithMetadata instead
func (s *Service) ProcessChunkWithOptions(
	ctx context.Context,
	chunk *workspace.Chunk,
	normalize bool,
	compressionType VectorType,
) error {
	if chunk == nil {
		return fmt.Errorf("nil chunk")
	}

	// Check if the chunk already has a vector in the vector_store
	exists, err := s.chunkHasVector(ctx, chunk.ID)
	if err != nil {
		return fmt.Errorf("checking if chunk has vector: %w", err)
	}

	if exists {
		s.logger.Debug("Chunk already has a vector in store, skipping",
			"chunk_id", chunk.ID)
		return nil
	}

	// Create metadata with options
	metadata := map[string]interface{}{
		"normalized":       normalize,
		"compression_type": string(compressionType),
	}

	// Use the new metadata-based approach
	return s.StoreVectorWithMetadata(ctx, chunk, metadata)
}

// ProcessChunksWithComplexity processes chunks with adaptive dimensionality
// based on complexity levels (simple, medium, complex)
func (s *Service) ProcessChunksWithComplexity(
	ctx context.Context,
	chunks []*workspace.Chunk,
	complexityFn func(*workspace.Chunk) string,
) error {
	if len(chunks) == 0 {
		return nil
	}

	// Get chunks that already have vectors
	chunkIDs := make([]string, len(chunks))
	for i, c := range chunks {
		if c != nil {
			chunkIDs[i] = c.ID
		}
	}

	chunksWithVectors, err := s.getChunksWithVectors(ctx, chunkIDs)
	if err != nil {
		return fmt.Errorf("checking for existing vectors: %w", err)
	}

	// Filter chunks that don't have vectors
	var pendingChunks []*workspace.Chunk
	for _, c := range chunks {
		if c != nil && !chunksWithVectors[c.ID] {
			pendingChunks = append(pendingChunks, c)
		}
	}

	if len(pendingChunks) == 0 {
		s.logger.Debug("No chunks need processing")
		return nil
	}

	// Generate all embeddings first
	texts := make([]string, len(pendingChunks))
	for i, c := range pendingChunks {
		texts[i] = c.Content
	}

	embeddings, err := s.GenerateBatchEmbeddings(ctx, texts)
	if err != nil {
		return fmt.Errorf("generating batch embeddings: %w", err)
	}

	// Process each chunk based on complexity
	for i, chunk := range pendingChunks {
		// Determine complexity
		complexity := complexityFn(chunk)

		var dimensions int

		// Choose dimensions based on complexity
		switch complexity {
		case "simple":
			// Use 1/4 of dimensions for simple code
			dimensions = len(embeddings[i]) / 4
		case "medium":
			// Use 1/2 of dimensions for medium complexity
			dimensions = len(embeddings[i]) / 2
		default: // "complex" or unknown
			// Use full dimensions for complex code
			dimensions = len(embeddings[i])
		}

		// Make sure dimensions is at least 16
		if dimensions < 16 {
			dimensions = 16
		}

		// Store with appropriate dimensions
		vectorID, err := s.repo.StoreVectorWithDimensions(ctx, embeddings[i], dimensions)
		if err != nil {
			return fmt.Errorf("storing vector for chunk %s: %w", chunk.ID, err)
		}

		// Create a mapping in vector_store table
		metadata := map[string]interface{}{
			"complexity": complexity,
			"dimensions": dimensions,
		}

		_, err = s.repo.StoreVectorWithMetadata(
			ctx,
			embeddings[i],
			chunk.ID,
			chunk.WorkspaceID,
			metadata,
		)

		if err != nil {
			// Clean up the vector if storing failed
			_ = s.repo.DeleteVector(ctx, vectorID)
			return fmt.Errorf("storing vector metadata for chunk %s: %w", chunk.ID, err)
		}

		s.logger.Debug("Processed chunk with complexity-based dimensions",
			"chunk_id", chunk.ID,
			"complexity", complexity,
			"dimensions", dimensions)
	}

	return nil
}

// FindSimilarWithOptions finds similar chunks with the specified distance metric
// Deprecated: Use FindSimilarUsingStore instead
func (s *Service) FindSimilarWithOptions(
	ctx context.Context,
	text string,
	opts *SearchOptions,
) ([]*ScoredChunk, error) {
	// Forward to the new store-based implementation
	return s.FindSimilarUsingStore(ctx, text, opts)
}

// StoreVectorWithMetadata stores a vector with metadata for a chunk
func (s *Service) StoreVectorWithMetadata(
	ctx context.Context,
	chunk *workspace.Chunk,
	metadata map[string]interface{},
) error {
	if chunk == nil {
		return fmt.Errorf("nil chunk")
	}

	// Generate embedding
	embedding, err := s.GenerateEmbedding(ctx, chunk.Content)
	if err != nil {
		return fmt.Errorf("generating embedding: %w", err)
	}

	// Store with metadata
	_, err = s.repo.StoreVectorWithMetadata(
		ctx,
		embedding,
		chunk.ID,
		chunk.WorkspaceID,
		metadata,
	)
	if err != nil {
		return fmt.Errorf("storing vector with metadata: %w", err)
	}

	// We don't update the chunk.VectorID here because the new approach uses direct
	// relationships between chunks and vectors in the vector_store table

	s.logger.Debug("Stored vector with metadata",
		"chunk_id", chunk.ID,
		"metadata_keys", strings.Join(mapKeys(metadata), ", "))
	return nil
}

// Helper function to get map keys as a slice
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// FindSimilarUsingStore finds chunks similar to text using the optimized vector store
func (s *Service) FindSimilarUsingStore(ctx context.Context, text string, opts *SearchOptions) ([]*ScoredChunk, error) {
	if text == "" {
		return nil, fmt.Errorf("empty query text")
	}

	if opts == nil {
		// Use defaults from config when no options provided
		opts = NewSearchOptions().WithConfigDefaults(s.config)
	}

	// Generate embedding for the query text
	embedding, err := s.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("generating embedding: %w", err)
	}

	// Use the new vector store search method
	return s.repo.FindSimilarInStore(ctx, embedding, opts)
}

// BuildContextWithStore builds a context window using the optimized vector store
func (s *Service) BuildContextWithStore(ctx context.Context, queryText string, opts *SearchOptions, maxTokens int) (*Context, error) {
	if maxTokens <= 0 {
		maxTokens = 2048 // Default token limit
	}

	// Find similar chunks using the vector store
	chunks, err := s.FindSimilarUsingStore(ctx, queryText, opts)
	if err != nil {
		return nil, fmt.Errorf("finding similar chunks in store: %w", err)
	}

	if len(chunks) == 0 {
		return &Context{
			Chunks:      []*ScoredChunk{},
			TotalTokens: 0,
		}, nil
	}

	// Calculate total tokens and filter chunks to fit within token limit
	context := &Context{
		Chunks:      []*ScoredChunk{},
		TotalTokens: 0,
	}

	// Sort by similarity (highest first)
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Similarity > chunks[j].Similarity
	})

	// Add chunks until we reach the token limit
	for _, chunk := range chunks {
		tokens := EstimateTokens(chunk.Chunk.Content)
		if context.TotalTokens+tokens > maxTokens {
			// Skip this chunk if it would exceed the token limit
			continue
		}

		context.Chunks = append(context.Chunks, chunk)
		context.TotalTokens += tokens

		// Stop if we're close to the token limit
		if context.TotalTokens >= int(float64(maxTokens)*0.9) {
			break
		}
	}

	s.logger.Debug("Built context using vector store",
		"chunks", len(context.Chunks),
		"tokens", context.TotalTokens,
		"max_tokens", maxTokens)

	return context, nil
}
