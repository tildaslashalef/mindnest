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
	LLM       LLMConfig       `yaml:"llm"`
	Ollama    OllamaConfig    `yaml:"ollama"`
	Claude    ClaudeConfig    `yaml:"claude"`
	Embedding EmbeddingConfig `yaml:"embedding"`
	Context   ContextConfig   `yaml:"context"`
	GitHub    GitHubConfig    `yaml:"github"`
	Workspace WorkspaceConfig `yaml:"workspace"`
	Database  DatabaseConfig  `yaml:"database"`
	Logging   LoggingConfig   `yaml:"logging"`
	Server    ServerConfig    `yaml:"server"`
}

// EmbeddingConfig represents embedding-specific configuration
type EmbeddingConfig struct {
	Model          string        `yaml:"model"`
	NSimilarChunks int           `yaml:"n_similar_chunks"`
	Timeout        time.Duration `yaml:"timeout"`
	ExecPath       string        `yaml:"exec_path"`
	BatchSize      int           `yaml:"batch_size"`
}

// ContextConfig represents context retrieval configuration
type ContextConfig struct {
	MaxFilesSameDir int `yaml:"max_files_same_directory"`
	ContextDepth    int `yaml:"context_depth"`
}

// GitHubConfig represents GitHub-specific configuration
type GitHubConfig struct {
	Token          string        `yaml:"token" json:"token"`                     // GitHub Personal Access Token
	APIURL         string        `yaml:"api_url" json:"api_url"`                 // GitHub API base URL
	RequestTimeout time.Duration `yaml:"request_timeout" json:"request_timeout"` // Request timeout for GitHub API
	Concurrency    int           `yaml:"concurrency" json:"concurrency"`         // Number of concurrent API requests
}

// WorkspaceConfig represents workspace handling configuration
type WorkspaceConfig struct {
	AutoCreate bool `yaml:"auto_create"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Path            string        `yaml:"path"`
	BusyTimeout     int           `yaml:"busy_timeout"`
	JournalMode     string        `yaml:"journal_mode"`
	SynchronousMode string        `yaml:"synchronous_mode"`
	CacheSize       int           `yaml:"cache_size"`
	ForeignKeys     bool          `yaml:"foreign_keys"`
	ConnMaxLife     time.Duration `yaml:"conn_max_life"`
	QueryTimeout    time.Duration `yaml:"query_timeout"`
	MigrationsPath  string        `yaml:"migrations_path"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level      string `yaml:"level"`       // debug, info, warn, error
	Format     string `yaml:"format"`      // text or json
	Output     string `yaml:"output"`      // stdout, stderr, or file path
	AddSource  bool   `yaml:"add_source"`  // Include source code position in logs
	TimeFormat string `yaml:"time_format"` // Time format for logs (empty uses RFC3339)
}

// OllamaConfig holds configuration specific to the Ollama client
type OllamaConfig struct {
	Endpoint            string        `yaml:"endpoint" json:"endpoint"`                               // Ollama API endpoint URL
	Timeout             time.Duration `yaml:"timeout" json:"timeout"`                                 // Request timeout
	MaxRetries          int           `yaml:"max_retries" json:"max_retries"`                         // Maximum number of retries on failure
	DefaultModel        string        `yaml:"default_model" json:"default_model"`                     // Default model to use if none specified
	MaxIdleConns        int           `yaml:"max_idle_conns" json:"max_idle_conns"`                   // Maximum number of idle connections
	MaxIdleConnsPerHost int           `yaml:"max_idle_conns_per_host" json:"max_idle_conns_per_host"` // Maximum number of idle connections per host
	IdleConnTimeout     time.Duration `yaml:"idle_conn_timeout" json:"idle_conn_timeout"`             // How long to keep idle connections alive
}

