package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

func TestNewLanguageDetector(t *testing.T) {
	// Create a test logger
	logger := loggy.NewNoopLogger()

	// Create a language detector
	detector := NewLanguageDetector(logger)

	if detector == nil {
		t.Error("NewLanguageDetector() returned nil")
	}
}

func TestLanguageDetector_DetectLanguage(t *testing.T) {
	// Create a test logger
	logger := loggy.NewNoopLogger()

	// Create a language detector
	detector := NewLanguageDetector(logger)

	tests := []struct {
		name         string
		fileContent  string
		fileName     string
		wantLanguage string
		wantErr      bool
	}{
		{
			name:         "Detect Go by extension",
			fileContent:  `package main`,
			fileName:     "main.go",
			wantLanguage: "Go",
			wantErr:      false,
		},
		{
			name:         "Detect JavaScript by extension",
			fileContent:  `const x = 1;`,
			fileName:     "script.js",
			wantLanguage: "JavaScript",
			wantErr:      false,
		},
		{
			name:         "Detect Python by extension",
			fileContent:  `def main():`,
			fileName:     "script.py",
			wantLanguage: "Python",
			wantErr:      false,
		},
		{
			name:         "Detect Rust by extension",
			fileContent:  `fn main() {}`,
			fileName:     "main.rs",
			wantLanguage: "Rust",
			wantErr:      false,
		},
		{
			name:         "Detect generic text file",
			fileContent:  `This is a plain text file.`,
			fileName:     "readme.txt",
			wantLanguage: "Documentation",
			wantErr:      false,
		},
		{
			name:         "Detect Markdown",
			fileContent:  `# Heading\n\nText`,
			fileName:     "readme.md",
			wantLanguage: "Markdown",
			wantErr:      false,
		},
		{
			name:         "No extension",
			fileContent:  `#!/bin/bash`,
			fileName:     "script",
			wantLanguage: "Shell",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file
			dir := t.TempDir()
			filePath := filepath.Join(dir, tt.fileName)
			err := os.WriteFile(filePath, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatal(err)
			}

			// Detect language
			lang, err := detector.DetectLanguage(filePath)

			// Check results
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectLanguage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && lang != tt.wantLanguage {
				t.Errorf("DetectLanguage() = %v, want %v", lang, tt.wantLanguage)
			}
		})
	}
}

func TestLanguageDetector_IsVendorFile(t *testing.T) {
	// Create a test logger
	logger := loggy.NewNoopLogger()

	// Create a language detector
	detector := NewLanguageDetector(logger)

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "Vendor directory",
			filePath: "path/to/vendor/file.go",
			want:     true,
		},
		{
			name:     "Node modules directory",
			filePath: "path/to/node_modules/package/index.js",
			want:     true,
		},
		{
			name:     "Git directory",
			filePath: ".git/HEAD",
			want:     true,
		},
		{
			name:     "Regular file",
			filePath: "src/main.go",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detector.IsVendorFile(tt.filePath); got != tt.want {
				t.Errorf("IsVendorFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLanguageDetector_IsDocumentationFile(t *testing.T) {
	// Create a test logger
	logger := loggy.NewNoopLogger()

	// Create a language detector
	detector := NewLanguageDetector(logger)

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "README file",
			filePath: "README.md",
			want:     true,
		},
		{
			name:     "LICENSE file",
			filePath: "LICENSE",
			want:     true,
		},
		{
			name:     "CONTRIBUTING file",
			filePath: "CONTRIBUTING.md",
			want:     true,
		},
		{
			name:     "Regular source file",
			filePath: "src/main.go",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detector.IsDocumentationFile(tt.filePath); got != tt.want {
				t.Errorf("IsDocumentationFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLanguageDetector_ListGitIgnoredFiles(t *testing.T) {
	// Create a test logger
	logger := loggy.NewNoopLogger()

	// Create a language detector
	detector := NewLanguageDetector(logger)

	// Create a temporary directory
	dir := t.TempDir()

	// Create a .gitignore file
	gitignorePath := filepath.Join(dir, ".gitignore")
	gitignoreContent := `
# Ignore node_modules
node_modules/

# Ignore build directory
build/

# Ignore log files
*.log
`
	err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create some files that should be ignored
	os.MkdirAll(filepath.Join(dir, "node_modules"), 0755)
	os.MkdirAll(filepath.Join(dir, "build"), 0755)
	os.WriteFile(filepath.Join(dir, "app.log"), []byte("log content"), 0644)

	// Create a file that should not be ignored
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	// Test the function
	ignoredPatterns, err := detector.ListGitIgnoredFiles(dir)
	assert.NoError(t, err)

	// Check that the patterns were parsed correctly
	assert.Contains(t, ignoredPatterns, "node_modules/")
	assert.Contains(t, ignoredPatterns, "build/")
	assert.Contains(t, ignoredPatterns, "*.log")
}
