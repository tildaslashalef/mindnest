package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// LoadFromEnv loads configuration from environment variables
// Parameters:
// - configDir: Directory containing config files (or empty for default)
// - configFilePath: Path to .env file (or empty for default)
// - isInitializing: Whether this is being called during explicit initialization (e.g., from init command)
func LoadFromEnv(configDir string, configFilePath string, isInitializing bool) (*Config, error) {
	// Load empty configuration
	cfg := New()

	// If configDir is empty, use the default
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		configDir = filepath.Join(homeDir, ".mindnest")

		// Create directory if it doesn't exist, but only do minimal setup if not initializing
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}

	}

	// Default database path is in the config directory
	cfg.Database.Path = filepath.Join(configDir, "mindnest.db")

	// Default log path is in the config directory
	defaultLogPath := filepath.Join(configDir, "mindnest.log")

	// Use provided config file path or default
	if configFilePath == "" {
		configFilePath = filepath.Join(configDir, ".env")
	}

	// Check if ENV_FILE_PATH is set to load from a custom .env file
	envFilePath := getEnvString("ENV_FILE_PATH", "")
	if envFilePath != "" {
		// User specified a custom env file path
		err := godotenv.Load(envFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load env file from %s: %w", envFilePath, err)
		}
	} else {
		// Try to load from config directory first
		err := godotenv.Load(configFilePath)
		if err != nil {
			// Then try current directory as fallback
			_ = godotenv.Load() // Ignore errors if file doesn't exist
		}
	}

	// LLM Configuration
	cfg.DefaultLLMProvider = getEnvString("MINDNEST_LLM_DEFAULT_PROVIDER", "ollama")

	// Load Ollama Configuration
	cfg.Ollama = OllamaConfig{
		Endpoint:            getEnvString("MINDNEST_OLLAMA_ENDPOINT", "http://localhost:11434"),
		Timeout:             getEnvDuration("MINDNEST_OLLAMA_TIMEOUT", 600*time.Second),
		MaxRetries:          getEnvInt("MINDNEST_OLLAMA_MAX_RETRIES", 3),
		Model:               getEnvString("MINDNEST_OLLAMA_MODEL", "gemma3"),
		EmbeddingModel:      getEnvString("MINDNEST_OLLAMA_EMBEDDING_MODEL", "nomic-embed-text"),
		MaxTokens:           getEnvInt("MINDNEST_OLLAMA_MAX_TOKENS", 2048),
		Temperature:         getEnvFloat("MINDNEST_OLLAMA_TEMPERATURE", 0.7),
		MaxIdleConns:        getEnvInt("MINDNEST_OLLAMA_MAX_IDLE_CONNS", 100),
		MaxIdleConnsPerHost: getEnvInt("MINDNEST_OLLAMA_MAX_IDLE_CONNS_PER_HOST", 100),
		IdleConnTimeout:     getEnvDuration("MINDNEST_OLLAMA_IDLE_CONN_TIMEOUT", 120*time.Second),
	}

	// Load Claude config
	rawApiBeta := strings.Split(getEnvString("MINDNEST_CLAUDE_API_BETA", ""), ",")
	var apiBeta []string
	for _, beta := range rawApiBeta {
		beta = strings.TrimSpace(beta)
		if beta != "" && !strings.HasPrefix(beta, "#") {
			apiBeta = append(apiBeta, beta)
		}
	}

	cfg.Claude = ClaudeConfig{
		APIKey:           getEnvString("MINDNEST_CLAUDE_API_KEY", ""),
		BaseURL:          getEnvString("MINDNEST_CLAUDE_BASE_URL", "https://api.anthropic.com"),
		Model:            getEnvString("MINDNEST_CLAUDE_MODEL", "claude-3-7-sonnet-20250219"),
		EmbeddingModel:   getEnvString("MINDNEST_CLAUDE_EMBEDDING_MODEL", "ollama"),
		Timeout:          getEnvDuration("MINDNEST_CLAUDE_TIMEOUT", 60*time.Second),
		MaxRetries:       getEnvInt("MINDNEST_CLAUDE_MAX_RETRIES", 3),
		MaxTokens:        getEnvInt("MINDNEST_CLAUDE_MAX_TOKENS", 4096),
		Temperature:      getEnvFloat("MINDNEST_CLAUDE_TEMPERATURE", 0.1),
		TopP:             getEnvFloat("MINDNEST_CLAUDE_TOP_P", 0.9),
		TopK:             getEnvInt("MINDNEST_CLAUDE_TOP_K", 40),
		APIVersion:       getEnvString("MINDNEST_CLAUDE_API_VERSION", "2023-06-01"),
		UseAPIBeta:       getEnvBool("MINDNEST_CLAUDE_USE_API_BETA", false),
		APIBeta:          apiBeta,
		UseStopSequences: getEnvBool("MINDNEST_CLAUDE_USE_STOP_SEQUENCES", false),
		StopSequences:    strings.Split(getEnvString("MINDNEST_CLAUDE_STOP_SEQUENCES", ""), ","),
	}

	// Load Gemini configuration
	cfg.Gemini = GeminiConfig{
		APIKey:           getEnvString("MINDNEST_GEMINI_API_KEY", ""),
		BaseURL:          getEnvString("MINDNEST_GEMINI_BASE_URL", "https://generativelanguage.googleapis.com"),
		APIVersion:       getEnvString("MINDNEST_GEMINI_API_VERSION", "v1beta"),
		EmbeddingVersion: getEnvString("MINDNEST_GEMINI_EMBEDDING_VERSION", "v1beta"),
		Model:            getEnvString("MINDNEST_GEMINI_MODEL", "gemini-2.5-pro-exp-03-25"),
		EmbeddingModel:   getEnvString("MINDNEST_GEMINI_EMBEDDING_MODEL", "gemini-embedding-exp-03-07"),
		Timeout:          getEnvDuration("MINDNEST_GEMINI_TIMEOUT", 60*time.Second),
		MaxRetries:       getEnvInt("MINDNEST_GEMINI_MAX_RETRIES", 3),
		MaxTokens:        getEnvInt("MINDNEST_GEMINI_MAX_TOKENS", 4096),
		Temperature:      getEnvFloat("MINDNEST_GEMINI_TEMPERATURE", 0.1),
		TopP:             getEnvFloat("MINDNEST_GEMINI_TOP_P", 0.9),
		TopK:             getEnvInt("MINDNEST_GEMINI_TOP_K", 40),
	}

	// RAG Configuration (formerly Embedding config)
	cfg.RAG = RAGConfig{
		NSimilarChunks:  getEnvInt("MINDNEST_RAG_N_SIMILAR_CHUNKS", 5),
		BatchSize:       getEnvInt("MINDNEST_RAG_BATCH_SIZE", 20),
		MaxFilesSameDir: getEnvInt("MINDNEST_CONTEXT_MAX_FILES_SAME_DIR", 10),
		ContextDepth:    getEnvInt("MINDNEST_CONTEXT_CONTEXT_DEPTH", 3),

		// Vector operation configurations
		DefaultMetric:     getEnvString("MINDNEST_RAG_DEFAULT_METRIC", "cosine"), // NOTE: Only cosine distance is supported by the vector_index table.
		Normalization:     getEnvBool("MINDNEST_RAG_NORMALIZATION", true),
		MinSimilarity:     getEnvFloat("MINDNEST_RAG_MIN_SIMILARITY", 0.0),
		VectorType:        getEnvString("MINDNEST_RAG_VECTOR_TYPE", "float32"),
		EnableCompression: getEnvBool("MINDNEST_RAG_ENABLE_COMPRESSION", false),
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
	}

	// Logging Configuration
	cfg.Logging = LoggingConfig{
		Level:      getEnvString("MINDNEST_LOG_LEVEL", "info"),
		Format:     getEnvString("MINDNEST_LOG_FORMAT", "text"),
		Output:     getEnvString("MINDNEST_LOG_OUTPUT", defaultLogPath),
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
