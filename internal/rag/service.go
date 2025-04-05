package rag

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/llm"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// Service provides RAG (Retrieval Augmented Generation) functionality
type Service struct {
	workspace  *workspace.Service
	vectorRepo Repository
	llmClient  llm.Client
	config     *config.Config
	logger     *loggy.Logger
}

// NewService creates a new RAG service
func NewService(
	workspaceService *workspace.Service,
	vectorRepo Repository,
	llmClient llm.Client,
	cfg *config.Config,
	logger *loggy.Logger,
) *Service {
	return &Service{
		workspace:  workspaceService,
		vectorRepo: vectorRepo,
		llmClient:  llmClient,
		config:     cfg,
		logger:     logger,
	}
}

// GenerateEmbedding generates an embedding for the given text
func (s *Service) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Generate a single embedding for the text
	embeddings, err := s.GenerateBatchEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("generating embedding: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings generated")
	}

	return embeddings[0], nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (s *Service) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	// If LLM client is not initialized, we can't generate embeddings
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not initialized, can't generate embeddings")
	}

	// Prepare embedding requests
	reqs := make([]llm.EmbeddingRequest, len(texts))
	for i, text := range texts {
		reqs[i] = llm.EmbeddingRequest{
			// Embedding model is now handled by the LLM client based on the provider type
			Text: text,
		}
	}

	// Use the LLM client to generate embeddings
	embeddings, err := s.llmClient.BatchEmbeddings(ctx, reqs)
	if err != nil {
		return nil, fmt.Errorf("generating embeddings: %w", err)
	}

	return embeddings, nil
}

// ProcessChunk processes a single chunk, generating an embedding and storing it
func (s *Service) ProcessChunk(ctx context.Context, c *workspace.Chunk) error {
	// If the chunk already has a vector ID, skip it
	if c.VectorID.Valid {
		s.logger.Debug("Chunk already has vector ID, skipping", "chunk_id", c.ID, "vector_id", c.VectorID.Int64)
		return nil
	}

	// Generate embedding for the chunk
	embedding, err := s.GenerateEmbedding(ctx, c.Content)
	if err != nil {
		return fmt.Errorf("generating embedding: %w", err)
	}

	// Store the embedding in the vector store
	vectorID, err := s.vectorRepo.StoreVector(ctx, embedding)
	if err != nil {
		return fmt.Errorf("storing vector: %w", err)
	}

	// Set the vector ID on the chunk
	c.SetVectorID(vectorID)

	// Update the chunk in the database
	err = s.updateChunk(ctx, c)
	if err != nil {
		// If updating the chunk fails, try to clean up the vector
		_ = s.vectorRepo.DeleteVector(ctx, vectorID)
		return fmt.Errorf("updating chunk: %w", err)
	}

	s.logger.Debug("Processed chunk", "chunk_id", c.ID, "vector_id", vectorID)
	return nil
}

// updateChunk updates a chunk in the workspace repository
func (s *Service) updateChunk(ctx context.Context, c *workspace.Chunk) error {
	// The workspace service doesn't have a direct UpdateChunk method,
	// so we need to create a modified implementation
	repo := s.workspace.GetRepository()
	return repo.UpdateChunk(ctx, c)
}

// getChunkByID retrieves a chunk by its ID
func (s *Service) getChunkByID(ctx context.Context, chunkID string) (*workspace.Chunk, error) {
	return s.workspace.GetChunk(ctx, chunkID)
}

// getChunksWithoutVectorID gets chunks without vector IDs
func (s *Service) getChunksWithoutVectorID(ctx context.Context, workspaceID string) ([]*workspace.Chunk, error) {
	// Get all chunks for the workspace
	chunks, err := s.workspace.GetChunksByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	// Filter chunks without vector IDs
	var pendingChunks []*workspace.Chunk
	for _, c := range chunks {
		if !c.VectorID.Valid {
			pendingChunks = append(pendingChunks, c)
		}
	}

	return pendingChunks, nil
}

// getChunksWithVectorID gets chunks with vector IDs
func (s *Service) getChunksWithVectorID(ctx context.Context, workspaceID string) ([]*workspace.Chunk, error) {
	// Get all chunks for the workspace
	chunks, err := s.workspace.GetChunksByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	// Filter chunks with vector IDs
	var matchingChunks []*workspace.Chunk
	for _, c := range chunks {
		if c.VectorID.Valid {
			matchingChunks = append(matchingChunks, c)
		}
	}

	return matchingChunks, nil
}

