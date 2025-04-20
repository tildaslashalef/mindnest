package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// Global configuration instance
	globalConfig *Config
	configMutex  sync.RWMutex
)

// Get returns the global configuration instance
// If the configuration has not been initialized, it will return an error
func Get() (*Config, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	if globalConfig == nil {
		return nil, fmt.Errorf("configuration not initialized")
	}

	return globalConfig, nil
}

// Set sets the global configuration instance
func Set(cfg *Config) {
	configMutex.Lock()
	defer configMutex.Unlock()

	globalConfig = cfg
}

// Config represents the complete application configuration
type Config struct {
	DefaultLLMProvider string // Which provider to use by default (ollama, claude, or gemini)
	Ollama             OllamaConfig
	Claude             ClaudeConfig
	Gemini             GeminiConfig
	RAG                RAGConfig // Retrieval-augmented generation configuration
	GitHub             GitHubConfig
	Workspace          WorkspaceConfig
	Database           DatabaseConfig
	Logging            LoggingConfig
	Server             ServerConfig
	configDir          string // Internal: Directory where config was loaded from
}

// RAGConfig represents retrieval-augmented generation configuration
type RAGConfig struct {
	NSimilarChunks  int // Number of similar chunks to retrieve
	BatchSize       int // Number of chunks to process in each batch
	MaxFilesSameDir int // Maximum number of files to include from the same directory for context
	ContextDepth    int // How deep to search in the directory hierarchy for context

	// Vector operation configurations
	DefaultMetric     string  // Default distance metric (cosine, l2, dot, hamming)
	Normalization     bool    // Whether to normalize vectors by default (true recommended)
	MinSimilarity     float64 // Default minimum similarity threshold (0.0-1.0)
	VectorType        string  // Default vector compression type (float32, int8, binary)
	EnableCompression bool    // Whether to use compression by default
}

// GitHubConfig represents GitHub-specific configuration
type GitHubConfig struct {
	Token          string        // GitHub Personal Access Token
	APIURL         string        // GitHub API base URL
	RequestTimeout time.Duration // Request timeout for GitHub API
	Concurrency    int           // Number of concurrent API requests
}

// WorkspaceConfig represents workspace handling configuration
type WorkspaceConfig struct {
	AutoCreate bool // Whether to automatically create workspaces
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Path            string        // Path to the SQLite database file
	JournalMode     string        // Journal mode (WAL recommended)
	SynchronousMode string        // Synchronous mode
	BusyTimeout     int           // Busy timeout in milliseconds
	CacheSize       int           // Cache size in KiB
	ForeignKeys     bool          // Whether to enforce foreign key constraints
	ConnMaxLife     time.Duration // Maximum connection lifetime
	QueryTimeout    time.Duration // Query timeout
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level      string // debug, info, warn, error
	Format     string // text or json
	Output     string // stdout, stderr, or file path
	AddSource  bool   // Include source code position in logs
	TimeFormat string // Time format for logs (empty uses RFC3339)
}

// OllamaConfig holds configuration specific to the Ollama client
type OllamaConfig struct {
	// Connection settings
	Endpoint            string        // Ollama API endpoint URL
	MaxIdleConns        int           // Maximum number of idle connections
	MaxIdleConnsPerHost int           // Maximum number of idle connections per host
	IdleConnTimeout     time.Duration // How long to keep idle connections alive

	// Model settings
	Model          string // Default model to use
	EmbeddingModel string // Default embedding model to use

	// Request settings
	Timeout    time.Duration // Request timeout
	MaxRetries int           // Maximum number of retries on failure

	// Generation parameters
	MaxTokens   int     // Max tokens to generate for responses
	Temperature float64 // Default temperature for generation

	// Rate limiting
	RequestsPerMinute int // Added for rate limiting
	BurstLimit        int // Added for rate limiting
}

// ClaudeConfig holds Claude API configuration
type ClaudeConfig struct {
	// Authentication and connection
	APIKey     string   // Claude API key
	BaseURL    string   // Claude API base URL
	APIVersion string   // API version to use
	UseAPIBeta bool     // Whether to use API beta features
	APIBeta    []string // Beta features to enable

	// Model settings
	Model          string // Claude model to use
	EmbeddingModel string // Claude embedding model to use (when Claude adds embedding support)

	// Request settings
	Timeout    time.Duration // Request timeout
	MaxRetries int           // Maximum number of retries on failure

	// Generation parameters
	MaxTokens   int     // Max tokens to generate for Claude responses
	Temperature float64 // Default temperature for Claude
	TopP        float64 // Top-p sampling parameter
	TopK        int     // Top-k sampling parameter

	// Stop sequence settings
	UseStopSequences bool     // Whether to use stop sequences
	StopSequences    []string // Custom stop sequences

	// Rate limiting
	RequestsPerMinute int // Added for rate limiting
	BurstLimit        int // Added for rate limiting
}

