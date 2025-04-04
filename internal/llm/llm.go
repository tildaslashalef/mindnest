package llm

import (
	"context"
	"fmt"

	"github.com/tildaslashalef/mindnest/internal/claude"
	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/ollama"
)

// ChatRequest represents a generic chat request to any LLM
type ChatRequest struct {
	Model         string                 `json:"model"`
	Messages      []Message              `json:"messages"`
	MaxTokens     int                    `json:"max_tokens,omitempty"`
	Temperature   float64                `json:"temperature,omitempty"`
	Stream        bool                   `json:"stream,omitempty"`
	Options       map[string]interface{} `json:"options,omitempty"`
	FormatOptions *FormatOptions         `json:"format_options,omitempty"`
}

// Message represents a chat message with role and content
type Message struct {
	Role    string `json:"role"` // user, assistant, or system
	Content string `json:"content"`
}

// ChatResponse represents a response from a chat request
type ChatResponse struct {
	Content   string `json:"content"`
	Model     string `json:"model"`
	Completed bool   `json:"completed"`
	Error     string `json:"error,omitempty"`
}

// EmbeddingRequest represents a request for generating embeddings
type EmbeddingRequest struct {
	Model string `json:"model"`
	Text  string `json:"text"`
}

// FormatOptions represents the structured output format options
type FormatOptions struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

// GenerateRequest represents a request for text generation
type GenerateRequest struct {
	Model         string                 `json:"model"`
	Prompt        string                 `json:"prompt"`
	System        string                 `json:"system,omitempty"`
	MaxTokens     int                    `json:"max_tokens,omitempty"`
	Temperature   float64                `json:"temperature,omitempty"`
	Stream        bool                   `json:"stream,omitempty"`
	Options       map[string]interface{} `json:"options,omitempty"`
	FormatOptions *FormatOptions         `json:"format_options,omitempty"`
}

// GenerateResponse represents a response from a text generation request
type GenerateResponse struct {
	Content   string `json:"content"`
	Model     string `json:"model"`
	Completed bool   `json:"completed"`
	Error     string `json:"error,omitempty"`
}

// Client defines the interface for LLM clients
type Client interface {
	// GenerateChat sends a non-streaming chat request
	GenerateChat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// GenerateChatStream sends a streaming chat request
	GenerateChatStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error)

	// GenerateCompletion sends a non-streaming completion request
	GenerateCompletion(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)

	// GenerateEmbedding generates an embedding for text
	GenerateEmbedding(ctx context.Context, req EmbeddingRequest) ([]float32, error)

	// BatchEmbeddings generates embeddings in batch
	BatchEmbeddings(ctx context.Context, reqs []EmbeddingRequest) ([][]float32, error)
}

// ClientType defines the type of LLM client
type ClientType string

const (
	// Ollama client type
	Ollama ClientType = "ollama"

	// Claude client type
	Claude ClientType = "claude"
)

// Factory creates and returns LLM clients
type Factory struct {
	config *config.Config
	ollama *ollama.Client
	claude *claude.Client
}

// NewFactory creates a new LLM client factory
func NewFactory(cfg *config.Config) *Factory {
	f := &Factory{
		config: cfg,
	}

	// Initialize Ollama client if configured
	if cfg.Ollama.Endpoint != "" {
		f.ollama = ollama.NewClient(cfg.Ollama)
		loggy.Info("initialized Ollama client", "endpoint", cfg.Ollama.Endpoint)
	}

	// Initialize Claude client if configured
	if cfg.Claude.APIKey != "" {
		// Create top-p, top-k, and temperature pointers as needed
		var topP, temperature *float64
		var topK *int

		if cfg.Claude.TopP > 0 {
			topP = &cfg.Claude.TopP
		}

		if cfg.Claude.TopK > 0 {
			topK = &cfg.Claude.TopK
		}

		if cfg.Claude.Temperature > 0 {
			temperature = &cfg.Claude.Temperature
		}

		// Filter out empty API beta strings
		var apiBeta []string
		for _, beta := range cfg.Claude.APIBeta {
			if beta != "" {
				apiBeta = append(apiBeta, beta)
			}
		}

		f.claude = claude.NewClient(claude.Config{
			APIKey:           cfg.Claude.APIKey,
			BaseURL:          cfg.Claude.BaseURL,
			DefaultModel:     cfg.Claude.Model,
			Timeout:          cfg.Claude.Timeout,
			MaxRetries:       cfg.Claude.MaxRetries,
			DefaultMaxTokens: cfg.Claude.MaxTokens,
			APIVersion:       cfg.Claude.APIVersion,
			APIBeta:          apiBeta,
			TopP:             topP,
			TopK:             topK,
			Temperature:      temperature,
		})
		loggy.Info("initialized Claude client", "base_url", cfg.Claude.BaseURL, "model", cfg.Claude.Model)
	}

	return f
}

// GetClient returns an LLM client of the specified type
func (f *Factory) GetClient(clientType ClientType) (Client, error) {
	switch clientType {
	case Ollama:
		if f.ollama == nil {
			return nil, fmt.Errorf("Ollama client not initialized - check configuration")
		}
		return newOllamaClientAdapter(f.ollama), nil

	case Claude:
		if f.claude == nil {
			return nil, fmt.Errorf("Claude client not initialized - check configuration")
		}
		// If Ollama is also configured, use it for embeddings
		if f.ollama != nil {
			return newClaudeClientAdapterWithOllama(f.claude, f.ollama), nil
		}
		return newClaudeClientAdapter(f.claude), nil

	default:
		return nil, fmt.Errorf("unknown client type: %s", clientType)
	}
}

// GetDefaultClient returns the default client based on configuration
func (f *Factory) GetDefaultClient() (Client, ClientType, error) {
	defaultType := f.config.LLM.DefaultProvider

	client, err := f.GetClient(ClientType(defaultType))
	if err != nil {
		// Fallback to first available client
		if f.ollama != nil {
			return newOllamaClientAdapter(f.ollama), Ollama, nil
		}
		if f.claude != nil {
			// If Ollama is also available, use it for embeddings
			if f.ollama != nil {
				return newClaudeClientAdapterWithOllama(f.claude, f.ollama), Claude, nil
			}
			return newClaudeClientAdapter(f.claude), Claude, nil
		}
		return nil, "", fmt.Errorf("no LLM clients initialized - check configuration")
	}

	return client, ClientType(defaultType), nil
}