// ProcessChunks processes multiple chunks, generating embeddings and storing them
func (s *Service) ProcessChunks(ctx context.Context, chunks []*workspace.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Process in batches to avoid hitting API rate limits
	batchSize := s.config.RAG.BatchSize
	if batchSize <= 0 {
		batchSize = 20 // Default batch size
	}

	// Filter chunks that don't have vector IDs
	var pendingChunks []*workspace.Chunk
	for _, c := range chunks {
		if !c.VectorID.Valid {
			pendingChunks = append(pendingChunks, c)
		}
	}

	if len(pendingChunks) == 0 {
		s.logger.Debug("No chunks need processing")
		return nil
	}

	s.logger.Info("Processing chunks", "total", len(pendingChunks))

	// Process in batches
	for i := 0; i < len(pendingChunks); i += batchSize {
		end := i + batchSize
		if end > len(pendingChunks) {
			end = len(pendingChunks)
		}

		batch := pendingChunks[i:end]
		if err := s.processBatch(ctx, batch); err != nil {
			return fmt.Errorf("processing batch %d-%d: %w", i, end, err)
		}
	}

	s.logger.Info("Processed all chunks successfully", "count", len(pendingChunks))
	return nil
}

// processBatch processes a batch of chunks
func (s *Service) processBatch(ctx context.Context, chunks []*workspace.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Extract texts from chunks
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}

	// Generate embeddings for all texts in the batch
	embeddings, err := s.GenerateBatchEmbeddings(ctx, texts)
	if err != nil {
		return fmt.Errorf("generating batch embeddings: %w", err)
	}

	// Store embeddings and update chunks
	for i, c := range chunks {
		// Store the embedding
		vectorID, err := s.vectorRepo.StoreVector(ctx, embeddings[i])
		if err != nil {
			return fmt.Errorf("storing vector for chunk %s: %w", c.ID, err)
		}

		// Update the chunk with the vector ID
		c.SetVectorID(vectorID)
		if err := s.updateChunk(ctx, c); err != nil {
			// If updating the chunk fails, try to clean up the vector
			_ = s.vectorRepo.DeleteVector(ctx, vectorID)
			return fmt.Errorf("updating chunk %s: %w", c.ID, err)
		}

	}

	return nil
}

// ProcessPendingChunks processes all chunks that don't have vector IDs
func (s *Service) ProcessPendingChunks(ctx context.Context, workspaceID string) error {
	// Get all chunks without vector IDs for the workspace
	chunks, err := s.getChunksWithoutVectorID(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("fetching chunks without vector IDs: %w", err)
	}

	return s.ProcessChunks(ctx, chunks)
}

// GetSimilarChunks retrieves chunks similar to the given text
func (s *Service) GetSimilarChunks(ctx context.Context, fileID string, text string, limit int) ([]*ScoredChunk, error) {
	return s.GetSimilarChunksByWorkspace(ctx, fileID, text, "", limit)
}

// GetSimilarChunksByWorkspace retrieves chunks similar to the given text, filtered by workspace
func (s *Service) GetSimilarChunksByWorkspace(ctx context.Context, fileID string, text string, workspaceID string, limit int) ([]*ScoredChunk, error) {
	if limit <= 0 {
		limit = s.config.RAG.NSimilarChunks
	}

	// Generate an embedding for the text
	embedding, err := s.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("generating embedding: %w", err)
	}

	// Find chunks with similarity scores directly in a single query
	// This is more efficient than separate vector queries followed by chunk retrieval
	chunksWithSimilarity, err := s.vectorRepo.FindChunksWithSimilarity(
		ctx,
		embedding,
		workspaceID,
		"", // No chunk type filter by default
		"", // No file exclusion by default
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("finding chunks with similarity: %w", err)
	}

	if len(chunksWithSimilarity) == 0 {
		return []*ScoredChunk{}, nil
	}

	// Convert to ScoredChunk format
	scoredChunks := make([]*ScoredChunk, len(chunksWithSimilarity))
	for i, cwv := range chunksWithSimilarity {
		scoredChunks[i] = &ScoredChunk{
			Chunk:      cwv.Chunk,
			Similarity: cwv.Similarity,
		}
	}

	return scoredChunks, nil
}

