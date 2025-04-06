package rag

import (
	"github.com/tildaslashalef/mindnest/internal/config"
)

// DefaultMetric returns the default distance metric to use
// NOTE: Currently only cosine distance is supported by the vector_index table.
// Other metrics in the config will be ignored until we implement configurable distance metrics.
func DefaultMetric(cfg *config.Config) DistanceMetric {
	// Check if a metric is specified in config
	if cfg != nil && cfg.RAG.DefaultMetric != "" {
		switch cfg.RAG.DefaultMetric {
		case "cosine":
			return DistanceMetricCosine
		case "l2":
			return DistanceMetricL2
		case "dot":
			return DistanceMetricDot
		case "hamming":
			return DistanceMetricHamming
		}
	}

	// By default, use cosine similarity for code embedding search
	return DistanceMetricCosine
}

// DefaultCompressionType returns the default vector compression type to use
func DefaultCompressionType(cfg *config.Config) VectorType {
	// Check if a compression type is specified in config
	if cfg != nil && cfg.RAG.EnableCompression && cfg.RAG.VectorType != "" {
		switch cfg.RAG.VectorType {
		case "float32":
			return VectorTypeFloat32
		case "int8":
			return VectorTypeInt8
		case "binary", "bit":
			return VectorTypeBinary
		}
	}

	// By default, use float32 for highest precision
	return VectorTypeFloat32
}

// GetBatchSize returns the batch size for processing chunks
func GetBatchSize(cfg *config.Config) int {
	if cfg.RAG.BatchSize <= 0 {
		return 20 // Default batch size
	}
	return cfg.RAG.BatchSize
}

// GetSimilarChunksLimit returns the default limit for similar chunks
func GetSimilarChunksLimit(cfg *config.Config) int {
	if cfg.RAG.NSimilarChunks <= 0 {
		return 10 // Default number of similar chunks
	}
	return cfg.RAG.NSimilarChunks
}

// RAGConfigExtension provides extended configuration for the RAG2 package
// These can be stored in a separate metadata table or JSON config file
type RAGConfigExtension struct {
	// Vector storage options
	DefaultMetric      DistanceMetric `json:"default_metric"`
	EnableCompression  bool           `json:"enable_compression"`
	CompressionType    VectorType     `json:"compression_type"`
	NormalizeByDefault bool           `json:"normalize_by_default"`

	// Advanced options
	AdaptiveDimensions bool `json:"adaptive_dimensions"`
	MinimumDimensions  int  `json:"minimum_dimensions"`
	ContextWindowSize  int  `json:"context_window_size"`
	TokensPerChunk     int  `json:"tokens_per_chunk"`
}

// NewDefaultExtension creates a default RAG2 config extension
func NewDefaultExtension() *RAGConfigExtension {
	return &RAGConfigExtension{
		DefaultMetric:      DistanceMetricCosine,
		EnableCompression:  false,
		CompressionType:    VectorTypeFloat32,
		NormalizeByDefault: true,
		AdaptiveDimensions: false,
		MinimumDimensions:  128,
		ContextWindowSize:  2048,
		TokensPerChunk:     256,
	}
}

// ApplyExtension applies RAG2 extension settings to search options
func ApplyExtension(opts *SearchOptions, ext *RAGConfigExtension) *SearchOptions {
	if opts == nil {
		opts = NewSearchOptions()
	}

	if ext == nil {
		return opts
	}

	// Apply extension settings to options
	opts.Metric = ext.DefaultMetric
	opts.Normalization = ext.NormalizeByDefault

	if ext.EnableCompression {
		opts.CompressionType = ext.CompressionType
	}

	return opts
}
