package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/review"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// Client handles HTTP communication with the Mindnest server
type Client struct {
	baseURL      string
	token        string
	timeout      time.Duration
	httpClient   *http.Client
	logger       *loggy.Logger
	settingsRepo config.SettingsRepository
}

// NewClient creates a new HTTP client for server communication
func NewClient(baseURL, token string, timeout time.Duration, logger *loggy.Logger) *Client {
	// Create HTTP client with custom transport for connection pooling
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	return &Client{
		baseURL:    baseURL,
		token:      token,
		timeout:    timeout,
		httpClient: httpClient,
		logger:     logger,
	}
}

// SetToken updates the authentication token
func (c *Client) SetToken(token string) {
	c.token = token
}

// SetSettingsRepository sets the settings repository for the client
func (c *Client) SetSettingsRepository(repo config.SettingsRepository) {
	c.settingsRepo = repo
}

// GetToken returns the current token, checking the settings repository if available
func (c *Client) GetToken() string {
	// If we have a settings repo, try to get the latest token from it
	if c.settingsRepo != nil && c.token == "" {
		// Use context with short timeout for DB lookup
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		token, err := c.settingsRepo.GetSetting(ctx, "sync.server_token")
		if err != nil {
			c.logger.Warn("Failed to get token from settings, using cached token", "error", err)
		} else if token != "" {
			// Update local cache
			c.token = token
		}
	}

	return c.token
}

// APIError represents an error response from the API
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	ErrorCode  string `json:"error"`
}

func (e APIError) Error() string {
	return fmt.Sprintf("API error %d: %s - %s", e.StatusCode, e.ErrorCode, e.Message)
}

// SyncWorkspaceRequest represents a request to sync a workspace
type SyncWorkspaceRequest struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Path        string          `json:"path"`
	GitRepoURL  string          `json:"git_repo_url,omitempty"`
	Description string          `json:"description,omitempty"`
	ModelConfig json.RawMessage `json:"model_config,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// SyncReviewRequest represents a request to sync a review
type SyncReviewRequest struct {
	ID          string            `json:"id"`
	WorkspaceID string            `json:"workspace_id"`
	Type        review.ReviewType `json:"review_type"`
	CommitHash  string            `json:"commit_hash,omitempty"`
	BranchFrom  string            `json:"branch_from,omitempty"`
	BranchTo    string            `json:"branch_to,omitempty"`
	Status      string            `json:"status"`
	Result      json.RawMessage   `json:"result,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// SyncReviewFileRequest represents a request to sync a review file
type SyncReviewFileRequest struct {
	ID          string                  `json:"id"`
	ReviewID    string                  `json:"review_id"`
	FileID      string                  `json:"file_id"`
	Status      review.ReviewFileStatus `json:"status"`
	IssuesCount int                     `json:"issues_count"`
	Summary     string                  `json:"summary,omitempty"`
	Assessment  string                  `json:"assessment,omitempty"`
	Metadata    json.RawMessage         `json:"metadata,omitempty"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

// SyncIssueRequest represents a request to sync an issue
type SyncIssueRequest struct {
	ID           string               `json:"id"`
	ReviewID     string               `json:"review_id"`
	FileID       string               `json:"file_id"`
	Type         review.IssueType     `json:"type"`
	Severity     review.IssueSeverity `json:"severity"`
	Title        string               `json:"title"`
	Description  string               `json:"description"`
	LineStart    int                  `json:"line_start,omitempty"`
	LineEnd      int                  `json:"line_end,omitempty"`
	Suggestion   string               `json:"suggestion,omitempty"`
	AffectedCode string               `json:"affected_code,omitempty"`
	CodeSnippet  string               `json:"code_snippet,omitempty"`
	IsValid      bool                 `json:"is_valid"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

// SyncFileRequest represents a request to sync a file
type SyncFileRequest struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspace_id"`
	Path        string          `json:"path"`
	Language    string          `json:"language"`
	LastParsed  *time.Time      `json:"last_parsed,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// SyncResponse represents a response from the API for a sync operation
type SyncResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// CreateWorkspaceFromModel creates a workspace sync request from a workspace model
func CreateWorkspaceFromModel(ws *workspace.Workspace) *SyncWorkspaceRequest {
	return &SyncWorkspaceRequest{
		ID:          ws.ID,
		Name:        ws.Name,
		Path:        ws.Path,
		GitRepoURL:  ws.GitRepoURL,
		Description: ws.Description,
		ModelConfig: ws.ModelConfig,
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
	}
}

// SyncWorkspace syncs a workspace to the server
func (c *Client) SyncWorkspace(ctx context.Context, workspace *SyncWorkspaceRequest) (*SyncResponse, error) {
	url := fmt.Sprintf("%s/api/sync/workspace", c.baseURL)
	return c.sendRequest(ctx, "POST", url, workspace)
}

// SyncReview syncs a review to the server
func (c *Client) SyncReview(ctx context.Context, review *SyncReviewRequest) (*SyncResponse, error) {
	url := fmt.Sprintf("%s/api/sync/review", c.baseURL)
	return c.sendRequest(ctx, "POST", url, review)
}

// SyncReviewFile syncs a review file to the server
func (c *Client) SyncReviewFile(ctx context.Context, reviewFile *SyncReviewFileRequest) (*SyncResponse, error) {
	url := fmt.Sprintf("%s/api/sync/review-file", c.baseURL)
	return c.sendRequest(ctx, "POST", url, reviewFile)
}

// SyncIssue syncs an issue to the server
func (c *Client) SyncIssue(ctx context.Context, issue *SyncIssueRequest) (*SyncResponse, error) {
	url := fmt.Sprintf("%s/api/sync/issue", c.baseURL)
	return c.sendRequest(ctx, "POST", url, issue)
}

// SyncFile syncs a file to the server
func (c *Client) SyncFile(ctx context.Context, file *SyncFileRequest) (*SyncResponse, error) {
	url := fmt.Sprintf("%s/api/sync/file", c.baseURL)
	return c.sendRequest(ctx, "POST", url, file)
}

// VerifyToken verifies if a token is valid
func (c *Client) VerifyToken(ctx context.Context) (bool, error) {
	url := fmt.Sprintf("%s/api/auth/verify", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("creating request: %w", err)
	}

	// Add auth headers with latest token
	token := c.GetToken()
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("Content-Type", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	// If status is unauthorized, token is invalid
	if resp.StatusCode == http.StatusUnauthorized {
		return false, nil
	}

	// For other errors, parse the error response
	var apiErr APIError
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		return false, fmt.Errorf("decoding error response: %w", err)
	}

	return false, apiErr
}

// sendRequest is a helper function to send requests to the API
func (c *Client) sendRequest(ctx context.Context, method, url string, body interface{}) (*SyncResponse, error) {
	// Convert body to JSON
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Get latest token and add auth headers
	token := c.GetToken()
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("Content-Type", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr APIError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			// If we can't decode the error, return a generic one
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, resp.Status)
		}
		return nil, apiErr
	}

	// Parse response
	var syncResp SyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &syncResp, nil
}
