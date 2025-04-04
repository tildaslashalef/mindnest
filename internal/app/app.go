// Package app provides the application initialization and lifecycle management
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/database"
	"github.com/tildaslashalef/mindnest/internal/git"
	"github.com/tildaslashalef/mindnest/internal/github"
	"github.com/tildaslashalef/mindnest/internal/llm"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/parser"
	"github.com/tildaslashalef/mindnest/internal/rag"
	"github.com/tildaslashalef/mindnest/internal/review"
	"github.com/tildaslashalef/mindnest/internal/sync"
	"github.com/tildaslashalef/mindnest/internal/workspace"
	"github.com/urfave/cli/v2"
)

// App represents the application instance with its dependencies
type App struct {
	Config    *config.Config
	Workspace *workspace.Service
	RAG       *rag.Service
	Review    *review.Service
	Sync      *sync.Service
	GitHub    *github.Service
	Settings  config.SettingsRepository
}

// New initializes a new application instance with all its dependencies
func New() (*App, error) {
	// Load configuration with default paths, not in initialization mode
	cfg, err := config.LoadFromEnv("", "", false)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Set the global configuration
	config.Set(cfg)

	// Initialize logger early so we can log
	err = loggy.Init(loggy.Config{
		Level:      config.ParseLogLevel(cfg.Logging.Level),
		Format:     cfg.Logging.Format,
		Output:     cfg.Logging.Output,
		AddSource:  cfg.Logging.AddSource,
		TimeFormat: cfg.Logging.TimeFormat,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Log initialization information
	loggy.Info("Application initializing",
		"version", os.Getenv("VERSION"),
		"log_level", cfg.Logging.Level,
	)

	// 4. Ensure necessary directories exist
	if err := ensureDirectories(cfg); err != nil {
		return nil, fmt.Errorf("failed to create necessary directories: %w", err)
	}

	// 5. Initialize database
	if err := database.InitDB(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// 6. Initialize workspace repository and service
	db, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	logger := loggy.GetGlobalLogger()

	// Initialize settings repository and load persistent settings
	settingsRepo := config.NewSQLSettingsRepository(db, logger)
	ctx := context.Background()
	if err := config.LoadSyncSettings(ctx, cfg, settingsRepo); err != nil {
		loggy.Warn("Failed to load sync settings from database", "error", err)
		// Continue anyway, using defaults
	}

	// Initialize git service
	gitService := git.NewService(logger)

	// Initialize parser service
	parserService := parser.NewService(logger)

	// Initialize workspace repository and service
	workspaceRepo := workspace.NewSQLRepository(db, logger)
	workspaceService := workspace.NewService(workspaceRepo, logger, gitService, parserService)

	// Initialize LLM client
	llmFactory := llm.NewFactory(cfg)
	llmClient, llmType, err := llmFactory.GetDefaultClient()
	if err != nil {
		loggy.Warn("Failed to initialize LLM client, embedding functionality will be disabled", "error", err)
	} else {
		loggy.Info("Initialized LLM client", "type", llmType)
	}

	// Initialize RAG vector repository
	vectorRepo := rag.NewSQLRepository(db, logger)

	// Initialize RAG service with full functionality
	ragService := rag.NewService(
		workspaceService,
		vectorRepo,
		llmClient,
		cfg,
		logger,
	)

	// Initialize review repository and service
	reviewRepo := review.NewSQLRepository(db, logger)
	reviewService := review.NewService(
		reviewRepo,
		workspaceService,
		ragService,
		llmClient,
		cfg,
		logger,
	)

	// Initialize sync repository and service
	syncRepo := sync.NewSQLRepository(db, logger)
	syncService := sync.NewService(
		cfg,
		syncRepo,
		workspaceRepo,
		workspaceService,
		reviewRepo,
		reviewService,
		logger,
	)

	// Attach the settings repository to the sync service
	syncService.SetSettingsRepository(settingsRepo)

	// Initialize GitHub service
	githubService := github.NewService(
		cfg,
		logger,
	)
	githubService.SetSettingsRepository(settingsRepo)
	// Set workspace service for repository URL lookup
	githubService.SetWorkspaceService(workspaceService)

	loggy.Info("Application initialized successfully")

	return &App{
		Config:    cfg,
		Workspace: workspaceService,
		RAG:       ragService,
		Review:    reviewService,
		Sync:      syncService,
		GitHub:    githubService,
		Settings:  settingsRepo,
	}, nil
}

// Shutdown gracefully shuts down the application
func (app *App) Shutdown() error {
	loggy.Info("Shutting down application")

	// Close database connection
	if err := database.CloseDB(); err != nil {
		loggy.Error("Error closing database connection", "error", err)
	}

	return nil
}

// FromContext retrieves the App instance from the CLI context
func FromContext(c *cli.Context) (*App, error) {
	if c.App.Metadata == nil {
		return nil, fmt.Errorf("app metadata not found in context")
	}

	app, ok := c.App.Metadata["app"].(*App)
	if !ok {
		return nil, fmt.Errorf("app instance not found in context")
	}

	return app, nil
}

// ensureDirectories creates necessary directories for the application
func ensureDirectories(cfg *config.Config) error {
	// Ensure database directory exists
	dbDir := filepath.Dir(cfg.Database.Path)
	if dbDir != "" && dbDir != "." && dbDir != ":memory:" {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	return nil
}
