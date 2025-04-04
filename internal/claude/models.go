package claude

import (
	"fmt"
)

// Message represents a chat message
type Message struct {
	Role    string `json:"role"` // user, assistant, or system
	Content string `json:"content"`
}

// ChatRequest represents a chat completion request to Claude API
type ChatRequest struct {
	Model         string      `json:"model"`                    // Claude model to use (e.g., "claude-3-opus-20240229")
	Messages      []Message   `json:"messages"`                 // Chat history messages
	System        string      `json:"system,omitempty"`         // System instructions
	MaxTokens     int         `json:"max_tokens,omitempty"`     // Maximum tokens to generate
	Temperature   *float64    `json:"temperature,omitempty"`    // Controls randomness
	TopP          *float64    `json:"top_p,omitempty"`          // Nucleus sampling parameter
	TopK          *int        `json:"top_k,omitempty"`          // Top-k sampling parameter
	Stream        bool        `json:"stream,omitempty"`         // Whether to stream the response
	StopSequences []string    `json:"stop_sequences,omitempty"` // Sequences that cause generation to stop
	Tools         []Tool      `json:"tools,omitempty"`          // Tools available for the model to use
	ToolChoice    *ToolChoice `json:"tool_choice,omitempty"`    // Force the model to use a specific tool
}

// Tool represents a tool that Claude can use
type Tool struct {
	Type            string      `json:"type"`                        // Tool type (e.g., "custom", "computer_20250212")
	Name            string      `json:"name"`                        // Tool name
	Description     string      `json:"description,omitempty"`       // Tool description (for custom tools)
	InputSchema     interface{} `json:"input_schema,omitempty"`      // JSON schema for input (for custom tools)
	DisplayHeightPx *int        `json:"display_height_px,omitempty"` // For computer tool
	DisplayWidthPx  *int        `json:"display_width_px,omitempty"`  // For computer tool
	DisplayNumber   *int        `json:"display_number,omitempty"`    // For computer tool
}

// ToolChoice represents a directive to force Claude to use a specific tool
type ToolChoice struct {
	Type string `json:"type"` // Tool type
	Name string `json:"name"` // Tool name
}

// ContentBlock represents a block of content in a response
// Claude responses can contain multiple content blocks of different types
type ContentBlock struct {
	Type string `json:"type"` // Content type (e.g., "text", "thinking")
	Text string `json:"text"` // The actual content text
}

// ChatResponse represents a response from the chat completion endpoint
type ChatResponse struct {
	ID         string         `json:"id,omitempty"`          // Response ID
	Model      string         `json:"model,omitempty"`       // Model used
	Created    int64          `json:"created,omitempty"`     // Creation timestamp
	Message    Message        `json:"message,omitempty"`     // Response message
	Done       bool           `json:"done,omitempty"`        // Indicates completion
	ErrorMsg   string         `json:"error,omitempty"`       // Error message if any
	Usage      *UsageInfo     `json:"usage,omitempty"`       // Token usage information
	Content    []ContentBlock `json:"content,omitempty"`     // Response content blocks
	StopReason string         `json:"stop_reason,omitempty"` // Reason why generation stopped
}

// MessageResponse represents the full message response from Claude API
type MessageResponse struct {
	ID      string         `json:"id"`      // Message ID
	Type    string         `json:"type"`    // Message type
	Role    string         `json:"role"`    // Message role (e.g., "assistant")
	Content []ContentBlock `json:"content"` // Message content blocks
	Model   string         `json:"model"`   // Model used
	Created int64          `json:"created"` // Creation timestamp
}

// MessageStreamResponse represents a streamed message response
// Claude streams responses in chunks, with different event types
type MessageStreamResponse struct {
	Type    string `json:"type"` // Event type (e.g., "content_block_start", "content_block_delta", "message_delta")
	Message struct {
		ID      string         `json:"id"`      // Message ID
		Type    string         `json:"type"`    // Message type
		Role    string         `json:"role"`    // Message role
		Content []ContentBlock `json:"content"` // Message content
		Model   string         `json:"model"`   // Model used
	} `json:"message"`
	Delta struct {
		Type       string `json:"type"`        // Delta type
		Text       string `json:"text"`        // Text content delta
		StopReason string `json:"stop_reason"` // Reason for stopping
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"` // Output token count
	} `json:"usage"`
}

// EmbeddingRequest represents a request for generating embeddings
// Note: Claude doesn't natively support embeddings, this is for interface compatibility
type EmbeddingRequest struct {
	Model  string `json:"model"`  // Model name (not used for Claude)
	Prompt string `json:"prompt"` // Text to embed (not used for Claude)
}

// EmbeddingResponse represents the response from an embedding request
// Note: Claude doesn't natively support embeddings, this is for interface compatibility
type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`       // Empty for Claude
	ErrorMsg  string    `json:"error,omitempty"` // Error message
}

// UsageInfo contains token usage information for a request
type UsageInfo struct {
	InputTokens  int `json:"input_tokens"`  // Number of input tokens
	OutputTokens int `json:"output_tokens"` // Number of output tokens
}

// APIError represents an error response from the Claude API
type APIError struct {
	Type         string `json:"type"`
	ErrorDetails struct {
		Type    string `json:"type"`    // Error type
		Message string `json:"message"` // Error message
	} `json:"error"`
}

// Error implements the error interface for APIError
func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorDetails.Type, e.ErrorDetails.Message)
}

// RequestOptions contains optional parameters for generation
type RequestOptions struct {
	Temperature *float64 `json:"temperature,omitempty"` // Controls randomness (0.0 to 1.0)
	MaxTokens   *int     `json:"max_tokens,omitempty"`  // Max tokens to generate
	TopP        *float64 `json:"top_p,omitempty"`       // Controls diversity (0.0 to 1.0)
	TopK        *int     `json:"top_k,omitempty"`       // Top-k sampling parameter
	Stop        []string `json:"stop,omitempty"`        // Stop sequences
}

// DefaultOptions returns RequestOptions with good default values for Claude
func DefaultOptions() *RequestOptions {
	temp := 0.7
	maxTokens := 4096
	return &RequestOptions{
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}
}

// ChatOptions returns RequestOptions optimized for chat interactions with Claude
func ChatOptions() *RequestOptions {
	opts := DefaultOptions()
	return opts
}

// Float64Ptr creates a float64 pointer from a value
// This is a helper function for creating option values
func Float64Ptr(v float64) *float64 {
	return &v
}

// IntPtr creates an int pointer from a value
// This is a helper function for creating option values
func IntPtr(v int) *int {
	return &v
}
