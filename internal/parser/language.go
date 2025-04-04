// Package parser provides code parsing and language detection utilities for the Mindnest application
package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-enry/go-enry/v2"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// Language types supported by the parser system
const (
	// Common language types
	LanguageGo         = "Go"
	LanguageJavaScript = "JavaScript"
	LanguagePython     = "Python"
	LanguageRust       = "Rust"
	LanguageGeneric    = "Generic"
	LanguageUnknown    = "Unknown"
	LanguageVendor     = "Vendor"
	LanguageDoc        = "Documentation"
	LanguageBinary     = "Binary"
)

// langFilePatterns is a map of language names to file patterns
// This is useful for filtering files when scanning a directory
var langFilePatterns = map[string][]string{
	"Go":         {".go"},
	"JavaScript": {".js", ".jsx", ".mjs"},
	"TypeScript": {".ts", ".tsx"},
	"Python":     {".py", ".pyw", ".ipynb"},
	"Java":       {".java"},
	"C":          {".c", ".h"},
	"C++":        {".cpp", ".hpp", ".cc", ".hh", ".cxx", ".hxx"},
	"C#":         {".cs"},
	"Ruby":       {".rb"},
	"PHP":        {".php"},
	"Rust":       {".rs"},
	"Swift":      {".swift"},
	"Kotlin":     {".kt", ".kts"},
	"HTML":       {".html", ".htm"},
	"CSS":        {".css"},
	"JSON":       {".json"},
	"YAML":       {".yml", ".yaml"},
	"Markdown":   {".md", ".markdown"},
	"XML":        {".xml"},
	"SQL":        {".sql"},
	"Shell":      {".sh", ".bash"},
	"PowerShell": {".ps1"},
}

// LanguageDetector detects the programming language of a file
type LanguageDetector struct {
	logger *loggy.Logger
}

// NewLanguageDetector creates a new language detector
func NewLanguageDetector(logger *loggy.Logger) *LanguageDetector {
	return &LanguageDetector{
		logger: logger,
	}
}

// DetectLanguage determines the programming language of a file
func (d *LanguageDetector) DetectLanguage(filePath string) (string, error) {
	// Get the file name and extension
	fileName := filepath.Base(filePath)
	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		d.logger.Debug("File does not exist or cannot be accessed", "path", filePath, "error", err)
		return "", fmt.Errorf("accessing file: %w", err)
	}

	// Check if it's a documentation file first
	if d.IsDocumentationFile(filePath) {
		// Handle Markdown files
		if strings.HasSuffix(fileName, ".md") || strings.HasSuffix(fileName, ".markdown") {
			return "Markdown", nil
		}
		return "Documentation", nil
	}

	// Read a small sample of the file
	data, err := readFileSample(filePath, 8*1024) // Read up to 8KB
	if err != nil {
		d.logger.Debug("Error reading file sample", "path", filePath, "error", err)
		return "", fmt.Errorf("reading file: %w", err)
	}

	// Use go-enry to determine the language
	language := enry.GetLanguage(fileName, data)
	if language != "" {
		// Go specific handling
		if language == "Go" {
			return "Go", nil
		}

		// Special handling for text-based files
		if language == "Text" || strings.HasSuffix(fileName, ".txt") {
			return "Text", nil
		}

		if language == "Markdown" {
			return "Markdown", nil
		}

		d.logger.Debug("Detected file with language", "path", filePath, "language", language)
		return language, nil
	}

	// Fallback to extension-based detection if content detection isn't safe
	language, _ = enry.GetLanguageByExtension(filePath)
	d.logger.Debug("Fallback to extension detection", "path", filePath, "detected", language)

	if language != "" {
		return language, nil
	}

	// If extension doesn't help, use filename
	language, _ = enry.GetLanguageByFilename(fileName)
	d.logger.Debug("Fallback to filename detection", "path", filePath, "detected", language)

	if language != "" {
		return language, nil
	}

	// If it's binary, mark it as such
	if enry.IsBinary(data) {
		d.logger.Debug("Detected binary file", "path", filePath)
		return "Binary", nil
	}

	// If it's hidden, check the file extension
	if enry.IsVendor(filePath) || strings.HasPrefix(fileName, ".") {
		ext := filepath.Ext(fileName)
		if ext != "" {
			return ext[1:], nil // Remove the leading dot
		}
	}

	// If all else fails, treat as plain text
	d.logger.Debug("No language detected, defaulting to Text", "path", filePath)
	return "Text", nil
}

// IsGoFile checks if a file is a Go source file
func (d *LanguageDetector) IsGoFile(filePath string) bool {
	language, err := d.DetectLanguage(filePath)
	if err != nil {
		return false
	}
	return language == LanguageGo
}

// readFileSample reads a sample of a file up to maxSize bytes
func readFileSample(filePath string, maxSize int64) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("getting file info: %w", err)
	}

	// Determine how much to read
	size := fileInfo.Size()
	if size > maxSize {
		size = maxSize
	}

	// Read the sample
	sample := make([]byte, size)
	_, err = file.Read(sample)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	return sample, nil
}

// IsVendorFile checks if the file is in a vendor directory
func (d *LanguageDetector) IsVendorFile(path string) bool {
	// Check for .git directory or files
	if strings.Contains(path, "/.git/") || path == ".git" || strings.HasPrefix(path, ".git/") {
		d.logger.Debug("File is in .git directory", "path", path)
		return true
	}

	// Check for vendor directories
	vendorDirs := []string{
		"/vendor/",
		"/node_modules/",
	}

	for _, dir := range vendorDirs {
		if strings.Contains(path, dir) {
			return true
		}
	}

	// Use enry's vendor detection
	isVendor := enry.IsVendor(path)

	return isVendor
}

// IsDocumentationFile checks if a file is a documentation file
func (d *LanguageDetector) IsDocumentationFile(filePath string) bool {
	return enry.IsDocumentation(filePath)
}

// GetLangFilePatterns returns a list of file patterns for a language
func (d *LanguageDetector) GetLangFilePatterns(language string) []string {
	if patterns, ok := langFilePatterns[language]; ok {
		return patterns
	}
	return []string{}
}

// ListGitIgnoredFiles returns a list of patterns from a .gitignore file
func (d *LanguageDetector) ListGitIgnoredFiles(repoPath string) ([]string, error) {
	gitignorePath := filepath.Join(repoPath, ".gitignore")

	// Check if .gitignore file exists
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		return []string{}, nil // Return empty list if .gitignore doesn't exist
	}

	// Read .gitignore file
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		return nil, fmt.Errorf("reading .gitignore: %w", err)
	}

	// Parse patterns
	var patterns []string
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		patterns = append(patterns, line)
	}

	return patterns, nil
}
