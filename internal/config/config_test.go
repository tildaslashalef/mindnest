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
	assert.Empty(t, cfg.DefaultLLMProvider)
	assert.Empty(t, cfg.Ollama.Endpoint)
	assert.Zero(t, cfg.Ollama.Timeout)
	assert.Empty(t, cfg.Logging.Level)
	assert.False(t, cfg.Workspace.AutoCreate)
}

func TestLoadFromEnv(t *testing.T) {
	// Reset any environment variables that might affect the test
	vars := []string{
		"MINDNEST_LLM_DEFAULT_PROVIDER",
		"MINDNEST_OLLAMA_ENDPOINT", "MINDNEST_OLLAMA_TIMEOUT", "MINDNEST_OLLAMA_MAX_RETRIES", "MINDNEST_OLLAMA_DEFAULT_MODEL",
		"MINDNEST_LOG_LEVEL", "MINDNEST_WORKSPACE_AUTO_CREATE",
	}

	for _, v := range vars {
		os.Unsetenv(v)
	}

	// Create a test .env file to check for interference
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(envFile, []byte("# Test env file\n"), 0644)
	assert.NoError(t, err)

	// Load config with defaults using the test directory
	cfgFromEnv, err := LoadFromEnv(tmpDir, envFile, false)
	assert.NoError(t, err)

	// Verify default values are set correctly
	assert.Equal(t, "claude", cfgFromEnv.DefaultLLMProvider)

	// Verify Ollama config
	assert.Equal(t, "http://localhost:11434", cfgFromEnv.Ollama.Endpoint)
	assert.Equal(t, 120*time.Second, cfgFromEnv.Ollama.Timeout)
	assert.Equal(t, 3, cfgFromEnv.Ollama.MaxRetries)
	assert.Equal(t, "gemma3", cfgFromEnv.Ollama.Model) // Updated to match current default
	assert.Equal(t, 100, cfgFromEnv.Ollama.MaxIdleConns)
	assert.Equal(t, 100, cfgFromEnv.Ollama.MaxIdleConnsPerHost)
	assert.Equal(t, 90*time.Second, cfgFromEnv.Ollama.IdleConnTimeout)

	// Other config fields
	assert.Equal(t, "info", cfgFromEnv.Logging.Level)
	assert.Equal(t, true, cfgFromEnv.Workspace.AutoCreate)
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
	testCfg.Ollama.Temperature = 0.5 // Change a value in Ollama config
	Set(testCfg)

	// Get should work now
	cfg, err := Get()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify the changed value
	assert.Equal(t, 0.5, cfg.Ollama.Temperature)
}

func TestValidate(t *testing.T) {
	// Valid config from LoadFromEnv should pass validation
	cfg, err := LoadFromEnv("", "", false)
	assert.NoError(t, err)
	err = cfg.Validate()
	assert.NoError(t, err)

	// Invalid LLM config
	invalidLLM := New()
	invalidLLM.DefaultLLMProvider = ""
	err = invalidLLM.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LLM config")

	// Invalid Ollama config
	invalidOllama := New()
	// Set required LLM values to pass LLM validation
	invalidOllama.DefaultLLMProvider = "ollama"
	// Set all required Ollama values except MaxRetries
	invalidOllama.Ollama.Endpoint = "http://localhost:11434"
	invalidOllama.Ollama.Timeout = 5 * time.Second
	invalidOllama.Ollama.MaxRetries = 0 // Invalid
	invalidOllama.Ollama.Model = "llama2"
	invalidOllama.Ollama.MaxTokens = 100
	invalidOllama.Ollama.Temperature = 0.7
	invalidOllama.Ollama.MaxIdleConns = 10
	invalidOllama.Ollama.MaxIdleConnsPerHost = 5
	invalidOllama.Ollama.IdleConnTimeout = 5 * time.Second
	err = invalidOllama.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Ollama config")
	assert.Contains(t, err.Error(), "max_retries must be positive")

	// Invalid RAG config
	invalidRAG := New()
	// Set required LLM values
	invalidRAG.DefaultLLMProvider = "ollama"
	// Set required Ollama values
	invalidRAG.Ollama.Endpoint = "http://localhost:11434"
	invalidRAG.Ollama.Timeout = 5 * time.Second
	invalidRAG.Ollama.MaxRetries = 3
	invalidRAG.Ollama.Model = "llama2"
	invalidRAG.Ollama.MaxTokens = 100
	invalidRAG.Ollama.Temperature = 0.7
	invalidRAG.Ollama.MaxIdleConns = 10
	invalidRAG.Ollama.MaxIdleConnsPerHost = 5
	invalidRAG.Ollama.IdleConnTimeout = 5 * time.Second
	// Set invalid RAG value
	invalidRAG.RAG.NSimilarChunks = 0 // Invalid
	invalidRAG.RAG.BatchSize = 10
	invalidRAG.RAG.MaxFilesSameDir = 10
	invalidRAG.RAG.ContextDepth = 2
	err = invalidRAG.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RAG config")
	assert.Contains(t, err.Error(), "number of similar chunks must be positive")

	// Test another RAG validation error
	invalidRAG.RAG.NSimilarChunks = 10
	invalidRAG.RAG.MaxFilesSameDir = 0 // Invalid
	err = invalidRAG.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max files in same directory must be positive")

	// Invalid logging config
	invalidLogging := New()
	// Set required LLM values
	invalidLogging.DefaultLLMProvider = "ollama"
	// Set required database values
	invalidLogging.Database.Path = "/tmp/test.db"
	invalidLogging.Database.BusyTimeout = 5000
	invalidLogging.Database.ConnMaxLife = 5 * time.Minute
	invalidLogging.Database.QueryTimeout = 30 * time.Second
	// Set required Ollama values
	invalidLogging.Ollama.Endpoint = "http://localhost:11434"
	invalidLogging.Ollama.Timeout = 5 * time.Second
	invalidLogging.Ollama.MaxRetries = 3
	invalidLogging.Ollama.Model = "llama2"
	invalidLogging.Ollama.MaxTokens = 100
	invalidLogging.Ollama.Temperature = 0.7
	invalidLogging.Ollama.MaxIdleConns = 10
	invalidLogging.Ollama.MaxIdleConnsPerHost = 5
	invalidLogging.Ollama.IdleConnTimeout = 5 * time.Second
	// Set required RAG values
	invalidLogging.RAG.NSimilarChunks = 10
	invalidLogging.RAG.BatchSize = 20
	invalidLogging.RAG.MaxFilesSameDir = 10
	invalidLogging.RAG.ContextDepth = 3
	// Set invalid logging value
	invalidLogging.Logging.Level = "invalid" // Invalid logging level
	err = invalidLogging.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logging config")
	assert.Contains(t, err.Error(), "invalid log level")
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

			// Test in Config for the Ollama temperature
			cfg := New()
			cfg.Ollama.Temperature = temp
			assert.Equal(t, temp, cfg.Ollama.Temperature, "Temperature in config should maintain exact precision")
		})
	}
}

// Helper function to format float without scientific notation
func formatFloat(f float64) string {
	return fmt.Sprintf("%.9f", f)
}
