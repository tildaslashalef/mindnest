package llm

import (
	"context"
	"fmt"

	"golang.org/x/time/rate" // Import the rate limiting package

	"github.com/tildaslashalef/mindnest/internal/claude"
	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/gemini"
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

	// Gemini client type
	Gemini ClientType = "gemini"
)

// Factory creates and returns LLM clients
type Factory struct {
	config *config.Config
	ollama *ollama.Client
	claude *claude.Client
	gemini *gemini.Client
	logger *loggy.Logger

	// Add rate limiters
	ollamaLimiter *rate.Limiter
	claudeLimiter *rate.Limiter
	geminiLimiter *rate.Limiter
}

// helper function to create a rate limiter from RPM and Burst
func newLimiter(rpm, burst int) *rate.Limiter {
	if rpm <= 0 {
		// If RPM is zero or negative, allow infinite rate (no limiting)
		return rate.NewLimiter(rate.Inf, burst)
	}
	// Calculate rate per second
	r := rate.Limit(float64(rpm) / 60.0)
	// Burst should be at least 1
	b := burst
	if b <= 0 {
		b = 1
	}
	return rate.NewLimiter(r, b)
}

// NewFactory creates a new LLM client factory
func NewFactory(cfg *config.Config, logger *loggy.Logger) *Factory {
	f := &Factory{
		config: cfg,
		logger: logger,
	}

	// Initialize Ollama client and limiter if configured
	if cfg.Ollama.Endpoint != "" {
		f.ollama = ollama.NewClient(cfg.Ollama)
		f.ollamaLimiter = newLimiter(cfg.Ollama.RequestsPerMinute, cfg.Ollama.BurstLimit)
		loggy.Info("initialized Ollama client", "endpoint", cfg.Ollama.Endpoint, "rpm", cfg.Ollama.RequestsPerMinute, "burst", cfg.Ollama.BurstLimit)
	}

	// Initialize Claude client and limiter if configured
	if cfg.Claude.APIKey != "" {
		f.claude = claude.NewClient(cfg.Claude)
		f.claudeLimiter = newLimiter(cfg.Claude.RequestsPerMinute, cfg.Claude.BurstLimit)
		loggy.Info("initialized Claude client", "base_url", cfg.Claude.BaseURL, "model", cfg.Claude.Model, "rpm", cfg.Claude.RequestsPerMinute, "burst", cfg.Claude.BurstLimit)
	}

	// Initialize Gemini client and limiter if configured
	if cfg.Gemini.APIKey != "" {
		f.gemini = gemini.NewClient(cfg.Gemini)
		f.geminiLimiter = newLimiter(cfg.Gemini.RequestsPerMinute, cfg.Gemini.BurstLimit)
		loggy.Info("initialized Gemini client",
			"base_url", cfg.Gemini.BaseURL,
			"model", cfg.Gemini.Model,
			"rpm", cfg.Gemini.RequestsPerMinute,
			"burst", cfg.Gemini.BurstLimit)
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
		// Pass limiter to adapter
		return newOllamaClientAdapter(f.ollama, f.config, f.ollamaLimiter), nil

	case Claude:
		if f.claude == nil {
			return nil, fmt.Errorf("Claude client not initialized - check configuration")
		}
		// Pass limiter to adapter
		if f.ollama != nil && f.config.Claude.EmbeddingModel == "ollama" {
			// Pass both limiters if using Ollama for embeddings
			return newClaudeClientAdapterWithOllama(f.claude, f.ollama, f.config, f.claudeLimiter, f.ollamaLimiter), nil
		}
		return newClaudeClientAdapter(f.claude, f.config, f.claudeLimiter), nil

	case Gemini:
		if f.gemini == nil {
			return nil, fmt.Errorf("Gemini client not initialized - check configuration")
		}
		// Pass limiter to adapter
		if f.ollama != nil && f.config.Gemini.EmbeddingModel == "ollama" {
			// Pass both limiters if using Ollama for embeddings
			return newGeminiClientAdapterWithOllama(f.gemini, f.ollama, f.config, f.geminiLimiter, f.ollamaLimiter), nil
		}
		return newGeminiClientAdapter(f.gemini, f.config, f.geminiLimiter), nil

	default:
		return nil, fmt.Errorf("unknown client type: %s", clientType)
	}
}

