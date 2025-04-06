// Package rag provides an optimized retrieval augmented generation system
// with advanced vector operations for Mindnest.
package rag

import (
	"errors"
	"time"

	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// Vector types and storage format options
type VectorType string

const (
	// VectorTypeFloat32 is the standard 32-bit floating point vector type
	VectorTypeFloat32 VectorType = "float32"
	// VectorTypeInt8 provides 8-bit quantization for storage efficiency
	VectorTypeInt8 VectorType = "int8"
	// VectorTypeBit provides binary quantization for maximum storage efficiency
	VectorTypeBit VectorType = "bit"
	// VectorTypeBinary is an alias for VectorTypeBit (for compatibility)
	VectorTypeBinary VectorType = "binary"
)

// DistanceMetric defines the distance calculation method for vector similarity
type DistanceMetric string

const (
	// DistanceMetricCosine computes cosine distance (1 - cosine similarity)
	// Best for semantic similarity searches
	DistanceMetricCosine DistanceMetric = "cosine"

	// DistanceMetricL2 computes Euclidean (L2) distance
	// Good for feature vectors where magnitude matters
	DistanceMetricL2 DistanceMetric = "l2"

	// DistanceMetricDot computes dot product similarity
	DistanceMetricDot DistanceMetric = "dot"

	// DistanceMetricHamming computes Hamming distance (bit difference)
	// Best for binary vectors
	DistanceMetricHamming DistanceMetric = "hamming"
)

// Common errors returned by this package
var (
	// ErrInvalidVector is returned when a vector is invalid
	ErrInvalidVector = errors.New("invalid vector")

	// ErrVectorNotFound is returned when a vector is not found
	ErrVectorNotFound = errors.New("vector not found")

	// ErrEmbeddingFailed is returned when embedding generation fails
	ErrEmbeddingFailed = errors.New("embedding generation failed")

	// ErrExtensionNotLoaded is returned when SQLite-vec extension is not available
	ErrExtensionNotLoaded = errors.New("sqlite-vec extension not loaded")

	// ErrUnsupportedOperation is returned for operations that are not supported
	// by the vector type or method
	ErrUnsupportedOperation = errors.New("unsupported operation")
)

// Vector represents an embedding vector with its metadata
type Vector struct {
	ID        int64      `json:"id"`
	Embedding []float32  `json:"embedding"`
	Type      VectorType `json:"type,omitempty"`
}

// ScoredVector represents a vector with similarity score
type ScoredVector struct {
	ID         int64   `json:"id"`
	Similarity float64 `json:"similarity"`
	Distance   float64 `json:"distance,omitempty"`
}

// ScoredChunk represents a chunk with its similarity score to a query
type ScoredChunk struct {
	Chunk      *workspace.Chunk `json:"chunk"`
	Similarity float64          `json:"similarity"`
	Distance   float64          `json:"distance,omitempty"`
	Metadata   map[string]any   `json:"metadata,omitempty"`
}

// SearchFilter represents a filter to apply when searching for vectors
type SearchFilter struct {
	WorkspaceID    string
	FileID         string
	FileIDs        []string
	ChunkIDs       []string
	ExcludeIDs     []string
	CreatedAfter   *time.Time
	MetadataFilter map[string]interface{}
}

// SearchOptions provides configuration options for similarity search
type SearchOptions struct {
	// Distance/similarity metric to use
	Metric DistanceMetric

	// Number of results to return (K in KNN)
	Limit int

	// Whether to normalize the query vector
	Normalization bool

	// Filter to apply to the search
	Filter *SearchFilter

	// Type of vector compression to use for the query
	CompressionType VectorType

	// Similarity threshold (0.0-1.0)
	MinSimilarity float64

	// Whether to include similarity scores in results
	IncludeScores bool

	// WorkspaceID filters results by workspace
	WorkspaceID string

	// ChunkType filters results by chunk type
	ChunkType workspace.ChunkType

	// ExcludeFileID excludes chunks from this file
	ExcludeFileID string

	// IncludeDistance includes raw distance values in results
	IncludeDistance bool
}

// NewSearchOptions returns default search options
func NewSearchOptions() *SearchOptions {
	return &SearchOptions{
		Limit:           10,
		Metric:          DistanceMetricCosine,
		Normalization:   true,
		CompressionType: VectorTypeFloat32,
		MinSimilarity:   0.0, // No threshold by default
		IncludeScores:   true,
	}
}

// WithWorkspace sets the workspace filter
func (o *SearchOptions) WithWorkspace(workspaceID string) *SearchOptions {
	if o.Filter == nil {
		o.Filter = &SearchFilter{}
	}
	o.Filter.WorkspaceID = workspaceID
	// For backward compatibility
	o.WorkspaceID = workspaceID
	return o
}

// WithFile sets the file filter
func (o *SearchOptions) WithFile(fileID string) *SearchOptions {
	if o.Filter == nil {
		o.Filter = &SearchFilter{}
	}
	o.Filter.FileID = fileID
	return o
}

// WithFiles sets multiple file IDs for filtering
func (o *SearchOptions) WithFiles(fileIDs []string) *SearchOptions {
	if o.Filter == nil {
		o.Filter = &SearchFilter{}
	}
	o.Filter.FileIDs = fileIDs
	return o
}

// WithLimit sets the result limit
func (o *SearchOptions) WithLimit(limit int) *SearchOptions {
	o.Limit = limit
	return o
}

// WithMetric sets the distance metric
func (o *SearchOptions) WithMetric(metric DistanceMetric) *SearchOptions {
	o.Metric = metric
	return o
}

// WithMinSimilarity sets the minimum similarity threshold
func (o *SearchOptions) WithMinSimilarity(threshold float64) *SearchOptions {
	o.MinSimilarity = threshold
	return o
}

// WithNormalization enables or disables vector normalization
func (o *SearchOptions) WithNormalization(enable bool) *SearchOptions {
	o.Normalization = enable
	return o
}

// WithExcludeIDs excludes specific chunk IDs from results
func (o *SearchOptions) WithExcludeIDs(ids []string) *SearchOptions {
	if o.Filter == nil {
		o.Filter = &SearchFilter{}
	}
	o.Filter.ExcludeIDs = ids
	return o
}

// WithCompression sets the vector compression type
func (o *SearchOptions) WithCompression(compressionType VectorType) *SearchOptions {
	o.CompressionType = compressionType
	return o
}

// WithChunkType sets the chunk type filter
func (o *SearchOptions) WithChunkType(chunkType workspace.ChunkType) *SearchOptions {
	o.ChunkType = chunkType
	return o
}

// WithExcludeFile sets the file to exclude from search results
func (o *SearchOptions) WithExcludeFile(fileID string) *SearchOptions {
	o.ExcludeFileID = fileID
	return o
}

// WithConfigDefaults applies defaults from the configuration to the search options
func (o *SearchOptions) WithConfigDefaults(cfg *config.Config) *SearchOptions {
	if cfg == nil {
		return o
	}

	// Apply similar chunks limit from config
	if o.Limit <= 0 {
		o.Limit = GetSimilarChunksLimit(cfg)
	}

	// Apply default metric from config
	o.Metric = DefaultMetric(cfg)

	// Apply normalization setting from config
	if cfg.RAG.Normalization {
		o.Normalization = true
	}

	// Apply minimum similarity threshold from config
	if cfg.RAG.MinSimilarity > 0 && o.MinSimilarity == 0 {
		o.MinSimilarity = cfg.RAG.MinSimilarity
	}

	// Apply vector compression type from config
	if cfg.RAG.EnableCompression {
		o.CompressionType = DefaultCompressionType(cfg)
	}

	// Future: Apply MaxFilesSameDir and ContextDepth limits when implementing
	// directory-aware context building

	return o
}

// ChunkProcessor is a function that processes a chunk
type ChunkProcessor func(*workspace.Chunk) (*workspace.Chunk, error)

// EstimateTokens estimates the number of tokens in a text
// using a simple heuristic (1 token ≈ 4 characters for code)
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(text) / 4
}

// SearchResult represents a single result from a vector similarity search
type SearchResult struct {
	ChunkID    string
	Similarity float64
	Metadata   map[string]interface{}
}

// SearchResults represents a collection of search results
type SearchResults struct {
	Results         []SearchResult
	QueryVector     []float32
	TotalFound      int
	Metric          DistanceMetric
	ExecutionTimeMs int64
}
