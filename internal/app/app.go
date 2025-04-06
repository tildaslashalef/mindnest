// Package app provides the application initialization and lifecycle management
package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"

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
	Settings  *config.SettingsService
}

// New initializes a new application instance with all its dependencies
func New() (*App, error) {
	// Initialize configuration
	cfg, err := initConfig()
	if err != nil {
		return nil, err
	}

	// Initialize logger
	if err := initLogger(cfg); err != nil {
		return nil, err
	}

	// Log initialization information
	loggy.Info("Application initializing",
		"version", os.Getenv("VERSION"),
		"log_level", cfg.Logging.Level,
	)

	// Initialize database
	if err := database.InitDB(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Get database connection
	db, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	// Initialize all services
	app, err := initServices(cfg, db)
	if err != nil {
		return nil, err
	}

	loggy.Info("Application initialized successfully")
	return app, nil
}

// initConfig loads and sets up the application configuration
func initConfig() (*config.Config, error) {
	// Load configuration with default paths, not in initialization mode
	cfg, err := config.LoadFromEnv("", "", false)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Set the global configuration
	config.Set(cfg)
	return cfg, nil
}

// initLogger initializes the logging system
func initLogger(cfg *config.Config) error {
	err := loggy.Init(loggy.Config{
		Level:      config.ParseLogLevel(cfg.Logging.Level),
		Format:     cfg.Logging.Format,
		Output:     cfg.Logging.Output,
		AddSource:  cfg.Logging.AddSource,
		TimeFormat: cfg.Logging.TimeFormat,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	return nil
}

// initServices initializes all application services
func initServices(cfg *config.Config, db *sql.DB) (*App, error) {
	logger := loggy.GetGlobalLogger()
	ctx := context.Background()

	// Core services initialization
	settingsService := config.NewSettingsService(db, cfg, logger)
	if err := settingsService.LoadSyncSettings(ctx); err != nil {
		loggy.Warn("Failed to load sync settings from database", "error", err)
		// Continue anyway, using defaults
	}

	// Initialize supporting services
	gitService := git.NewService(logger)
	parserService := parser.NewService(logger)

	// Initialize primary services
	workspaceService := workspace.NewService(db, logger, gitService, parserService)

	// Initialize LLM client
	llmClient, llmType, err := initLLMClient(cfg, logger)
	if err != nil {
		// Non-fatal error, continue with nil client
		loggy.Warn("Failed to initialize LLM client, embedding functionality will be disabled", "error", err)
	} else {
		loggy.Info("Initialized LLM client", "type", llmType)
	}

	// Initialize application services
	ragService := rag.NewService(
		workspaceService,
		db,
		llmClient,
		cfg,
		logger,
	)

	reviewService := review.NewService(
		db,
		workspaceService,
		ragService,
		llmClient,
		cfg,
		logger,
	)

	syncService := sync.NewService(
		cfg,
		workspaceService,
		db,
		reviewService,
		settingsService,
		logger,
	)

	githubService := github.NewService(
		cfg,
		logger,
		settingsService,
		workspaceService,
	)

	// Return the initialized application
	return &App{
		Config:    cfg,
		Workspace: workspaceService,
		RAG:       ragService,
		Review:    reviewService,
		Sync:      syncService,
		GitHub:    githubService,
		Settings:  settingsService,
	}, nil
}

// initLLMClient initializes the LLM client
func initLLMClient(cfg *config.Config, logger *loggy.Logger) (llm.Client, llm.ClientType, error) {
	llmFactory := llm.NewFactory(cfg, logger)
	return llmFactory.GetDefaultClient()
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
