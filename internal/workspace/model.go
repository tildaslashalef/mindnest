// Package workspace provides workspace management for the Mindnest application
package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/ulid"
)

// IssueType represents the type of an issue
type IssueType string

// Issue severity levels
const (
	IssueTypeBug          IssueType = "bug"
	IssueTypeSecurity     IssueType = "security"
	IssueTypePerformance  IssueType = "performance"
	IssueTypeDesign       IssueType = "design"
	IssueTypeStyle        IssueType = "style"
	IssueTypeComplexity   IssueType = "complexity"
	IssueTypeBestPractice IssueType = "best_practice"
)

// IssueSeverity represents the severity of an issue
type IssueSeverity string

// Issue severity levels
const (
	IssueSeverityCritical IssueSeverity = "critical"
	IssueSeverityHigh     IssueSeverity = "high"
	IssueSeverityMedium   IssueSeverity = "medium"
	IssueSeverityLow      IssueSeverity = "low"
	IssueSeverityInfo     IssueSeverity = "info"
)

// Issue represents an issue identified during a code review
type Issue struct {
	ID           string                 `json:"id"`
	ReviewID     string                 `json:"review_id"`
	FileID       string                 `json:"file_id"`
	Type         IssueType              `json:"type"`
	Severity     IssueSeverity          `json:"severity"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	LineStart    int                    `json:"line_start,omitempty"`
	LineEnd      int                    `json:"line_end,omitempty"`
	Suggestion   string                 `json:"suggestion,omitempty"`
	AffectedCode string                 `json:"affected_code,omitempty"`
	CodeSnippet  string                 `json:"code_snippet,omitempty"`
	IsValid      bool                   `json:"is_valid,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	SyncedAt     time.Time              `json:"synced_at,omitempty"`
}

// Workspace represents a code workspace to be analyzed
type Workspace struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Path        string          `json:"path"`
	GitRepoURL  string          `json:"git_repo_url,omitempty"`
	Description string          `json:"description,omitempty"`
	ModelConfig json.RawMessage `json:"model_config,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	SyncedAt    time.Time       `json:"synced_at,omitempty"`
}

// New creates a new workspace with the given path and name
func New(path string, name string, cfg *config.Config) (*Workspace, error) {
	// Generate a new ULID
	id := ulid.WorkspaceID()

	// Normalize path to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path: %w", err)
	}

	// Check if path exists
	_, err = os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("checking workspace path: %w", err)
	}

	// Create initial model config JSON from the global config
	modelConfig, err := createInitialModelConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating initial model config: %w", err)
	}

	now := time.Now()

	return &Workspace{
		ID:          id,
		Name:        name,
		Path:        absPath,
		ModelConfig: modelConfig,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// createInitialModelConfig creates the initial model configuration JSON
func createInitialModelConfig(cfg *config.Config) (json.RawMessage, error) {
	// Create a map with the model configuration
	configMap := map[string]interface{}{
		"llm": map[string]interface{}{
			"default_provider": cfg.LLM.DefaultProvider,
			"default_model":    cfg.LLM.DefaultModel,
			"max_tokens":       cfg.LLM.MaxTokens,
			"temperature":      cfg.LLM.Temperature,
		},
		"embedding": map[string]interface{}{
			"model":            cfg.Embedding.Model,
			"n_similar_chunks": cfg.Embedding.NSimilarChunks,
		},
		"context": map[string]interface{}{
			"max_files_same_directory": cfg.Context.MaxFilesSameDir,
			"context_depth":            cfg.Context.ContextDepth,
		},
	}

	// Marshal the map to JSON
	modelConfig, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("marshaling model config: %w", err)
	}

	return modelConfig, nil
}

// HasGitRepo checks if the workspace path contains a Git repository
func (w *Workspace) HasGitRepo() bool {
	gitDir := filepath.Join(w.Path, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

// SetGitRepoURL sets the Git repository URL for the workspace
func (w *Workspace) SetGitRepoURL(url string) {
	w.GitRepoURL = url
	w.UpdatedAt = time.Now()
}

// SetDescription sets the description for the workspace
func (w *Workspace) SetDescription(description string) {
	w.Description = description
	w.UpdatedAt = time.Now()
}

// UpdateModelConfig updates the model configuration for the workspace
func (w *Workspace) UpdateModelConfig(cfg *config.Config) error {
	// Create a new model config
	modelConfig, err := createInitialModelConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating model config: %w", err)
	}

	w.ModelConfig = modelConfig
	w.UpdatedAt = time.Now()
	return nil
}

// SetModelConfig sets the model configuration for the workspace
func (w *Workspace) SetModelConfig(config json.RawMessage) {
	w.ModelConfig = config
	w.UpdatedAt = time.Now()
}
