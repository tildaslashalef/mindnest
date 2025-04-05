package llm

import (
	"context"
	"fmt"

	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/gemini"
)

// geminiClientAdapter adapts the Gemini client to the LLM Client interface
type geminiClientAdapter struct {
	client *gemini.Client
	config *config.Config
}

// newGeminiClientAdapter creates a new Gemini client adapter
func newGeminiClientAdapter(client *gemini.Client, cfg *config.Config) *geminiClientAdapter {
	return &geminiClientAdapter{
		client: client,
		config: cfg,
	}
}

// GenerateChat implements the Client interface for Gemini
func (a *geminiClientAdapter) GenerateChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Convert llm.Message to gemini.Content
	contents := make([]gemini.Content, len(req.Messages))
	for i, msg := range req.Messages {
		contents[i] = gemini.Content{
			Role: convertRoleToGemini(msg.Role),
			Parts: []gemini.Part{
				{Text: msg.Content},
			},
		}
	}

	// Create Gemini request
	geminiReq := gemini.ChatRequest{
		Model:       req.Model,
		Contents:    contents,
		MaxTokens:   req.MaxTokens,
		Temperature: getTemperature(req.Temperature),
		Stream:      req.Stream,
	}

	// Set options if provided
	if req.Options != nil {
		if topP, ok := req.Options["top_p"].(float64); ok {
			geminiReq.TopP = &topP
		}
		if topK, ok := req.Options["top_k"].(int); ok {
			geminiReq.TopK = &topK
		}
	}

	// Make the request
	resp, err := a.client.GenerateChat(ctx, geminiReq)
	if err != nil {
		return nil, fmt.Errorf("gemini chat generation failed: %w", err)
	}

	// Convert to ChatResponse
	var content string
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		content = resp.Candidates[0].Content.Parts[0].Text
	}

	return &ChatResponse{
		Content:   content,
		Model:     req.Model,
		Completed: true,
		Error:     resp.ErrorMsg,
	}, nil
}

// GenerateChatStream implements the Client interface for Gemini
func (a *geminiClientAdapter) GenerateChatStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	// Convert llm.Message to gemini.Content
	contents := make([]gemini.Content, len(req.Messages))
	for i, msg := range req.Messages {
		contents[i] = gemini.Content{
			Role: convertRoleToGemini(msg.Role),
			Parts: []gemini.Part{
				{Text: msg.Content},
			},
		}
	}

	// Create Gemini request
	geminiReq := gemini.ChatRequest{
		Model:       req.Model,
		Contents:    contents,
		MaxTokens:   req.MaxTokens,
		Temperature: getTemperature(req.Temperature),
		Stream:      true,
	}

	// Set options if provided
	if req.Options != nil {
		if topP, ok := req.Options["top_p"].(float64); ok {
			geminiReq.TopP = &topP
		}
		if topK, ok := req.Options["top_k"].(int); ok {
			geminiReq.TopK = &topK
		}
	}

	// Get the stream channel
	geminiRespChan, err := a.client.GenerateChatStream(ctx, geminiReq)
	if err != nil {
		return nil, fmt.Errorf("gemini stream generation failed: %w", err)
	}

	// Create output channel
	responseChan := make(chan ChatResponse)

	// Process stream in goroutine
	go func() {
		defer close(responseChan)

		for geminiResp := range geminiRespChan {
			var content string
			if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
				content = geminiResp.Candidates[0].Content.Parts[0].Text
			}

			responseChan <- ChatResponse{
				Content:   content,
				Model:     req.Model,
				Completed: len(geminiResp.Candidates) > 0 && geminiResp.Candidates[0].FinishReason != "",
				Error:     geminiResp.ErrorMsg,
			}
		}
	}()

	return responseChan, nil
}

// GenerateCompletion implements the Client interface for Gemini
func (a *geminiClientAdapter) GenerateCompletion(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Create content from prompt
	contents := []gemini.Content{
		{
			Role: "user",
			Parts: []gemini.Part{
				{Text: req.Prompt},
			},
		},
	}

	// Add system message if provided
	if req.System != "" {
		// Gemini doesn't have a direct system message, so we prepend it as a user message with a marker
		systemContent := gemini.Content{
			Role: "user",
			Parts: []gemini.Part{
				{Text: fmt.Sprintf("[System instruction] %s", req.System)},
			},
		}
		contents = append([]gemini.Content{systemContent}, contents...)
	}

	// Create Gemini request
	geminiReq := gemini.ChatRequest{
		Model:       req.Model,
		Contents:    contents,
		MaxTokens:   req.MaxTokens,
		Temperature: getTemperature(req.Temperature),
		Stream:      false,
	}

	// Make the request
	resp, err := a.client.GenerateChat(ctx, geminiReq)
	if err != nil {
		return nil, fmt.Errorf("gemini completion failed: %w", err)
	}

	// Convert to GenerateResponse
	var content string
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		content = resp.Candidates[0].Content.Parts[0].Text
	}

	return &GenerateResponse{
		Content:   content,
		Model:     req.Model,
		Completed: true,
		Error:     resp.ErrorMsg,
	}, nil
}

// GenerateEmbedding implements the Client interface for Gemini
func (a *geminiClientAdapter) GenerateEmbedding(ctx context.Context, req EmbeddingRequest) ([]float32, error) {
	geminiReq := gemini.EmbeddingRequest{
		// Always use the embedding model from config
		Model: a.config.Gemini.EmbeddingModel,
		Text:  req.Text,
	}

	resp, err := a.client.GenerateEmbedding(ctx, geminiReq)
	if err != nil {
		return nil, fmt.Errorf("gemini embedding failed: %w", err)
	}

	return resp.Embedding, nil
}

// BatchEmbeddings implements the Client interface for Gemini
func (a *geminiClientAdapter) BatchEmbeddings(ctx context.Context, reqs []EmbeddingRequest) ([][]float32, error) {
	// Convert requests
	geminiReqs := make([]gemini.EmbeddingRequest, len(reqs))
	for i, req := range reqs {
		geminiReqs[i] = gemini.EmbeddingRequest{
			// Always use the embedding model from config
			Model: a.config.Gemini.EmbeddingModel,
			Text:  req.Text,
		}
	}

	// Make the batch request
	resps, err := a.client.BatchEmbeddings(ctx, geminiReqs)
	if err != nil {
		return nil, fmt.Errorf("gemini batch embeddings failed: %w", err)
	}

	// Convert responses
	embeddings := make([][]float32, len(resps))
	for i, resp := range resps {
		embeddings[i] = resp.Embedding
	}

	return embeddings, nil
}

// convertRoleToGemini converts standard roles to Gemini roles
func convertRoleToGemini(role string) string {
	switch role {
	case "system":
		return "user" // Gemini doesn't have system role, handled specially
	case "assistant":
		return "model"
	case "user":
		return "user"
	default:
		return "user"
	}
}

// getTemperature returns a pointer to the temperature value
func getTemperature(temp float64) *float64 {
	if temp <= 0 {
		return nil
	}
	return &temp
}
