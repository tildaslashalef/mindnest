package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/tildaslashalef/mindnest/internal/claude"
	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/ollama"
)

// claudeClientAdapter adapts the Claude client to the LLM Client interface
type claudeClientAdapter struct {
	client *claude.Client
	ollama *ollama.Client // Added for embedding support
	config *config.Config
}

// newClaudeClientAdapter creates a new Claude client adapter
func newClaudeClientAdapter(client *claude.Client, cfg *config.Config) *claudeClientAdapter {
	return &claudeClientAdapter{
		client: client,
		config: cfg,
	}
}

// newClaudeClientAdapterWithOllama creates a new Claude client adapter with Ollama for embeddings
func newClaudeClientAdapterWithOllama(client *claude.Client, ollamaClient *ollama.Client, cfg *config.Config) *claudeClientAdapter {
	return &claudeClientAdapter{
		client: client,
		ollama: ollamaClient,
		config: cfg,
	}
}

// GenerateChat implements the Client interface for Claude
func (a *claudeClientAdapter) GenerateChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Extract any system message from messages array
	var claudeMessages []claude.Message
	var systemMessage string

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemMessage = msg.Content
		} else {
			claudeMessages = append(claudeMessages, claude.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Convert to Claude request
	claudeReq := claude.ChatRequest{
		Model:    req.Model,
		Stream:   false,
		Messages: claudeMessages,
		System:   systemMessage, // Use extracted system message
	}

	// Set options if provided
	if req.MaxTokens > 0 {
		claudeReq.MaxTokens = req.MaxTokens
	}
	if req.Temperature > 0 {
		temp := req.Temperature
		claudeReq.Temperature = &temp
	}

	// Handle additional options if provided
	if req.Options != nil {
		if val, ok := req.Options["top_p"].(float64); ok {
			claudeReq.TopP = &val
		}
		if val, ok := req.Options["top_k"].(int); ok {
			claudeReq.TopK = &val
		}
		if val, ok := req.Options["stop_sequences"].([]string); ok {
			claudeReq.StopSequences = val
		}
	}

	// Make the request
	resp, err := a.client.GenerateChat(ctx, claudeReq)
	if err != nil {
		return nil, fmt.Errorf("claude chat generation failed: %w", err)
	}

	// Extract text content from blocks
	content := extractClaudeContent(resp.Content)
	if content == "" && resp.Message.Content != "" {
		content = resp.Message.Content
	}

	// Convert to ChatResponse
	return &ChatResponse{
		Content:   content,
		Model:     resp.Model,
		Completed: resp.Done,
		Error:     resp.ErrorMsg,
	}, nil
}

// GenerateChatStream implements the Client interface for Claude
func (a *claudeClientAdapter) GenerateChatStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	// Extract any system message from messages array
	var claudeMessages []claude.Message
	var systemMessage string

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemMessage = msg.Content
		} else {
			claudeMessages = append(claudeMessages, claude.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Convert to Claude request
	claudeReq := claude.ChatRequest{
		Model:    req.Model,
		Stream:   true,
		Messages: claudeMessages,
		System:   systemMessage, // Use extracted system message
	}

	// Set options if provided
	if req.MaxTokens > 0 {
		claudeReq.MaxTokens = req.MaxTokens
	}
	if req.Temperature > 0 {
		temp := req.Temperature
		claudeReq.Temperature = &temp
	}

	// Handle additional options if provided
	if req.Options != nil {
		if val, ok := req.Options["top_p"].(float64); ok {
			claudeReq.TopP = &val
		}
		if val, ok := req.Options["top_k"].(int); ok {
			claudeReq.TopK = &val
		}
		if val, ok := req.Options["stop_sequences"].([]string); ok {
			claudeReq.StopSequences = val
		}
	}

	// Get the streaming channel from Claude
	claudeRespChan, err := a.client.GenerateChatStream(ctx, claudeReq)
	if err != nil {
		return nil, fmt.Errorf("claude stream generation failed: %w", err)
	}

	// Create output channel
	responseChan := make(chan ChatResponse)

	// Process stream in goroutine
	go func() {
		defer close(responseChan)

		for claudeResp := range claudeRespChan {
			// Extract text from content blocks
			content := extractClaudeContent(claudeResp.Content)

			// Fallback to Message.Content if needed
			if content == "" && claudeResp.Message.Content != "" {
				content = claudeResp.Message.Content
			}

			responseChan <- ChatResponse{
				Content:   content,
				Model:     claudeResp.Model,
				Completed: claudeResp.Done,
				Error:     claudeResp.ErrorMsg,
			}
		}
	}()

	return responseChan, nil
}

// GenerateEmbedding implements the Client interface for Claude
func (a *claudeClientAdapter) GenerateEmbedding(ctx context.Context, req EmbeddingRequest) ([]float32, error) {
	// If Ollama client is available, use it for embeddings
	if a.ollama != nil {
		ollamaReq := ollama.EmbeddingRequest{
			// Always use the embedding model from config
			Model: a.config.Claude.EmbeddingModel,
			Input: req.Text,
		}

		resp, err := a.ollama.GenerateEmbedding(ctx, ollamaReq)
		if err != nil {
			// Try legacy endpoint as fallback
			legacyReq := ollama.SingleEmbeddingRequest{
				Model:  a.config.Claude.EmbeddingModel,
				Prompt: req.Text,
			}

			legacyResp, legacyErr := a.ollama.GenerateSingleEmbedding(ctx, legacyReq)
			if legacyErr != nil {
				return nil, fmt.Errorf("ollama embedding generation failed: %w", err)
			}

			return legacyResp.Embedding, nil
		}

		// For the new /api/embed endpoint, we expect an array of embeddings
		if len(resp.Embeddings) > 0 {
			return resp.Embeddings[0], nil
		}

		return nil, fmt.Errorf("empty embedding response")
	}

	// If Ollama client is not available, return error
	return nil, fmt.Errorf("claude does not support embeddings and no ollama client is configured")
}

// BatchEmbeddings implements the Client interface for Claude
func (a *claudeClientAdapter) BatchEmbeddings(ctx context.Context, reqs []EmbeddingRequest) ([][]float32, error) {
	// If Ollama client is available, use it for embeddings
	if a.ollama != nil {
		ollamaReqs := make([]ollama.EmbeddingRequest, len(reqs))
		for i, req := range reqs {
			ollamaReqs[i] = ollama.EmbeddingRequest{
				// Always use the embedding model from config
				Model: a.config.Claude.EmbeddingModel,
				Input: req.Text,
			}
		}

		resps, err := a.ollama.BatchEmbeddings(ctx, ollamaReqs)
		if err != nil {
			return nil, fmt.Errorf("ollama batch embedding generation failed: %w", err)
		}

		embeddings := make([][]float32, len(resps))
		for i, resp := range resps {
			if len(resp.Embeddings) > 0 {
				embeddings[i] = resp.Embeddings[0]
			} else {
				embeddings[i] = []float32{}
			}
		}

		return embeddings, nil
	}

	// If Ollama client is not available, return error
	return nil, fmt.Errorf("claude does not support embeddings and no ollama client is configured")
}

// GenerateCompletion implements the Client interface for Claude
func (a *claudeClientAdapter) GenerateCompletion(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Claude doesn't have a direct equivalent to the generate endpoint
	// We could implement this by using the chat API, but for now we'll just return an error
	return nil, fmt.Errorf("claude does not support the generate endpoint directly, use GenerateChat instead")
}

// Helper to convert message format
func convertMessagesToClaude(messages []Message) []claude.Message {
	var claudeMessages []claude.Message

	// Filter out system messages as they're handled differently in Claude
	for _, msg := range messages {
		if msg.Role != "system" {
			claudeMessages = append(claudeMessages, claude.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	return claudeMessages
}

// extractClaudeContent extracts text from Claude content blocks
func extractClaudeContent(blocks []claude.ContentBlock) string {
	var content strings.Builder
	for _, block := range blocks {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}
	return content.String()
}
