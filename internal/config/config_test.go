package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetEnvFloat(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue float64
		expected     float64
	}{
		{
			name:         "env not set, return default",
			envValue:     "",
			defaultValue: 0.2,
			expected:     0.2,
		},
		{
			name:         "env set to 0.1, return 0.1",
			envValue:     "0.1",
			defaultValue: 0.2,
			expected:     0.1,
		},
		{
			name:         "env set to 0.7, return 0.7 (not 0.7000000000001)",
			envValue:     "0.7",
			defaultValue: 0.2,
			expected:     0.7,
		},
		{
			name:         "env set to invalid value, return default",
			envValue:     "invalid",
			defaultValue: 0.2,
			expected:     0.2,
		},
		{
			name:         "env set to precise value, maintain precision",
			envValue:     "0.123456789",
			defaultValue: 0.2,
			expected:     0.123456789,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variable for the test
			key := "TEST_FLOAT_VALUE"
			if tt.envValue != "" {
				os.Setenv(key, tt.envValue)
				defer os.Unsetenv(key)
			} else {
				os.Unsetenv(key)
			}

			// Call the function
			result := getEnvFloat(key, tt.defaultValue)

			// Verify the result
			assert.Equal(t, tt.expected, result, "getEnvFloat should return the expected value with correct precision")
		})
	}
}