// GetDefaultClient returns the default client based on configuration
func (f *Factory) GetDefaultClient() (Client, ClientType, error) {
	defaultType := f.config.DefaultLLMProvider

	// Try getting the default client first
	client, err := f.GetClient(ClientType(defaultType))
	if err == nil {
		return client, ClientType(defaultType), nil
	}

	// Fallback to first available client
	f.logger.Warn("Default LLM provider not available, falling back", "default", defaultType, "error", err)

	if f.gemini != nil {
		clientType := Gemini
		var client Client
		if f.ollama != nil && f.config.Gemini.EmbeddingModel == "ollama" {
			client = newGeminiClientAdapterWithOllama(f.gemini, f.ollama, f.config, f.geminiLimiter, f.ollamaLimiter)
		} else {
			client = newGeminiClientAdapter(f.gemini, f.config, f.geminiLimiter)
		}
		return client, clientType, nil
	}
	if f.claude != nil {
		clientType := Claude
		var client Client
		if f.ollama != nil && f.config.Claude.EmbeddingModel == "ollama" {
			client = newClaudeClientAdapterWithOllama(f.claude, f.ollama, f.config, f.claudeLimiter, f.ollamaLimiter)
		} else {
			client = newClaudeClientAdapter(f.claude, f.config, f.claudeLimiter)
		}
		return client, clientType, nil
	}
	if f.ollama != nil {
		clientType := Ollama
		client := newOllamaClientAdapter(f.ollama, f.config, f.ollamaLimiter)
		return client, clientType, nil
	}
	return nil, "", fmt.Errorf("no LLM clients initialized - check configuration")
}

// GetDefaultProviderConfig returns the configuration for the default provider
func (f *Factory) GetDefaultProviderConfig() (interface{}, ClientType) {
	providerType := ClientType(f.config.DefaultLLMProvider)

	switch providerType {
	case Ollama:
		return f.config.Ollama, providerType
	case Claude:
		return f.config.Claude, providerType
	case Gemini:
		return f.config.Gemini, providerType
	default:
		// Fallback to first available provider
		if f.gemini != nil {
			return f.config.Gemini, Gemini
		}
		if f.claude != nil {
			return f.config.Claude, Claude
		}
		if f.ollama != nil {
			return f.config.Ollama, Ollama
		}
		return nil, ""
	}
}

// GenerateChat generates a chat response from the default LLM provider
func (f *Factory) GenerateChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	f.logger.Debug("Generating chat using default provider")

	defaultType := f.config.DefaultLLMProvider

	client, err := f.GetClient(ClientType(defaultType))
	if err != nil {
		return nil, fmt.Errorf("failed to get client for default provider %s: %w", defaultType, err)
	}

	return client.GenerateChat(ctx, req)
}

// GenerateChatStream generates a streaming chat response from the default LLM provider
func (f *Factory) GenerateChatStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	f.logger.Debug("Generating streaming chat using default provider")

	defaultType := f.config.DefaultLLMProvider

	client, err := f.GetClient(ClientType(defaultType))
	if err != nil {
		return nil, fmt.Errorf("failed to get client for default provider %s: %w", defaultType, err)
	}

	return client.GenerateChatStream(ctx, req)
}

// GenerateCompletion generates a completion response from the default LLM provider
func (f *Factory) GenerateCompletion(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	f.logger.Debug("Generating completion using default provider")

	defaultType := f.config.DefaultLLMProvider

	client, err := f.GetClient(ClientType(defaultType))
	if err != nil {
		return nil, fmt.Errorf("failed to get client for default provider %s: %w", defaultType, err)
	}

	return client.GenerateCompletion(ctx, req)
}

// GenerateEmbedding generates an embedding from the default LLM provider
func (f *Factory) GenerateEmbedding(ctx context.Context, req EmbeddingRequest) ([]float32, error) {
	f.logger.Debug("Generating embedding using default provider")

	defaultType := f.config.DefaultLLMProvider

	client, err := f.GetClient(ClientType(defaultType))
	if err != nil {
		return nil, fmt.Errorf("failed to get client for default provider %s: %w", defaultType, err)
	}

	return client.GenerateEmbedding(ctx, req)
}

// BatchEmbeddings generates multiple embeddings from the default LLM provider
func (f *Factory) BatchEmbeddings(ctx context.Context, reqs []EmbeddingRequest) ([][]float32, error) {
	f.logger.Debug("Generating batch embeddings using default provider")

	defaultType := f.config.DefaultLLMProvider

	client, err := f.GetClient(ClientType(defaultType))
	if err != nil {
		return nil, fmt.Errorf("failed to get client for default provider %s: %w", defaultType, err)
	}

	return client.BatchEmbeddings(ctx, reqs)
}
