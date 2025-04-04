package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v59/github"
	"github.com/tildaslashalef/mindnest/internal/config"
	"golang.org/x/oauth2"
)

// Client represents a GitHub API client
type Client struct {
	client *github.Client
	config *config.GitHubConfig
}

// NewClient creates a new GitHub API client with the provided token
func NewClient(token string) *Client {
	// Use the global config if available
	cfg, err := config.Get()
	if err != nil {
		cfg = config.New()
	}

	// If token is empty, use the one from config
	if token == "" {
		token = cfg.GitHub.Token
	}

	// Create auth token source
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	// Create HTTP client with appropriate timeout
	timeout := cfg.GitHub.RequestTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	tc := oauth2.NewClient(context.Background(), ts)
	tc.Timeout = timeout

	// Create GitHub client with custom base URL if specified
	var client *github.Client
	if cfg.GitHub.APIURL != "" && cfg.GitHub.APIURL != "https://api.github.com" {
		client, err = github.NewEnterpriseClient(cfg.GitHub.APIURL, cfg.GitHub.APIURL, tc)
		if err != nil {
			// Fall back to default client if enterprise client creation fails
			client = github.NewClient(tc)
		}
	} else {
		client = github.NewClient(tc)
	}

	return &Client{
		client: client,
		config: &cfg.GitHub,
	}
}

// GetPullRequest gets a pull request by number
func (c *Client) GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error) {
	// Owner and repo must be provided since we no longer have defaults
	if owner == "" || repo == "" {
		return nil, nil, fmt.Errorf("owner and repo must be provided")
	}

	return c.client.PullRequests.Get(ctx, owner, repo, number)
}

// GetPullRequestFiles gets the files in a pull request
func (c *Client) GetPullRequestFiles(ctx context.Context, owner, repo string, number int) ([]*github.CommitFile, *github.Response, error) {
	// Owner and repo must be provided since we no longer have defaults
	if owner == "" || repo == "" {
		return nil, nil, fmt.Errorf("owner and repo must be provided")
	}

	return c.client.PullRequests.ListFiles(ctx, owner, repo, number, nil)
}