func TestGetEnvString(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue string
		expected     string
	}{
		{
			name:         "env not set, return default",
			envValue:     "",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "env set, return env value",
			envValue:     "custom",
			defaultValue: "default",
			expected:     "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_STRING_VALUE"
			if tt.envValue != "" {
				os.Setenv(key, tt.envValue)
				defer os.Unsetenv(key)
			} else {
				os.Unsetenv(key)
			}

			result := getEnvString(key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue int
		expected     int
	}{
		{
			name:         "env not set, return default",
			envValue:     "",
			defaultValue: 100,
			expected:     100,
		},
		{
			name:         "env set to valid int, return int value",
			envValue:     "200",
			defaultValue: 100,
			expected:     200,
		},
		{
			name:         "env set to invalid int, return default",
			envValue:     "not_an_int",
			defaultValue: 100,
			expected:     100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_INT_VALUE"
			if tt.envValue != "" {
				os.Setenv(key, tt.envValue)
				defer os.Unsetenv(key)
			} else {
				os.Unsetenv(key)
			}

			result := getEnvInt(key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue bool
		expected     bool
	}{
		{
			name:         "env not set, return default",
			envValue:     "",
			defaultValue: true,
			expected:     true,
		},
		{
			name:         "env set to true, return true",
			envValue:     "true",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "env set to false, return false",
			envValue:     "false",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "env set to invalid bool, return default",
			envValue:     "not_a_bool",
			defaultValue: true,
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_BOOL_VALUE"
			if tt.envValue != "" {
				os.Setenv(key, tt.envValue)
				defer os.Unsetenv(key)
			} else {
				os.Unsetenv(key)
			}

			result := getEnvBool(key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnvDuration(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue time.Duration
		expected     time.Duration
	}{
		{
			name:         "env not set, return default",
			envValue:     "",
			defaultValue: 1 * time.Second,
			expected:     1 * time.Second,
		},
		{
			name:         "env set to valid duration, return duration value",
			envValue:     "5s",
			defaultValue: 1 * time.Second,
			expected:     5 * time.Second,
		},
		{
			name:         "env set to invalid duration, return default",
			envValue:     "not_a_duration",
			defaultValue: 1 * time.Second,
			expected:     1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_DURATION_VALUE"
			if tt.envValue != "" {
				os.Setenv(key, tt.envValue)
				defer os.Unsetenv(key)
			} else {
				os.Unsetenv(key)
			}

			result := getEnvDuration(key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNew(t *testing.T) {
	// New should return a bare-bones config with minimal fields set
	cfg := New()

	// Database path should not be set in New() anymore - it's set in LoadFromEnv()
	assert.Empty(t, cfg.Database.Path, "Database path should be empty")

	// All other fields should be at zero values
	assert.Empty(t, cfg.LLM.DefaultProvider)
	assert.Empty(t, cfg.LLM.DefaultModel)
	assert.Zero(t, cfg.LLM.MaxTokens)
	assert.Zero(t, cfg.LLM.Temperature)
	assert.Empty(t, cfg.Ollama.Endpoint)
	assert.Zero(t, cfg.Ollama.Timeout)
	assert.Empty(t, cfg.Logging.Level)
	assert.False(t, cfg.Workspace.AutoCreate)
}

func TestLoadFromEnv(t *testing.T) {
	// Reset any environment variables that might affect the test
	vars := []string{
		"LLM_DEFAULT_PROVIDER", "LLM_DEFAULT_MODEL", "LLM_MAX_TOKENS", "LLM_TEMPERATURE",
		"OLLAMA_ENDPOINT", "OLLAMA_TIMEOUT", "OLLAMA_MAX_RETRIES", "OLLAMA_DEFAULT_MODEL",
		"MINDNEST_LOG_LEVEL", "MINDNEST_WORKSPACE_AUTO_CREATE",
	}

	for _, v := range vars {
		os.Unsetenv(v)
	}

	// Load config with defaults
	cfg, err := LoadFromEnv("", "", false)
	assert.NoError(t, err)

	// Verify default values are set correctly
	assert.Equal(t, "claude", cfg.LLM.DefaultProvider)
	assert.Equal(t, "claude-3-7-sonnet-20250219", cfg.LLM.DefaultModel)
	assert.Equal(t, 4096, cfg.LLM.MaxTokens)
	assert.Equal(t, 0.1, cfg.LLM.Temperature, "Temperature precision should be exactly 0.1")

	// Verify Ollama config
	assert.Equal(t, "http://localhost:11434", cfg.Ollama.Endpoint)
	assert.Equal(t, 5*time.Minute, cfg.Ollama.Timeout)
	assert.Equal(t, 3, cfg.Ollama.MaxRetries)
	assert.Equal(t, "gemma3", cfg.Ollama.DefaultModel) // Updated to match current default
	assert.Equal(t, 100, cfg.Ollama.MaxIdleConns)
	assert.Equal(t, 100, cfg.Ollama.MaxIdleConnsPerHost)
	assert.Equal(t, 90*time.Second, cfg.Ollama.IdleConnTimeout)

	// Other config fields
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, true, cfg.Workspace.AutoCreate)
}

func TestSetGet(t *testing.T) {
	// Clear the global config first
	Set(nil)

	// Get should return error when not initialized
	_, err := Get()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Set a config
	testCfg := New()
	testCfg.LLM.Temperature = 0.5 // Change a value
	Set(testCfg)

	// Get should work now
	cfg, err := Get()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify the changed value
	assert.Equal(t, 0.5, cfg.LLM.Temperature)
}

func TestValidate(t *testing.T) {
	// Valid config from LoadFromEnv should pass validation
	cfg, err := LoadFromEnv("", "", false)
	assert.NoError(t, err)
	err = cfg.Validate()
	assert.NoError(t, err)

	// Invalid LLM config
	invalidLLM := New()
	invalidLLM.LLM.DefaultProvider = ""
	invalidLLM.LLM.DefaultModel = ""
	err = invalidLLM.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LLM config")

	// Invalid Ollama config
	invalidOllama := New()
	// Set required LLM values to pass LLM validation
	invalidOllama.LLM.DefaultProvider = "ollama"
	invalidOllama.LLM.DefaultModel = "model"
	// Set all required Ollama values except MaxRetries
	invalidOllama.Ollama.Endpoint = "http://localhost:11434"
	invalidOllama.Ollama.Timeout = 5 * time.Minute
	invalidOllama.Ollama.MaxRetries = 0 // Invalid value
	invalidOllama.Ollama.DefaultModel = "model"
	invalidOllama.Ollama.MaxIdleConns = 100
	invalidOllama.Ollama.MaxIdleConnsPerHost = 100
	invalidOllama.Ollama.IdleConnTimeout = 90 * time.Second

	err = invalidOllama.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Ollama config")

	// Invalid Embedding config
	invalidEmbedding := New()
	// Set required LLM values
	invalidEmbedding.LLM.DefaultProvider = "ollama"
	invalidEmbedding.LLM.DefaultModel = "model"
	// Set required Ollama values
	invalidEmbedding.Ollama.Endpoint = "http://localhost:11434"
	invalidEmbedding.Ollama.Timeout = 5 * time.Minute
	invalidEmbedding.Ollama.MaxRetries = 3
	invalidEmbedding.Ollama.DefaultModel = "model"
	invalidEmbedding.Ollama.MaxIdleConns = 100
	invalidEmbedding.Ollama.MaxIdleConnsPerHost = 100
	invalidEmbedding.Ollama.IdleConnTimeout = 90 * time.Second
	// Set invalid Embedding value
	invalidEmbedding.Embedding.Model = ""
	invalidEmbedding.Embedding.NSimilarChunks = 5

	err = invalidEmbedding.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embedding config")

	// Invalid logging config
	invalidLogging := New()
	// Set required LLM values
	invalidLogging.LLM.DefaultProvider = "ollama"
	invalidLogging.LLM.DefaultModel = "model"
	// Set required Ollama values
	invalidLogging.Ollama.Endpoint = "http://localhost:11434"
	invalidLogging.Ollama.Timeout = 5 * time.Minute
	invalidLogging.Ollama.MaxRetries = 3
	invalidLogging.Ollama.DefaultModel = "model"
	invalidLogging.Ollama.MaxIdleConns = 100
	invalidLogging.Ollama.MaxIdleConnsPerHost = 100
	invalidLogging.Ollama.IdleConnTimeout = 90 * time.Second
	// Set required Embedding values
	invalidLogging.Embedding.Model = "model"
	invalidLogging.Embedding.NSimilarChunks = 5
	// Set required Context values
	invalidLogging.Context.MaxFilesSameDir = 10
	invalidLogging.Context.ContextDepth = 3
	// Set required Database values
	invalidLogging.Database.Path = filepath.Join(os.TempDir(), "test.db")
	invalidLogging.Database.BusyTimeout = 5000
	invalidLogging.Database.ConnMaxLife = 5 * time.Minute
	invalidLogging.Database.QueryTimeout = 30 * time.Second

	// Set invalid Logging value
	invalidLogging.Logging.Level = "invalid"
	invalidLogging.Logging.Format = "text"

	err = invalidLogging.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logging config")
}

func TestParseLoglevel(t *testing.T) {
	tests := []struct {
		level  string
		expect slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"none", slog.Level(9999)},
		{"invalid", slog.LevelInfo}, // Default to info for invalid levels
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			level := ParseLogLevel(tt.level)
			assert.Equal(t, tt.expect, level)
		})
	}
}

func TestCheckDirectoryWritable(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Should be writable
	err := checkDirectoryWritable(tempDir)
	assert.NoError(t, err)

	// Test with non-existent directory
	err = checkDirectoryWritable("/path/that/does/not/exist")
	assert.Error(t, err)
}

func TestTemperaturePrecision(t *testing.T) {
	// Test multiple precision values to ensure they're preserved exactly
	temperatures := []float64{
		0.0,
		0.1,
		0.2,
		0.25,
		0.33,
		0.5,
		0.67,
		0.7,
		0.75,
		0.8,
		0.9,
		1.0,
	}

	for _, temp := range temperatures {
		t.Run(formatFloat(temp), func(t *testing.T) {
			// Set via environment
			os.Setenv("TEST_TEMP", formatFloat(temp))
			defer os.Unsetenv("TEST_TEMP")

			result := getEnvFloat("TEST_TEMP", 0.0)
			assert.Equal(t, temp, result, "Temperature should maintain exact precision")

			// Test in Config
			cfg := New()
			cfg.LLM.Temperature = temp
			assert.Equal(t, temp, cfg.LLM.Temperature, "Temperature in config should maintain exact precision")
		})
	}
}

// Helper function to format float without scientific notation
func formatFloat(f float64) string {
	return fmt.Sprintf("%.9f", f)
}