// GeminiConfig holds Gemini API configuration
type GeminiConfig struct {
	// Authentication and connection
	APIKey  string // Gemini API key
	BaseURL string // Gemini API base URL

	// API version settings
	APIVersion       string // API version for chat models (v1 or v1beta)
	EmbeddingVersion string // API version for embedding models (v1 or v1beta)

	// Model settings
	Model          string // Gemini model to use
	EmbeddingModel string // Gemini embedding model to use

	// Request settings
	Timeout    time.Duration // Request timeout
	MaxRetries int           // Maximum number of retries on failure

	// Generation parameters
	MaxTokens   int     // Max tokens to generate for Gemini responses
	Temperature float64 // Default temperature for Gemini
	TopP        float64 // Top-p sampling parameter
	TopK        int     // Top-k sampling parameter

	// Rate limiting
	RequestsPerMinute int // Added for rate limiting
	BurstLimit        int // Added for rate limiting
}

// ServerConfig holds configuration for the sync server
type ServerConfig struct {
	Enabled    bool          // Whether the server is enabled
	URL        string        // Server URL
	Token      string        // Authentication token
	Timeout    time.Duration // Request timeout
	DeviceName string        // Device name for identification
}

// New returns a new empty Config
func New() *Config {
	return &Config{
		DefaultLLMProvider: "",
		Ollama:             OllamaConfig{},
		Claude:             ClaudeConfig{},
		Gemini:             GeminiConfig{},
		RAG:                RAGConfig{},
		GitHub:             GitHubConfig{},
		Workspace:          WorkspaceConfig{},
		Database:           DatabaseConfig{},
		Logging:            LoggingConfig{},
		Server:             ServerConfig{},
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if err := c.validateLLM(); err != nil {
		return fmt.Errorf("LLM config: %w", err)
	}

	if err := c.validateOllama(); err != nil {
		return fmt.Errorf("Ollama config: %w", err)
	}

	if err := c.validateGemini(); err != nil {
		return fmt.Errorf("Gemini config: %w", err)
	}

	if err := c.validateRAG(); err != nil {
		return fmt.Errorf("RAG config: %w", err)
	}

	if err := c.validateDatabase(); err != nil {
		return fmt.Errorf("database config: %w", err)
	}

	if err := c.validateLogging(); err != nil {
		return fmt.Errorf("logging config: %w", err)
	}

	return nil
}

// ParseLogLevel parses a log level string to a slog.Level
func ParseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "none":
		// Set to a very high level that won't be triggered
		return slog.Level(9999)
	default:
		return slog.LevelInfo
	}
}

func (c *Config) validateLLM() error {
	if c.DefaultLLMProvider == "" {
		return fmt.Errorf("default provider cannot be empty")
	}
	return nil
}

func (c *Config) validateOllama() error {
	if c.Ollama.Endpoint == "" {
		return fmt.Errorf("endpoint cannot be empty")
	}

	if c.Ollama.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if c.Ollama.MaxRetries <= 0 {
		return fmt.Errorf("max_retries must be positive")
	}

	if c.Ollama.Model == "" {
		return fmt.Errorf("model cannot be empty")
	}

	if c.Ollama.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive")
	}

	if c.Ollama.Temperature <= 0 {
		return fmt.Errorf("temperature must be positive")
	}

	if c.Ollama.MaxIdleConns <= 0 {
		return fmt.Errorf("max_idle_conns must be positive")
	}

	if c.Ollama.MaxIdleConnsPerHost <= 0 {
		return fmt.Errorf("max_idle_conns_per_host must be positive")
	}

	if c.Ollama.IdleConnTimeout <= 0 {
		return fmt.Errorf("idle_conn_timeout must be positive")
	}

	return nil
}

