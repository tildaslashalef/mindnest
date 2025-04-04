package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/tildaslashalef/mindnest/internal/claude"
	"github.com/tildaslashalef/mindnest/internal/ollama"
)

// ollamaClientAdapter adapts the Ollama client to the LLM Client interface
type ollamaClientAdapter struct {
	client *ollama.Client
}

// newOllamaClientAdapter creates a new Ollama client adapter
func newOllamaClientAdapter(client *ollama.Client) *ollamaClientAdapter {
	return &ollamaClientAdapter{client: client}
}

// GenerateChat implements the Client interface for Ollama
func (a *ollamaClientAdapter) GenerateChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Create base request
	ollamaReq := ollama.ChatRequest{
		Model:    req.Model,
		Stream:   req.Stream,
		Messages: convertMessagesToOllama(req.Messages),
	}

	// If stream is not explicitly set, default to false
	if !req.Stream {
		ollamaReq.Stream = false
	}

	// Set options
	options := &ollama.RequestOptions{}
	if req.MaxTokens > 0 {
		numPredict := req.MaxTokens
		options.NumPredict = &numPredict
	}
	if req.Temperature > 0 {
		temp := req.Temperature
		options.Temperature = &temp
	}

	// Add any additional options from the request
	if req.Options != nil {
		for k, v := range req.Options {
			switch k {
			case "top_p":
				if val, ok := v.(float64); ok {
					options.TopP = &val
				}
			case "top_k":
				if val, ok := v.(int); ok {
					options.TopK = &val
				}
			case "seed":
				if val, ok := v.(int); ok {
					options.Seed = &val
				}
			case "num_ctx":
				if val, ok := v.(int); ok {
					options.NumCtx = &val
				}
			case "repeat_penalty":
				if val, ok := v.(float64); ok {
					options.RepeatPenalty = &val
				}
			case "stop":
				if val, ok := v.([]string); ok {
					options.Stop = val
				}
			}
		}
	}

	// Only set options if we have any non-nil values
	if options.Temperature != nil || options.TopP != nil || options.TopK != nil ||
		options.NumPredict != nil || options.Seed != nil || options.NumCtx != nil ||
		options.RepeatPenalty != nil || len(options.Stop) > 0 {
		ollamaReq.Options = options
	}

	// Make the request
	resp, err := a.client.GenerateChat(ctx, ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama chat generation failed: %w", err)
	}

	// Convert to ChatResponse
	return &ChatResponse{
		Content:   resp.Message.Content,
		Model:     resp.Model,
		Completed: resp.Done,
		Error:     resp.Error,
	}, nil
}

// GenerateChatStream implements the Client interface for Ollama
func (a *ollamaClientAdapter) GenerateChatStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	// Convert to Ollama request
	ollamaReq := ollama.ChatRequest{
		Model:    req.Model,
		Stream:   true,
		Messages: convertMessagesToOllama(req.Messages),
	}

	// Set options if provided
	if req.MaxTokens > 0 {
		numPredict := req.MaxTokens
		ollamaReq.Options = &ollama.RequestOptions{
			NumPredict: &numPredict,
		}
	}
	if req.Temperature > 0 {
		temp := req.Temperature
		if ollamaReq.Options == nil {
			ollamaReq.Options = &ollama.RequestOptions{
				Temperature: &temp,
			}
		} else {
			ollamaReq.Options.Temperature = &temp
		}
	}

	// Add any additional options that were provided
	if len(req.Options) > 0 {
		if ollamaReq.Options == nil {
			ollamaReq.Options = &ollama.RequestOptions{}
		}

		for k, v := range req.Options {
			switch k {
			case "top_p":
				if val, ok := v.(float64); ok {
					ollamaReq.Options.TopP = &val
				}
			case "top_k":
				if val, ok := v.(int); ok {
					ollamaReq.Options.TopK = &val
				}
			case "seed":
				if val, ok := v.(int); ok {
					ollamaReq.Options.Seed = &val
				}
			case "num_ctx":
				if val, ok := v.(int); ok {
					ollamaReq.Options.NumCtx = &val
				}
			case "repeat_penalty":
				if val, ok := v.(float64); ok {
					ollamaReq.Options.RepeatPenalty = &val
				}
			case "stop":
				if val, ok := v.([]string); ok {
					ollamaReq.Options.Stop = val
				}
			}
		}
	}

	// Get the stream channel
	ollamaRespChan, err := a.client.GenerateChatStream(ctx, ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama stream generation failed: %w", err)
	}

	// Create output channel
	responseChan := make(chan ChatResponse)

	// Process stream in goroutine
	go func() {
		defer close(responseChan)

		for ollamaResp := range ollamaRespChan {
			responseChan <- ChatResponse{
				Content:   ollamaResp.Message.Content,
				Model:     ollamaResp.Model,
				Completed: ollamaResp.Done,
				Error:     ollamaResp.Error,
			}
		}
	}()

	return responseChan, nil
}

