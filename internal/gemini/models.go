package gemini

// Message represents a chat message
type Message struct {
	Role    string        `json:"role"` // user, model, or system
	Content string        `json:"content"`
	Parts   []MessagePart `json:"parts,omitempty"`
}

// MessagePart represents a part of a message content
type MessagePart struct {
	Text string `json:"text,omitempty"`
}

// ChatRequest represents a request to the Gemini API for chat completion
type ChatRequest struct {
	Model       string    `json:"model,omitempty"`
	Contents    []Content `json:"contents"`
	MaxTokens   int       `json:"maxOutputTokens,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	TopP        *float64  `json:"topP,omitempty"`
	TopK        *int      `json:"topK,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Content represents content in a chat message
type Content struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts"`
}

// Part represents a part of content in a chat message
type Part struct {
	Text string `json:"text,omitempty"`
}

// Tool represents a tool that Gemini can use
type Tool struct {
	FunctionDeclarations []FunctionDeclaration `json:"function_declarations,omitempty"`
}

// FunctionDeclaration represents a function declaration for Gemini
type FunctionDeclaration struct {
	Name        string      `json:"name"`        // Function name
	Description string      `json:"description"` // Function description
	Parameters  interface{} `json:"parameters"`  // Function parameters schema
}

// ContentPart represents a part of content in a response
type ContentPart struct {
	Text         string        `json:"text,omitempty"`
	FunctionCall *FunctionCall `json:"function_call,omitempty"`
}

// FunctionCall represents a function call in a response
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatResponse represents a response from the Gemini API for chat completion
type ChatResponse struct {
	Candidates []Candidate `json:"candidates,omitempty"`
	ErrorMsg   string      `json:"-"` // For client-side errors that don't come from the API
}

// Candidate represents a candidate response from the Gemini API
type Candidate struct {
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason,omitempty"`
}

// SafetyRating represents a safety rating from Gemini
type SafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

// PromptFeedback represents feedback on the prompt from Gemini
type PromptFeedback struct {
	SafetyRatings []SafetyRating `json:"safetyRatings,omitempty"`
}

// StreamResponseChunk represents a chunk of a streaming response
type StreamResponseChunk struct {
	Candidates  []Candidate   `json:"candidates,omitempty"`
	ErrorDetail *ErrorDetails `json:"error,omitempty"`
}

// EmbeddingRequest represents a request to generate embeddings
type EmbeddingRequest struct {
	Model string `json:"-"` // Not part of the JSON, used internally
	Text  string `json:"-"` // Not part of the JSON, used internally
}

// EmbeddingResponse represents a response containing embeddings
type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding,omitempty"`
}

// UsageInfo contains token usage information for a request
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`     // Number of input tokens
	CompletionTokens int `json:"completion_tokens"` // Number of output tokens
	TotalTokens      int `json:"total_tokens"`      // Total tokens used
}

// APIError represents an error returned by the Gemini API
type APIError struct {
	ErrorDetail *ErrorDetails `json:"error,omitempty"`
}

// ErrorDetails contains details about an API error
type ErrorDetails struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Status  string `json:"status,omitempty"`
}

// Error implements the error interface for APIError
func (e *APIError) Error() string {
	if e.ErrorDetail != nil {
		return e.ErrorDetail.Message
	}
	return "unknown API error"
}

// RequestOptions contains optional parameters for generation
type RequestOptions struct {
	Temperature *float64 `json:"temperature,omitempty"`       // Controls randomness (0.0 to 1.0)
	MaxTokens   *int     `json:"max_output_tokens,omitempty"` // Max tokens to generate
	TopP        *float64 `json:"top_p,omitempty"`             // Controls diversity (0.0 to 1.0)
	TopK        *int     `json:"top_k,omitempty"`             // Top-k sampling parameter
}

// DefaultOptions returns RequestOptions with good default values for Gemini
func DefaultOptions() *RequestOptions {
	temp := 0.7
	maxTokens := 4096
	return &RequestOptions{
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}
}

// Float64Ptr creates a float64 pointer from a value
func Float64Ptr(v float64) *float64 {
	return &v
}

// IntPtr creates an int pointer from a value
func IntPtr(v int) *int {
	return &v
}