// findChunksByVectorID finds chunks that have the given vector ID
func (s *Service) findChunksByVectorID(ctx context.Context, vectorID int64) ([]*workspace.Chunk, error) {
	// Get all chunks with vector IDs
	chunks, err := s.getChunksWithVectorID(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("getting chunks with vector IDs: %w", err)
	}

	// Filter by the specific vector ID
	var matchingChunks []*workspace.Chunk
	for _, c := range chunks {
		if c.VectorID.Valid && c.VectorID.Int64 == vectorID {
			matchingChunks = append(matchingChunks, c)
		}
	}

	return matchingChunks, nil
}

// GetRelatedChunks retrieves chunks related to the given chunk
func (s *Service) GetRelatedChunks(ctx context.Context, chunkID string, limit int) ([]*ScoredChunk, error) {
	if limit <= 0 {
		limit = s.config.RAG.NSimilarChunks
	}

	// Get the chunk
	c, err := s.getChunkByID(ctx, chunkID)
	if err != nil {
		return nil, fmt.Errorf("getting chunk: %w", err)
	}

	// Make sure the chunk has a vector ID
	if !c.VectorID.Valid {
		// Generate an embedding for the chunk first
		if err := s.ProcessChunk(ctx, c); err != nil {
			return nil, fmt.Errorf("processing chunk: %w", err)
		}
	}

	// Get the vector
	vec, err := s.vectorRepo.GetVector(ctx, c.VectorID.Int64)
	if err != nil {
		return nil, fmt.Errorf("getting vector: %w", err)
	}

	// Find similar vectors
	similarVectors, err := s.vectorRepo.FindSimilar(ctx, vec.Embedding, limit+1) // +1 to account for the chunk itself
	if err != nil {
		return nil, fmt.Errorf("finding similar vectors: %w", err)
	}

	// Collect chunks for the similar vectors, excluding the original chunk
	var scoredChunks []*ScoredChunk
	for _, sim := range similarVectors {
		// Skip the original chunk's vector
		if sim.ID == c.VectorID.Int64 {
			continue
		}

		// Find chunks with this vector ID
		relatedChunks, err := s.findChunksByVectorID(ctx, sim.ID)
		if err != nil {
			s.logger.Warn("Error finding chunk for vector", "vector_id", sim.ID, "error", err)
			continue
		}

		for _, rc := range relatedChunks {
			scoredChunks = append(scoredChunks, &ScoredChunk{
				Chunk:      rc,
				Similarity: sim.Similarity,
			})
		}
	}

	// Sort by similarity (highest first)
	sort.Slice(scoredChunks, func(i, j int) bool {
		return scoredChunks[i].Similarity > scoredChunks[j].Similarity
	})

	// Limit the results
	if len(scoredChunks) > limit {
		scoredChunks = scoredChunks[:limit]
	}

	return scoredChunks, nil
}

// BuildContextWindow builds a context window for a given text
func (s *Service) BuildContextWindow(ctx context.Context, fileID string, text string, maxTokens int) (*ContextWindow, error) {
	if maxTokens <= 0 {
		// Use a default value since LLM.MaxTokens is no longer available
		// Alternatively, we could get this from the default provider's config
		maxTokens = 2048 // Default to 2048 tokens for context
	}

	// Get similar chunks
	limit := s.config.RAG.NSimilarChunks * 2 // Get more than we need to prioritize
	scoredChunks, err := s.GetSimilarChunks(ctx, fileID, text, limit)
	if err != nil {
		return nil, fmt.Errorf("getting similar chunks: %w", err)
	}

	// Build the context window
	window := &ContextWindow{
		Chunks:      []*ScoredChunk{},
		TotalTokens: 0,
	}

	// Add chunks until we reach the max tokens
	for _, sc := range scoredChunks {
		// Estimate tokens for this chunk
		chunkTokens := EstimateTokens(sc.Chunk.Content)

		// If adding this chunk would exceed the limit, skip it
		if window.TotalTokens+chunkTokens > maxTokens {
			continue
		}

		// Add the chunk to the window
		window.Chunks = append(window.Chunks, sc)
		window.TotalTokens += chunkTokens

		// If we're close to the limit, stop
		if window.TotalTokens >= int(float64(maxTokens)*0.9) {
			break
		}
	}

	s.logger.Debug("Built context window",
		"chunks", len(window.Chunks),
		"tokens", window.TotalTokens,
		"max_tokens", maxTokens)

	return window, nil
}

