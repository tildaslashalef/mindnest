package ollama

import (
	"time"
)

// Message represents a chat message with role and content
type Message struct {
	Role    string   `json:"role"`             // "user", "assistant", or "system"
	Content string   `json:"content"`          // Message content
	Images  []string `json:"images,omitempty"` // Base64 encoded images for multimodal models
}

// ChatRequest represents a request to the /api/chat endpoint
type ChatRequest struct {
	Model    string          `json:"model"`             // Model name (required)
	Messages []Message       `json:"messages"`          // Chat messages
	Format   *ResponseFormat `json:"format,omitempty"`  // Optional format specification
	Stream   bool            `json:"stream"`            // Whether to stream the response
	Options  *RequestOptions `json:"options,omitempty"` // Optional generation parameters
}

// ChatResponse represents a response from the /api/chat endpoint
type ChatResponse struct {
	Model              string    `json:"model"`                    // Model name
	CreatedAt          time.Time `json:"created_at"`               // Creation timestamp
	Message            Message   `json:"message"`                  // Response message
	Done               bool      `json:"done"`                     // Whether generation is complete
	DoneReason         string    `json:"done_reason,omitempty"`    // Reason for completion
	TotalDuration      int64     `json:"total_duration,omitempty"` // Total time in nanoseconds
	LoadDuration       int64     `json:"load_duration,omitempty"`  // Model load time in nanoseconds
	PromptEvalCount    int       `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64     `json:"prompt_eval_duration,omitempty"`
	EvalCount          int       `json:"eval_count,omitempty"`
	EvalDuration       int64     `json:"eval_duration,omitempty"`
	Error              string    `json:"error,omitempty"` // Error message if any
}

// ResponseFormat specifies structured output format
type ResponseFormat struct {
	Type       string                 `json:"type"`                 // Format type, e.g., "json"
	Properties map[string]interface{} `json:"properties,omitempty"` // Properties for JSON schema
	Required   []string               `json:"required,omitempty"`   // Required fields
}

// GenerateRequest represents a request to the /api/generate endpoint
type GenerateRequest struct {
	Model     string          `json:"model"`                // Model name (required)
	Prompt    string          `json:"prompt"`               // Text prompt
	System    string          `json:"system,omitempty"`     // System message
	Template  string          `json:"template,omitempty"`   // Custom prompt template
	Context   []int           `json:"context,omitempty"`    // Context from previous requests
	Format    *ResponseFormat `json:"format,omitempty"`     // Format specification
	Stream    bool            `json:"stream"`               // Whether to stream the response
	Raw       bool            `json:"raw,omitempty"`        // Whether to use raw prompting
	Images    []string        `json:"images,omitempty"`     // Base64 encoded images
	Options   *RequestOptions `json:"options,omitempty"`    // Generation parameters
	KeepAlive string          `json:"keep_alive,omitempty"` // Duration to keep model loaded
}

// GenerateResponse represents a response from the /api/generate endpoint
type GenerateResponse struct {
	Model              string    `json:"model"`
	CreatedAt          time.Time `json:"created_at"`
	Response           string    `json:"response"`
	Done               bool      `json:"done"`
	Context            []int     `json:"context,omitempty"`
	TotalDuration      int64     `json:"total_duration,omitempty"`
	LoadDuration       int64     `json:"load_duration,omitempty"`
	PromptEvalCount    int       `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64     `json:"prompt_eval_duration,omitempty"`
	EvalCount          int       `json:"eval_count,omitempty"`
	EvalDuration       int64     `json:"eval_duration,omitempty"`
	Error              string    `json:"error,omitempty"`
}

// EmbeddingRequest represents a request to the /api/embed endpoint
type EmbeddingRequest struct {
	Model     string          `json:"model"`                // Model name (required)
	Input     interface{}     `json:"input"`                // String or array of strings
	Options   *RequestOptions `json:"options,omitempty"`    // Generation parameters
	Truncate  *bool           `json:"truncate,omitempty"`   // Whether to truncate input
	KeepAlive string          `json:"keep_alive,omitempty"` // Duration to keep model loaded
}

// SingleEmbeddingRequest represents a request to the legacy /api/embeddings endpoint
type SingleEmbeddingRequest struct {
	Model     string          `json:"model"`                // Model name (required)
	Prompt    string          `json:"prompt"`               // Text to embed
	Options   *RequestOptions `json:"options,omitempty"`    // Generation parameters
	KeepAlive string          `json:"keep_alive,omitempty"` // Duration to keep model loaded
}

// EmbeddingResponse represents a response from the /api/embed endpoint
type EmbeddingResponse struct {
	Model           string      `json:"model"`
	Embeddings      [][]float32 `json:"embeddings"` // Array of embedding vectors
	TotalDuration   int64       `json:"total_duration,omitempty"`
	LoadDuration    int64       `json:"load_duration,omitempty"`
	PromptEvalCount int         `json:"prompt_eval_count,omitempty"`
	Error           string      `json:"error,omitempty"`
}

// SingleEmbeddingResponse represents a response from the legacy /api/embeddings endpoint
type SingleEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"` // Single embedding vector
	Error     string    `json:"error,omitempty"`
}

