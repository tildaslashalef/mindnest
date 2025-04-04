package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() (*Config, error) {
	// Load empty configuration
	cfg := New()
	homeDir := filepath.Dir(filepath.Dir(cfg.Database.Path))
	appDir := filepath.Join(homeDir, "Code", "mindnest")

	// Check if ENV_FILE_PATH is set to load from a .env file
	envFilePath := getEnvString("ENV_FILE_PATH", "")
	if envFilePath != "" {
		err := godotenv.Load(envFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load env file: %w", err)
		}
	} else {
		// Try to load .env from current directory
		_ = godotenv.Load() // Ignore errors if file doesn't exist
	}

	// LLM Configuration
	cfg.LLM = LLMConfig{
		DefaultProvider: getEnvString("MINDNEST_LLM_DEFAULT_PROVIDER", "claude"),
		DefaultModel:    getEnvString("MINDNEST_LLM_DEFAULT_MODEL", "claude-3-7-sonnet-20250219"),
		MaxTokens:       getEnvInt("MINDNEST_LLM_MAX_TOKENS", 4096),
		Temperature:     getEnvFloat("MINDNEST_LLM_TEMPERATURE", 0.1),
	}

	// Ollama Configuration
	cfg.Ollama = OllamaConfig{
		Endpoint:            getEnvString("MINDNEST_OLLAMA_ENDPOINT", "http://localhost:11434"),
		Timeout:             getEnvDuration("MINDNEST_OLLAMA_TIMEOUT", 5*time.Minute),
		MaxRetries:          getEnvInt("MINDNEST_OLLAMA_MAX_RETRIES", 3),
		DefaultModel:        getEnvString("MINDNEST_OLLAMA_DEFAULT_MODEL", "gemma3"),
		MaxIdleConns:        getEnvInt("MINDNEST_OLLAMA_MAX_IDLE_CONNS", 100),
		MaxIdleConnsPerHost: getEnvInt("MINDNEST_OLLAMA_MAX_IDLE_CONNS_PER_HOST", 100),
		IdleConnTimeout:     getEnvDuration("MINDNEST_OLLAMA_IDLE_CONN_TIMEOUT", 90*time.Second),
	}

	// Load Claude config
	cfg.Claude = ClaudeConfig{
		APIKey:           getEnvString("MINDNEST_CLAUDE_API_KEY", ""),
		BaseURL:          getEnvString("MINDNEST_CLAUDE_BASE_URL", "https://api.anthropic.com"),
		Model:            getEnvString("MINDNEST_CLAUDE_MODEL", "claude-3-7-sonnet-20250219"),
		Timeout:          getEnvDuration("MINDNEST_CLAUDE_TIMEOUT", 60*time.Second),
		MaxRetries:       getEnvInt("MINDNEST_CLAUDE_MAX_RETRIES", 3),
		MaxTokens:        getEnvInt("MINDNEST_CLAUDE_MAX_TOKENS", 4096),
		Temperature:      getEnvFloat("MINDNEST_CLAUDE_TEMPERATURE", 0.1),
		TopP:             getEnvFloat("MINDNEST_CLAUDE_TOP_P", 0.9),
		TopK:             getEnvInt("MINDNEST_CLAUDE_TOP_K", 40),
		APIVersion:       getEnvString("MINDNEST_CLAUDE_API_VERSION", "2023-06-01"),
		APIBeta:          strings.Split(getEnvString("MINDNEST_CLAUDE_API_BETA", ""), ","),
		UseStopSequences: getEnvBool("MINDNEST_CLAUDE_USE_STOP_SEQUENCES", false),
		StopSequences:    strings.Split(getEnvString("MINDNEST_CLAUDE_STOP_SEQUENCES", ""), ","),
	}

	// Embedding Configuration
	cfg.Embedding = EmbeddingConfig{
		Model:          getEnvString("MINDNEST_EMBEDDING_MODEL", "nomic-embed-text"),
		NSimilarChunks: getEnvInt("MINDNEST_EMBEDDING_N_SIMILAR_CHUNKS", 5),
		Timeout:        getEnvDuration("MINDNEST_EMBEDDING_TIMEOUT", 30*time.Second),
		ExecPath:       getEnvString("MINDNEST_EMBEDDING_EXEC_PATH", ""),
		BatchSize:      getEnvInt("MINDNEST_EMBEDDING_BATCH_SIZE", 20),
	}

	// Context Configuration
	cfg.Context = ContextConfig{
		MaxFilesSameDir: getEnvInt("MINDNEST_CONTEXT_MAX_FILES_SAME_DIR", 10),
		ContextDepth:    getEnvInt("MINDNEST_CONTEXT_DEPTH", 3),
	}

	// GitHub Configuration
	cfg.GitHub = GitHubConfig{
		Token:          getEnvString("MINDNEST_GITHUB_TOKEN", ""),
		APIURL:         getEnvString("MINDNEST_GITHUB_API_URL", "https://api.github.com"),
		RequestTimeout: getEnvDuration("MINDNEST_GITHUB_REQUEST_TIMEOUT", 30*time.Second),
		Concurrency:    getEnvInt("MINDNEST_GITHUB_CONCURRENCY", 5),
	}

	// Workspace Configuration
	cfg.Workspace = WorkspaceConfig{
		AutoCreate: getEnvBool("MINDNEST_WORKSPACE_AUTO_CREATE", true),
	}

	// Database Configuration
	cfg.Database = DatabaseConfig{
		Path:            getEnvString("MINDNEST_DB_PATH", cfg.Database.Path),
		BusyTimeout:     getEnvInt("MINDNEST_DB_BUSY_TIMEOUT", 5000),
		JournalMode:     getEnvString("MINDNEST_DB_JOURNAL_MODE", "WAL"),
		SynchronousMode: getEnvString("MINDNEST_DB_SYNCHRONOUS_MODE", "NORMAL"),
		CacheSize:       getEnvInt("MINDNEST_DB_CACHE_SIZE", -64000), // ~64MB
		ForeignKeys:     getEnvBool("MINDNEST_DB_FOREIGN_KEYS", true),
		ConnMaxLife:     getEnvDuration("MINDNEST_DB_CONN_MAX_LIFE", 5*time.Minute),
		QueryTimeout:    getEnvDuration("MINDNEST_DB_QUERY_TIMEOUT", 30*time.Second),
		MigrationsPath:  getEnvString("MINDNEST_DB_MIGRATIONS_PATH", filepath.Join(appDir, "migrations")),
	}

	// Logging Configuration
	cfg.Logging = LoggingConfig{
		Level:      getEnvString("MINDNEST_LOG_LEVEL", "info"),
		Format:     getEnvString("MINDNEST_LOG_FORMAT", "text"),
		Output:     getEnvString("MINDNEST_LOG_OUTPUT", "stdout"),
		AddSource:  getEnvBool("MINDNEST_LOG_ADD_SOURCE", true),
		TimeFormat: getEnvString("MINDNEST_LOG_TIME_FORMAT", time.RFC3339),
	}

	// Server Configuration
	cfg.Server = ServerConfig{
		Enabled:    getEnvBool("MINDNEST_SERVER_ENABLED", true),
		URL:        getEnvString("MINDNEST_SERVER_URL", "http://localhost:3000"),
		Token:      getEnvString("MINDNEST_SERVER_TOKEN", ""),
		Timeout:    getEnvDuration("MINDNEST_SERVER_TIMEOUT", 30*time.Second),
		DeviceName: getEnvString("MINDNEST_SERVER_DEVICE_NAME", ""),
	}

	// Validate the configuration
	return cfg, cfg.Validate()
}
