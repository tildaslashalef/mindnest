package llm

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/time/rate" // Import rate limiter

	"github.com/tildaslashalef/mindnest/internal/claude"
	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/ollama"
)

// claudeClientAdapter adapts the Claude client to the LLM Client interface
type claudeClientAdapter struct {
	client        *claude.Client
	ollama        *ollama.Client // Optional Ollama client for embeddings
	config        *config.Config
	limiter       *rate.Limiter // Added rate limiter for Claude API
	ollamaLimiter *rate.Limiter // Added optional rate limiter for Ollama (when used for embeddings)
}

// newClaudeClientAdapter creates a new Claude client adapter
// Updated to accept limiter
func newClaudeClientAdapter(client *claude.Client, cfg *config.Config, limiter *rate.Limiter) *claudeClientAdapter {
	return &claudeClientAdapter{
		client:  client,
		config:  cfg,
		limiter: limiter, // Store Claude limiter
	}
}

// newClaudeClientAdapterWithOllama creates a new Claude client adapter with Ollama for embeddings
// Updated to accept both Claude and Ollama limiters
func newClaudeClientAdapterWithOllama(client *claude.Client, ollamaClient *ollama.Client, cfg *config.Config, claudeLimiter *rate.Limiter, ollamaLimiter *rate.Limiter) *claudeClientAdapter {
	return &claudeClientAdapter{
		client:        client,
		ollama:        ollamaClient,
		config:        cfg,
		limiter:       claudeLimiter, // Store Claude limiter
		ollamaLimiter: ollamaLimiter, // Store Ollama limiter
	}
}

// GenerateChat implements the Client interface for Claude
func (a *claudeClientAdapter) GenerateChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Wait for Claude rate limiter
	if err := a.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("claude rate limiter error: %w", err)
	}

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
	// Wait for Claude rate limiter BEFORE starting the goroutine
	if err := a.limiter.Wait(ctx); err != nil {
		responseChan := make(chan ChatResponse)
		close(responseChan)
		return responseChan, fmt.Errorf("claude rate limiter error: %w", err)
	}

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
	// Check if we should use Ollama for embeddings
	if a.ollama != nil && a.config.Claude.EmbeddingModel == "ollama" {
		// Wait for OLLAMA rate limiter if available
		if a.ollamaLimiter != nil {
			if err := a.ollamaLimiter.Wait(ctx); err != nil {
				return nil, fmt.Errorf("ollama rate limiter error (via claude adapter): %w", err)
			}
		} else {
			// Should not happen if configured correctly, but log a warning
			loggy.Warn("Ollama client available for embeddings, but limiter is missing in claude adapter")
		}

		ollamaReq := ollama.EmbeddingRequest{
			// Use Ollama's embedding model from config
			Model: a.config.Ollama.EmbeddingModel,
			Input: req.Text,
		}

		resp, err := a.ollama.GenerateEmbedding(ctx, ollamaReq)
		if err != nil {
			// Try legacy endpoint as fallback
			legacyReq := ollama.SingleEmbeddingRequest{
				Model:  a.config.Ollama.EmbeddingModel,
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

	// Claude doesn't natively support embeddings
	return nil, fmt.Errorf("claude does not support embeddings natively and ollama delegation is not configured/available")
}

// BatchEmbeddings implements the Client interface for Claude
func (a *claudeClientAdapter) BatchEmbeddings(ctx context.Context, reqs []EmbeddingRequest) ([][]float32, error) {
	// Check if we should use Ollama for embeddings
	if a.ollama != nil && a.config.Claude.EmbeddingModel == "ollama" {
		// Wait for OLLAMA rate limiter if available (applied ONCE before the batch)
		if a.ollamaLimiter != nil {
			if err := a.ollamaLimiter.Wait(ctx); err != nil {
				return nil, fmt.Errorf("ollama rate limiter error (via claude adapter): %w", err)
			}
		} else {
			loggy.Warn("Ollama client available for embeddings, but limiter is missing in claude adapter")
		}

		ollamaReqs := make([]ollama.EmbeddingRequest, len(reqs))
		for i, req := range reqs {
			ollamaReqs[i] = ollama.EmbeddingRequest{
				// Use Ollama's embedding model from config
				Model: a.config.Ollama.EmbeddingModel,
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

	return nil, fmt.Errorf("claude does not support embeddings natively and ollama delegation is not configured/available")
}

// GenerateCompletion implements the Client interface for Claude
// Claude uses the Chat endpoint for completions, so rate limiting is handled by GenerateChat
func (a *claudeClientAdapter) GenerateCompletion(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Convert GenerateRequest to ChatRequest
	chatReq := ChatRequest{
		Model:       req.Model,
		Messages:    convertPromptToMessages(req.Prompt, req.System), // Need helper
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      false,
		Options:     req.Options,
	}

	// Call GenerateChat which includes rate limiting
	chatResp, err := a.GenerateChat(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("claude completion (via chat) failed: %w", err)
	}

	// Convert ChatResponse back to GenerateResponse
	return &GenerateResponse{
		Content:   chatResp.Content,
		Model:     chatResp.Model,
		Completed: chatResp.Completed,
		Error:     chatResp.Error,
	}, nil
}

// Helper to convert prompt/system to messages array
func convertPromptToMessages(prompt, system string) []Message {
	var messages []Message
	if system != "" {
		messages = append(messages, Message{Role: "system", Content: system})
	}
	messages = append(messages, Message{Role: "user", Content: prompt})
	return messages
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