// ClaudeConfig holds Claude API configuration
type ClaudeConfig struct {
	APIKey           string        `yaml:"api_key" json:"api_key"`                       // Claude API key
	BaseURL          string        `yaml:"base_url" json:"base_url"`                     // Claude API base URL
	Model            string        `yaml:"model" json:"model"`                           // Claude model to use
	Timeout          time.Duration `yaml:"timeout" json:"timeout"`                       // Request timeout
	MaxRetries       int           `yaml:"max_retries" json:"max_retries"`               // Maximum number of retries on failure
	MaxTokens        int           `yaml:"max_tokens" json:"max_tokens"`                 // Max tokens to generate for Claude responses
	Temperature      float64       `yaml:"temperature" json:"temperature"`               // Default temperature for Claude
	TopP             float64       `yaml:"top_p" json:"top_p"`                           // Top-p sampling parameter
	TopK             int           `yaml:"top_k" json:"top_k"`                           // Top-k sampling parameter
	APIVersion       string        `yaml:"api_version" json:"api_version"`               // API version to use
	APIBeta          []string      `yaml:"api_beta" json:"api_beta"`                     // Beta features to enable
	UseStopSequences bool          `yaml:"use_stop_sequences" json:"use_stop_sequences"` // Whether to use stop sequences
	StopSequences    []string      `yaml:"stop_sequences" json:"stop_sequences"`         // Custom stop sequences
}

// LLMConfig holds general LLM configuration that applies across providers
type LLMConfig struct {
	DefaultProvider string  `yaml:"default_provider" json:"default_provider"` // Which provider to use by default (ollama or claude)
	DefaultModel    string  `yaml:"default_model" json:"default_model"`       // Generic default model name (will be overridden by provider-specific defaults)
	MaxTokens       int     `yaml:"max_tokens" json:"max_tokens"`             // Default max tokens for generation
	Temperature     float64 `yaml:"temperature" json:"temperature"`           // Default temperature for generation
}

// ServerConfig holds configuration for the sync server
type ServerConfig struct {
	Enabled    bool          `yaml:"enabled"`
	URL        string        `yaml:"url"`
	Token      string        `yaml:"token"`
	Timeout    time.Duration `yaml:"timeout"`
	DeviceName string        `yaml:"device_name"`
}

// New returns a new empty Config
func New() *Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	configDir := filepath.Join(homeDir, ".mindnest")

	return &Config{
		LLM:       LLMConfig{},
		Ollama:    OllamaConfig{},
		Claude:    ClaudeConfig{},
		Embedding: EmbeddingConfig{},
		Context:   ContextConfig{},
		GitHub:    GitHubConfig{},
		Workspace: WorkspaceConfig{},
		Database: DatabaseConfig{
			Path: filepath.Join(configDir, "mindnest.db"),
		},
		Logging: LoggingConfig{},
		Server:  ServerConfig{},
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

	if err := c.validateEmbedding(); err != nil {
		return fmt.Errorf("embedding config: %w", err)
	}

	if err := c.validateContext(); err != nil {
		return fmt.Errorf("context config: %w", err)
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
	if c.LLM.DefaultProvider == "" {
		return fmt.Errorf("default provider cannot be empty")
	}

	if c.LLM.DefaultModel == "" {
		return fmt.Errorf("default model cannot be empty")
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

	if c.Ollama.DefaultModel == "" {
		return fmt.Errorf("default_model cannot be empty")
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

func (c *Config) validateEmbedding() error {
	if c.Embedding.Model == "" {
		return fmt.Errorf("model cannot be empty")
	}

	if c.Embedding.NSimilarChunks <= 0 {
		return fmt.Errorf("number of similar chunks must be positive")
	}

	return nil
}

func (c *Config) validateContext() error {
	if c.Context.MaxFilesSameDir <= 0 {
		return fmt.Errorf("max files in same directory must be positive")
	}

	if c.Context.ContextDepth <= 0 {
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

	if c.Database.MigrationsPath == "" {
		return fmt.Errorf("migrations path cannot be empty")
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
