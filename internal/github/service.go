package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v59/github"
	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// PRDetails contains the repository information needed for PR comments
type PRDetails struct {
	Owner string
	Repo  string
}

// PRComment contains all details needed to submit a comment to a GitHub PR
type PRComment struct {
	Owner       string
	Repo        string
	WorkspaceID string
	PRNumber    int
	FilePath    string
	LineStart   int
	LineEnd     int
	Commentary  string
}

// Service provides GitHub integration functionality
type Service struct {
	client           *Client
	config           *config.Config
	logger           *loggy.Logger
	settingsRepo     config.SettingsRepository
	workspaceService interface{}
}

// NewService creates a new GitHub service
func NewService(
	config *config.Config,
	logger *loggy.Logger,
) *Service {
	// Create client with token from config
	client := NewClient(config.GitHub.Token)

	return &Service{
		client: client,
		config: config,
		logger: logger,
	}
}

// SetSettingsRepository sets the settings repository for persisting configuration
func (s *Service) SetSettingsRepository(repo config.SettingsRepository) {
	s.settingsRepo = repo
}

// SetWorkspaceService sets the workspace service to use for workspace lookups
func (s *Service) SetWorkspaceService(service interface{}) {
	s.workspaceService = service
}

// SubmitPRComment submits a comment to a GitHub PR with user-provided content
func (s *Service) SubmitPRComment(ctx context.Context, comment *PRComment) error {
	owner := comment.Owner
	repo := comment.Repo

	// If WorkspaceID is provided and we have a workspace service, try to get repository details
	if comment.WorkspaceID != "" && s.workspaceService != nil {
		// Check if the workspace service has the GetWorkspace method
		if wsService, ok := s.workspaceService.(interface {
			GetWorkspace(context.Context, string) (*workspace.Workspace, error)
		}); ok {
			// Get workspace and extract owner/repo from GitRepoURL
			ws, err := wsService.GetWorkspace(ctx, comment.WorkspaceID)
			if err == nil && ws.GitRepoURL != "" {
				extractedOwner, extractedRepo, err := s.ExtractRepoDetailsFromURL(ws.GitRepoURL)
				if err == nil {
					owner = extractedOwner
					repo = extractedRepo
					s.logger.Debug("Using repository details from workspace",
						"workspace_id", comment.WorkspaceID,
						"git_repo_url", ws.GitRepoURL,
						"owner", owner,
						"repo", repo)
				} else {
					s.logger.Warn("Failed to extract repo details from workspace GitRepoURL",
						"workspace_id", comment.WorkspaceID,
						"git_repo_url", ws.GitRepoURL,
						"error", err)
				}
			}
		} else {
			s.logger.Debug("WorkspaceID provided, but workspace service doesn't have GetWorkspace method",
				"workspace_id", comment.WorkspaceID)
		}
	}

	// Validate that we have owner and repo values before proceeding
	if owner == "" || repo == "" {
		return fmt.Errorf("unable to determine GitHub repository owner/repo; ensure workspace has a valid GitRepoURL")
	}

	// First, get the PR to fetch the latest commit SHA
	pr, _, err := s.client.GetPullRequest(ctx, owner, repo, comment.PRNumber)
	if err != nil {
		s.logger.Error("Failed to get PR details",
			"error", err,
			"repo", fmt.Sprintf("%s/%s", owner, repo),
			"pr", comment.PRNumber)
		return fmt.Errorf("failed to get PR details: %w", err)
	}

	// Get the latest commit SHA
	if pr.Head == nil || pr.Head.SHA == nil {
		return fmt.Errorf("unable to determine head commit SHA for PR #%d", comment.PRNumber)
	}
	commitSHA := *pr.Head.SHA

	// Create GitHub PR comment with required fields
	prComment := &github.PullRequestComment{
		Path:     github.String(comment.FilePath),
		CommitID: github.String(commitSHA),
		Body:     github.String(comment.Commentary),
	}

	// Set line number instead of position - use LineEnd as the primary line
	if comment.LineEnd > 0 {
		prComment.Line = github.Int(comment.LineEnd)
	} else if comment.LineStart > 0 {
		prComment.Line = github.Int(comment.LineStart)
	}

	// Submit comment directly to GitHub
	_, _, err = s.client.client.PullRequests.CreateComment(
		ctx,
		owner,
		repo,
		comment.PRNumber,
		prComment,
	)

	if err != nil {
		s.logger.Error("Failed to submit comment to GitHub PR",
			"error", err,
			"repo", fmt.Sprintf("%s/%s", owner, repo),
			"pr", comment.PRNumber)
		return fmt.Errorf("failed to submit comment: %w", err)
	}

	s.logger.Info("Successfully submitted comment to GitHub PR",
		"pr_url", fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, comment.PRNumber))

	return nil
}

// ValidateRepoAccess checks if the GitHub token has access to the specified repository
func (s *Service) ValidateRepoAccess(ctx context.Context, owner, repo string) error {
	_, resp, err := s.client.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return fmt.Errorf("repository %s/%s not found or no access", owner, repo)
		}
		return fmt.Errorf("failed to validate repository access: %w", err)
	}

	return nil
}

// GetDefaultPRDetails returns an empty PRDetails since we no longer store default values
// This is kept for backward compatibility but should be migrated to workspace-based lookups
func (s *Service) GetDefaultPRDetails() PRDetails {
	// Return empty details - we'll use workspace gitURL instead
	return PRDetails{
		Owner: "",
		Repo:  "",
	}
}

// ExtractRepoDetailsFromURL extracts owner and repo from a Git URL
func (s *Service) ExtractRepoDetailsFromURL(gitURL string) (owner, repo string, err error) {
	if gitURL == "" {
		return "", "", fmt.Errorf("empty Git URL")
	}

	// Handle different URL formats
	// https://github.com/owner/repo.git
	// git@github.com:owner/repo.git
	// https://github.com/owner/repo

	// Remove .git suffix if present
	if strings.HasSuffix(gitURL, ".git") {
		gitURL = gitURL[:len(gitURL)-4]
	}

	var parts []string

	if strings.Contains(gitURL, "github.com/") {
		// Handle HTTPS URLs
		parts = strings.Split(gitURL, "github.com/")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid GitHub URL format: %s", gitURL)
		}
	} else if strings.Contains(gitURL, "github.com:") {
		// Handle SSH URLs
		parts = strings.Split(gitURL, "github.com:")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid GitHub SSH URL format: %s", gitURL)
		}
	} else {
		return "", "", fmt.Errorf("unsupported Git URL format: %s", gitURL)
	}

	// Split owner/repo
	ownerRepo := strings.Split(parts[1], "/")
	if len(ownerRepo) < 2 {
		return "", "", fmt.Errorf("could not extract owner/repo from URL: %s", gitURL)
	}

	return ownerRepo[0], ownerRepo[1], nil
}

// GetRepoDetailsFromWorkspace gets the owner and repo from workspace
func (s *Service) GetRepoDetailsFromWorkspace(ctx context.Context, workspaceID string, workspaceService interface {
	GetWorkspace(ctx context.Context, id string) (*workspace.Workspace, error)
}) (owner, repo string, err error) {
	// Get the workspace using the provided service
	ws, err := workspaceService.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get workspace: %w", err)
	}

	// If workspace has no GitRepoURL, return error
	if ws.GitRepoURL == "" {
		return "", "", fmt.Errorf("workspace has no Git repository URL")
	}

	// Extract owner and repo from the Git URL
	return s.ExtractRepoDetailsFromURL(ws.GitRepoURL)
}
