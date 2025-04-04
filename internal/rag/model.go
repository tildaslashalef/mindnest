// Package rag provides Retrieval Augmented Generation capabilities for the Mindnest application
package rag

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tildaslashalef/mindnest/internal/workspace"
)

var (
	// ErrVectorNotFound is returned when a vector is not found
	ErrVectorNotFound = errors.New("vector not found")

	// ErrEmbeddingGenerationFailed is returned when embedding generation fails
	ErrEmbeddingGenerationFailed = errors.New("embedding generation failed")

	// ErrInvalidEmbedding is returned when an embedding is invalid
	ErrInvalidEmbedding = errors.New("invalid embedding")
)

// Vector represents an embedding vector with its ID
type Vector struct {
	ID        int64     `json:"id"`
	Embedding []float32 `json:"embedding"`
}

// SimilarVector represents a vector with similarity score
type SimilarVector struct {
	ID         int64   `json:"id"`
	Similarity float64 `json:"similarity"`
}

// ScoredChunk represents a chunk with a similarity score
type ScoredChunk struct {
	Chunk      *workspace.Chunk `json:"chunk"`
	Similarity float64          `json:"similarity"`
}

// ChunkWithSimilarity represents a chunk with its vector similarity score
// This is used for efficient single-query retrieval of chunks with scores
type ChunkWithSimilarity struct {
	Chunk      *workspace.Chunk `json:"chunk"`
	Similarity float64          `json:"similarity"`
}

// ContextWindow represents a collection of chunks forming a context window
type ContextWindow struct {
	Chunks      []*ScoredChunk `json:"chunks"`
	TotalTokens int            `json:"total_tokens"`
}

// EstimateTokens estimates the number of tokens in a text
// This is a simple heuristic, and could be replaced with a more accurate model
func EstimateTokens(text string) int {
	// Simple heuristic: 1 token ≈ 4 characters for code
	return len(text) / 4
}

// FormatVectorForSQL converts a float32 slice to a string format for SQL
func FormatVectorForSQL(embedding []float32) string {
	if len(embedding) == 0 {
		return ""
	}

	// Use a pre-sized buffer for better performance
	var result []byte
	for i, val := range embedding {
		if i > 0 {
			result = append(result, ',')
		}
		// Format float with 6 decimal places
		result = append(result, []byte(fmt.Sprintf("%.6f", val))...)
	}
	return string(result)
}

// ParseVectorFromSQL converts a comma-separated string to a float32 slice
func ParseVectorFromSQL(embedStr string) ([]float32, error) {
	if embedStr == "" {
		return []float32{}, nil
	}

	// Split by comma
	parts := strings.Split(embedStr, ",")
	embedding := make([]float32, len(parts))

	for i, str := range parts {
		var val float64
		if _, err := fmt.Sscanf(str, "%f", &val); err != nil {
			return nil, fmt.Errorf("parsing embedding value: %w", err)
		}
		embedding[i] = float32(val)
	}

	return embedding, nil
}