// GetSimilarChunksByType retrieves chunks of a specific type that are similar to the provided text
func (s *Service) GetSimilarChunksByType(
	ctx context.Context,
	text string,
	workspaceID string,
	chunkType workspace.ChunkType,
	limit int,
) ([]*ScoredChunk, error) {
	// Get all chunks of the specified type for the workspace
	chunks, err := s.workspace.GetChunksByType(ctx, workspaceID, chunkType)
	if err != nil {
		return nil, fmt.Errorf("getting chunks: %w", err)
	}

	if len(chunks) == 0 {
		return nil, nil
	}

	// Get only chunks with vector IDs
	var chunksWithVectors []*workspace.Chunk
	for _, c := range chunks {
		if c.VectorID.Valid {
			chunksWithVectors = append(chunksWithVectors, c)
		}
	}

	if len(chunksWithVectors) == 0 {
		return nil, nil
	}

	// Get vector IDs from chunks
	vectorIDs := make([]int64, len(chunksWithVectors))
	for i, c := range chunksWithVectors {
		vectorIDs[i] = c.VectorID.Int64
	}

	// Create a map of vector ID to chunk for easy lookup
	chunkMap := make(map[int64]*workspace.Chunk)
	for _, c := range chunksWithVectors {
		chunkMap[c.VectorID.Int64] = c
	}

	// Generate embedding for the search text
	embedding, err := s.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("generating embedding: %w", err)
	}

	// Find similar vectors
	similarVectors, err := s.vectorRepo.FindSimilarByWorkspace(ctx, embedding, workspaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("finding similar vectors: %w", err)
	}

	// Convert to ScoredChunk
	scoredChunks := make([]*ScoredChunk, 0, len(similarVectors))
	for _, sv := range similarVectors {
		chunk, ok := chunkMap[sv.ID]
		if !ok {
			// This should not happen unless there's a data inconsistency
			continue
		}

		scoredChunks = append(scoredChunks, &ScoredChunk{
			Chunk:      chunk,
			Similarity: sv.Similarity,
		})
	}

	return scoredChunks, nil
}

// FindSimilarChunks finds chunks similar to the given text
func (s *Service) FindSimilarChunks(ctx context.Context, text string, workspaceID string, chunkType string, limit int) ([]*ScoredChunk, error) {
	if limit <= 0 {
		limit = s.config.RAG.NSimilarChunks
	}

	// Generate an embedding for the text
	embedding, err := s.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("generating embedding: %w", err)
	}

	// Find chunks with similarity scores directly in a single query
	chunksWithSimilarity, err := s.vectorRepo.FindChunksWithSimilarity(
		ctx,
		embedding,
		workspaceID,
		chunkType,
		"", // No file exclusion by default
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("finding chunks with similarity: %w", err)
	}

	if len(chunksWithSimilarity) == 0 {
		return []*ScoredChunk{}, nil
	}

	// Convert to ScoredChunk format
	scoredChunks := make([]*ScoredChunk, len(chunksWithSimilarity))
	for i, cwv := range chunksWithSimilarity {
		scoredChunks[i] = &ScoredChunk{
			Chunk:      cwv.Chunk,
			Similarity: cwv.Similarity,
		}
	}

	return scoredChunks, nil
}

// FindSimilarChunksExcludingFile finds chunks similar to the given text, excluding chunks from a specific file
func (s *Service) FindSimilarChunksExcludingFile(
	ctx context.Context,
	text string,
	workspaceID string,
	chunkType string,
	excludeFileID string,
	limit int,
) ([]*ScoredChunk, error) {
	if limit <= 0 {
		limit = s.config.RAG.NSimilarChunks
	}

	// Generate an embedding for the text
	embedding, err := s.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("generating embedding: %w", err)
	}

	// Find chunks with similarity scores directly in a single query
	chunksWithSimilarity, err := s.vectorRepo.FindChunksWithSimilarity(
		ctx,
		embedding,
		workspaceID,
		chunkType,
		excludeFileID,
		limit,
	)

	if err != nil {
		return nil, fmt.Errorf("finding chunks with similarity: %w", err)
	}

	if len(chunksWithSimilarity) == 0 {
		return []*ScoredChunk{}, nil
	}

	// Convert to ScoredChunk format
	scoredChunks := make([]*ScoredChunk, len(chunksWithSimilarity))
	for i, cwv := range chunksWithSimilarity {
		scoredChunks[i] = &ScoredChunk{
			Chunk:      cwv.Chunk,
			Similarity: cwv.Similarity,
		}
	}

	return scoredChunks, nil
}