// GenerateEmbedding implements the Client interface for Ollama
func (a *ollamaClientAdapter) GenerateEmbedding(ctx context.Context, req EmbeddingRequest) ([]float32, error) {
	ollamaReq := ollama.EmbeddingRequest{
		Model: req.Model,
		Input: req.Text,
	}

	resp, err := a.client.GenerateEmbedding(ctx, ollamaReq)
	if err != nil {
		// Try legacy endpoint as fallback
		legacyReq := ollama.SingleEmbeddingRequest{
			Model:  req.Model,
			Prompt: req.Text,
		}

		legacyResp, legacyErr := a.client.GenerateSingleEmbedding(ctx, legacyReq)
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

// BatchEmbeddings implements the Client interface for Ollama
func (a *ollamaClientAdapter) BatchEmbeddings(ctx context.Context, reqs []EmbeddingRequest) ([][]float32, error) {
	ollamaReqs := make([]ollama.EmbeddingRequest, len(reqs))
	for i, req := range reqs {
		ollamaReqs[i] = ollama.EmbeddingRequest{
			Model: req.Model,
			Input: req.Text,
		}
	}

	resps, err := a.client.BatchEmbeddings(ctx, ollamaReqs)
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

// Helper to convert message format
func convertMessagesToOllama(messages []Message) []ollama.Message {
	ollamaMessages := make([]ollama.Message, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = ollama.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return ollamaMessages
}

// claudeClientAdapter adapts the Claude client to the LLM Client interface
type claudeClientAdapter struct {
	client *claude.Client
	ollama *ollama.Client // Added for embedding support
}

// newClaudeClientAdapter creates a new Claude client adapter
func newClaudeClientAdapter(client *claude.Client) *claudeClientAdapter {
	return &claudeClientAdapter{client: client}
}

// newClaudeClientAdapterWithOllama creates a new Claude client adapter with Ollama for embeddings
func newClaudeClientAdapterWithOllama(client *claude.Client, ollamaClient *ollama.Client) *claudeClientAdapter {
	return &claudeClientAdapter{
		client: client,
		ollama: ollamaClient,
	}
}

// GenerateChat implements the Client interface for Claude
func (a *claudeClientAdapter) GenerateChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Convert to Claude request
	claudeReq := claude.ChatRequest{
		Model:    req.Model,
		Stream:   false,
		Messages: convertMessagesToClaude(req.Messages),
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

	// Extract system message if present
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			claudeReq.System = msg.Content
			break
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
	// Convert to Claude request
	claudeReq := claude.ChatRequest{
		Model:    req.Model,
		Stream:   true,
		Messages: convertMessagesToClaude(req.Messages),
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

	// Extract system message if present
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			claudeReq.System = msg.Content
			break
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
			Model: req.Model,
			Input: req.Text,
		}

		resp, err := a.ollama.GenerateEmbedding(ctx, ollamaReq)
		if err != nil {
			// Try legacy endpoint as fallback
			legacyReq := ollama.SingleEmbeddingRequest{
				Model:  req.Model,
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
				Model: req.Model,
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

// Helper to convert message format
func convertMessagesToClaude(messages []Message) []claude.Message {
	claudeMessages := make([]claude.Message, 0, len(messages))

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

// GenerateCompletion implements the Client interface for Ollama
func (a *ollamaClientAdapter) GenerateCompletion(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Convert to Ollama request
	ollamaReq := ollama.GenerateRequest{
		Model:  req.Model,
		Prompt: req.Prompt,
		System: req.System,
		Stream: false, // Explicitly set to false for non-streaming requests
	}

	// Set options if provided
	if req.MaxTokens > 0 {
		numPredict := req.MaxTokens
		ollamaReq.Options = &ollama.RequestOptions{
			NumPredict: &numPredict,
		}
	}
	if req.Temperature > 0 {
		temp := req.Temperature
		if ollamaReq.Options == nil {
			ollamaReq.Options = &ollama.RequestOptions{
				Temperature: &temp,
			}
		} else {
			ollamaReq.Options.Temperature = &temp
		}
	}

	// Add any additional options from the request
	if req.Options != nil {
		if ollamaReq.Options == nil {
			ollamaReq.Options = &ollama.RequestOptions{}
		}

		for k, v := range req.Options {
			switch k {
			case "top_p":
				if val, ok := v.(float64); ok {
					ollamaReq.Options.TopP = &val
				}
			case "top_k":
				if val, ok := v.(int); ok {
					ollamaReq.Options.TopK = &val
				}
			case "seed":
				if val, ok := v.(int); ok {
					ollamaReq.Options.Seed = &val
				}
			case "num_ctx":
				if val, ok := v.(int); ok {
					ollamaReq.Options.NumCtx = &val
				}
			case "repeat_penalty":
				if val, ok := v.(float64); ok {
					ollamaReq.Options.RepeatPenalty = &val
				}
			case "stop":
				if val, ok := v.([]string); ok {
					ollamaReq.Options.Stop = val
				}
			}
		}
	}

	// Make the request
	resp, err := a.client.GenerateCompletion(ctx, ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama completion generation failed: %w", err)
	}

	// Convert to GenerateResponse
	return &GenerateResponse{
		Content:   resp.Response,
		Model:     resp.Model,
		Completed: resp.Done,
		Error:     resp.Error,
	}, nil
}

// GenerateCompletion implements the Client interface for Claude
func (a *claudeClientAdapter) GenerateCompletion(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Claude doesn't have a direct equivalent to the generate endpoint
	// We could implement this by using the chat API, but for now we'll just return an error
	return nil, fmt.Errorf("claude does not support the generate endpoint directly, use GenerateChat instead")
}