func (c *Config) validateGemini() error {
	// If API key is not provided, return early
	if c.Gemini.APIKey == "" {
		return nil
	}

	// Set base URL default if not provided
	if c.Gemini.BaseURL == "" {
		c.Gemini.BaseURL = "https://generativelanguage.googleapis.com"
	}

	// Set API version defaults if not provided
	if c.Gemini.APIVersion == "" {
		c.Gemini.APIVersion = "v1beta"
	}

	if c.Gemini.EmbeddingVersion == "" {
		c.Gemini.EmbeddingVersion = "v1beta"
	}

	// Validate API versions are v1 or v1beta
	if c.Gemini.APIVersion != "v1" && c.Gemini.APIVersion != "v1beta" {
		return fmt.Errorf("invalid API version: %s (must be v1 or v1beta)", c.Gemini.APIVersion)
	}

	if c.Gemini.EmbeddingVersion != "v1" && c.Gemini.EmbeddingVersion != "v1beta" {
		return fmt.Errorf("invalid embedding API version: %s (must be v1 or v1beta)", c.Gemini.EmbeddingVersion)
	}

	// Set default model if not provided
	if c.Gemini.Model == "" {
		c.Gemini.Model = "gemini-2.5-pro"
	}

	// Set default embedding model if not provided
	if c.Gemini.EmbeddingModel == "" {
		c.Gemini.EmbeddingModel = "gemini-embedding-exp-03-07"
	}

	// Set default timeout if not provided
	if c.Gemini.Timeout == 0 {
		c.Gemini.Timeout = 30 * time.Second
	}

	// Set default max retries if not provided
	if c.Gemini.MaxRetries <= 0 {
		c.Gemini.MaxRetries = 3
	}

	// Set default max tokens if not provided
	if c.Gemini.MaxTokens <= 0 {
		c.Gemini.MaxTokens = 8192
	}

	return nil
}

func (c *Config) validateRAG() error {
	if c.RAG.NSimilarChunks <= 0 {
		return fmt.Errorf("number of similar chunks must be positive")
	}

	if c.RAG.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}

	if c.RAG.MaxFilesSameDir <= 0 {
		return fmt.Errorf("max files in same directory must be positive")
	}

	if c.RAG.ContextDepth <= 0 {
		return fmt.Errorf("context depth must be positive")
	}

	return nil
}

func (c *Config) validateDatabase() error {
	if c.Database.Path == "" {
		return fmt.Errorf("database path cannot be empty")
	}

	// Create the directory if it doesn't exist
	dir := filepath.Dir(c.Database.Path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for database: %w", err)
		}
	}

	// Check if directory is writable
	if err := checkDirectoryWritable(dir); err != nil {
		return fmt.Errorf("database directory: %w", err)
	}

	if c.Database.BusyTimeout <= 0 {
		return fmt.Errorf("busy timeout must be positive")
	}

	if c.Database.ConnMaxLife <= 0 {
		return fmt.Errorf("connection max life must be positive")
	}

	if c.Database.QueryTimeout <= 0 {
		return fmt.Errorf("query timeout must be positive")
	}

	return nil
}

func (c *Config) validateLogging() error {
	// Validate logging level
	level := strings.ToLower(c.Logging.Level)
	if level != "debug" && level != "info" && level != "warn" && level != "error" && level != "none" {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	// Validate format
	format := strings.ToLower(c.Logging.Format)
	if format != "text" && format != "json" {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}

	return nil
}

// getEnvString returns a string from the environment variable
func getEnvString(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvInt returns an int from the environment variable
func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvInt64 returns an int64 from the environment variable
func getEnvInt64(key string, defaultValue int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool returns a bool from the environment variable
func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getEnvDuration returns a time.Duration from the environment variable
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// getEnvFloat returns a float64 from the environment variable
func getEnvFloat(key string, defaultValue float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// getTimeFormat converts a named time format to its actual format string
func getTimeFormat(name string) string {
	switch name {
	case "RFC3339":
		return time.RFC3339
	case "RFC3339Nano":
		return time.RFC3339Nano
	case "RFC822":
		return time.RFC822
	case "RFC822Z":
		return time.RFC822Z
	case "RFC850":
		return time.RFC850
	case "RFC1123":
		return time.RFC1123
	case "RFC1123Z":
		return time.RFC1123Z
	case "Kitchen":
		return time.Kitchen
	case "Stamp":
		return time.Stamp
	case "StampMilli":
		return time.StampMilli
	case "StampMicro":
		return time.StampMicro
	case "StampNano":
		return time.StampNano
	case "DateTime":
		return "2006-01-02 15:04:05"
	case "DateTimeMS":
		return "2006-01-02 15:04:05.000"
	case "Date":
		return "2006-01-02"
	case "Time":
		return "15:04:05"
	default:
		return name
	}
}

// checkDirectoryWritable tests if a directory is writable
func checkDirectoryWritable(dir string) error {
	// Create a temporary file to test write permissions
	testFile := filepath.Join(dir, fmt.Sprintf("test_write_%d", time.Now().UnixNano()))
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("directory not writable: %w", err)
	}

	// Clean up
	f.Close()
	os.Remove(testFile)

	return nil
}