// GetChunkWithVector gets a chunk and its embedding
func (s *Service) GetChunkWithVector(ctx context.Context, chunkID string) (*workspace.Chunk, []float32, error) {
	// Get the chunk via the helper method
	c, err := s.getChunkByID(ctx, chunkID)
	if err != nil {
		return nil, nil, fmt.Errorf("getting chunk: %w", err)
	}

	// Check if the chunk has a vector ID
	if !c.VectorID.Valid {
		return nil, nil, fmt.Errorf("chunk does not have a vector ID: %w", ErrVectorNotFound)
	}

	// Get the vector
	vector, err := s.vectorRepo.GetVector(ctx, c.VectorID.Int64)
	if err != nil {
		return nil, nil, fmt.Errorf("getting vector: %w", err)
	}

	return c, vector.Embedding, nil
}

// GetVectorRepo returns the vector repository
func (s *Service) GetVectorRepo() Repository {
	return s.vectorRepo
}

// processDiffChunks processes and indexes chunks from a diff
func (s *Service) processDiffChunks(ctx context.Context, pendingChunks []*workspace.Chunk) error {
	if len(pendingChunks) == 0 {
		return nil
	}

	// Set batch size
	batchSize := s.config.RAG.BatchSize
	if batchSize <= 0 {
		batchSize = 20 // Default batch size
	}

	// Process in batches to avoid overloading the LLM API
	texts := make([]string, 0, len(pendingChunks))
	for _, chunk := range pendingChunks {
		texts = append(texts, chunk.Content)
	}

	// Process chunks in batches
	for i := 0; i < len(pendingChunks); i += batchSize {
		end := i + batchSize
		if end > len(pendingChunks) {
			end = len(pendingChunks)
		}

		// Get the current batch
		batchTexts := texts[i:end]
		batchChunks := pendingChunks[i:end]

		// Generate embeddings for all texts in the batch
		embeddings, err := s.GenerateBatchEmbeddings(ctx, batchTexts)
		if err != nil {
			return fmt.Errorf("generating batch embeddings: %w", err)
		}

		// Store embeddings and update chunks
		for j, chunk := range batchChunks {
			vectorID, err := s.vectorRepo.StoreVector(ctx, embeddings[j])
			if err != nil {
				return fmt.Errorf("storing vector: %w", err)
			}
			chunk.SetVectorID(vectorID)
		}

		// Allow a short pause between batches to avoid rate limiting
		if end < len(pendingChunks) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

// FindContext finds context chunks for a given file path
func (s *Service) FindContext(ctx context.Context, workspaceID string, filePath string, limit int) ([]*workspace.Chunk, error) {
	if limit <= 0 {
		limit = s.config.RAG.NSimilarChunks
	}

	// This is just a stub - you would implement the actual functionality here
	return nil, nil
}

// FindFileContext finds context chunks for a new file
func (s *Service) FindFileContext(ctx context.Context, workspaceID string, filePath string, limit int) ([]*workspace.Chunk, error) {
	if limit <= 0 {
		limit = s.config.RAG.NSimilarChunks
	}

	// This is just a stub - you would implement the actual functionality here
	return nil, nil
}

// RelatedLine represents a line of code and its relevance score
type RelatedLine struct {
	LineNumber int
	Content    string
	Similarity float64
}

// FindRelatedLines finds lines related to a given line in the file
func (s *Service) FindRelatedLines(ctx context.Context, workspaceID string, filePath string, lineNumber int, limit int) ([]RelatedLine, error) {
	if limit <= 0 {
		limit = s.config.RAG.NSimilarChunks * 2 // Get more than we need to prioritize
	}

	// This is just a stub - you would implement the actual functionality here
	return nil, nil
}

// FindSimilarChanges finds changes similar to a given change
func (s *Service) FindSimilarChanges(ctx context.Context, workspaceID string, chunk *workspace.Chunk, limit int) ([]*workspace.Chunk, error) {
	if limit <= 0 {
		limit = s.config.RAG.NSimilarChunks
	}

	// This is just a stub - you would implement the actual functionality here
	return nil, nil
}

// FindFunctionContext finds context chunks for a function
func (s *Service) FindFunctionContext(ctx context.Context, workspaceID string, functionName string, limit int) ([]*workspace.Chunk, error) {
	if limit <= 0 {
		limit = s.config.RAG.NSimilarChunks
	}

	// This is just a stub - you would implement the actual functionality here
	return nil, nil
}