// ModelInfo represents information about an available model
type ModelInfo struct {
	Name       string       `json:"name"`
	ModifiedAt time.Time    `json:"modified_at"`
	Size       int64        `json:"size"`
	Digest     string       `json:"digest"`
	Details    ModelDetails `json:"details"`
}

// ModelDetails contains information about a model
type ModelDetails struct {
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families,omitempty"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// ListModelsResponse represents the response from the /api/tags endpoint
type ListModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

// RunningModelInfo represents information about a model currently loaded in memory
type RunningModelInfo struct {
	Name      string       `json:"name"`
	Model     string       `json:"model"`
	Size      int64        `json:"size"`
	SizeVRAM  int64        `json:"size_vram"`
	Digest    string       `json:"digest"`
	ExpiresAt time.Time    `json:"expires_at"`
	Details   ModelDetails `json:"details"`
}

// ListRunningModelsResponse represents the response from the /api/ps endpoint
type ListRunningModelsResponse struct {
	Models []RunningModelInfo `json:"models"`
}

// VersionResponse represents the response from the /api/version endpoint
type VersionResponse struct {
	Version string `json:"version"`
}

// RequestOptions contains optional parameters for generation requests
type RequestOptions struct {
	// Temperature controls randomness in generation (0.0 to 1.0)
	Temperature *float64 `json:"temperature,omitempty"`

	// TopP controls diversity through nucleus sampling (0.0 to 1.0)
	TopP *float64 `json:"top_p,omitempty"`

	// TopK controls vocabulary size in sampling
	TopK *int `json:"top_k,omitempty"`

	// MinP is the minimum probability threshold for sampling
	MinP *float64 `json:"min_p,omitempty"`

	// TypicalP controls "typical" sampling
	TypicalP *float64 `json:"typical_p,omitempty"`

	// RepeatPenalty penalizes repetitions (1.0 = no penalty)
	RepeatPenalty *float64 `json:"repeat_penalty,omitempty"`

	// RepeatLastN is the number of tokens to look back for repetitions
	RepeatLastN *int `json:"repeat_last_n,omitempty"`

	// PresencePenalty penalizes tokens based on presence in text
	PresencePenalty *float64 `json:"presence_penalty,omitempty"`

	// FrequencyPenalty penalizes tokens based on frequency in text
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`

	// Mirostat mode (0 = disabled, 1 = mirostat, 2 = mirostat 2.0)
	Mirostat *int `json:"mirostat,omitempty"`

	// MirostatTau is the target entropy for mirostat
	MirostatTau *float64 `json:"mirostat_tau,omitempty"`

	// MirostatEta is the learning rate for mirostat
	MirostatEta *float64 `json:"mirostat_eta,omitempty"`

	// PenalizeNewline, when true, penalizes newline tokens
	PenalizeNewline *bool `json:"penalize_newline,omitempty"`

	// Seed for deterministic sampling
	Seed *int `json:"seed,omitempty"`

	// NumPredict is the maximum number of tokens to generate
	NumPredict *int `json:"num_predict,omitempty"`

	// NumKeep is the number of tokens to keep from initial prompt
	NumKeep *int `json:"num_keep,omitempty"`

	// NumCtx is the size of the context window
	NumCtx *int `json:"num_ctx,omitempty"`

	// NumBatch is the batch size for processing prompts
	NumBatch *int `json:"num_batch,omitempty"`

	// NumGPU is the number of GPUs to use
	NumGPU *int `json:"num_gpu,omitempty"`

	// MainGPU is the main GPU to use
	MainGPU *int `json:"main_gpu,omitempty"`

	// LowVRAM, when true, reduces VRAM usage at cost of performance
	LowVRAM *bool `json:"low_vram,omitempty"`

	// Stop sequences that trigger end of generation
	Stop []string `json:"stop,omitempty"`

	// NUMA, when true, enables NUMA support
	NUMA *bool `json:"numa,omitempty"`

	// UseMLock, when true, uses mlock to keep model in memory
	UseMLock *bool `json:"use_mlock,omitempty"`

	// UseMMap, when true, uses memory mapping for model loading
	UseMMap *bool `json:"use_mmap,omitempty"`

	// VocabOnly, when true, loads only vocabulary and not weights
	VocabOnly *bool `json:"vocab_only,omitempty"`

	// NumThread is the number of threads to use
	NumThread *int `json:"num_thread,omitempty"`
}

// NewDefaultOptions creates RequestOptions with standard defaults
func NewDefaultOptions() *RequestOptions {
	temp := 0.7
	topP := 0.9
	return &RequestOptions{
		Temperature: &temp,
		TopP:        &topP,
	}
}

// NewChatOptions creates RequestOptions optimized for chat
func NewChatOptions() *RequestOptions {
	opts := NewDefaultOptions()
	opts.Stop = []string{"</s>", "user:", "assistant:"}
	return opts
}

// Float64Ptr creates a float64 pointer from a value
func Float64Ptr(v float64) *float64 {
	return &v
}

// IntPtr creates an int pointer from a value
func IntPtr(v int) *int {
	return &v
}

// BoolPtr creates a bool pointer from a value
func BoolPtr(v bool) *bool {
	return &v
}
