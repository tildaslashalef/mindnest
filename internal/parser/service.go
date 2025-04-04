// Package parser provides code parsing and language detection utilities for the Mindnest application
package parser

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// Service provides code parsing functionality
type Service struct {
	logger           *loggy.Logger
	languageDetector *LanguageDetector
	moduleRegistry   *ModuleRegistry
}

// NewService creates a new parser service
func NewService(logger *loggy.Logger) *Service {
	// Create language detector
	languageDetector := NewLanguageDetector(logger)

	// Create module registry
	moduleRegistry := NewModuleRegistry(logger, languageDetector)

	s := &Service{
		logger:           logger,
		languageDetector: languageDetector,
		moduleRegistry:   moduleRegistry,
	}

	// Register default modules
	s.RegisterDefaultModules()

	return s
}

// RegisterDefaultModules registers the default language modules
func (s *Service) RegisterDefaultModules() {
	// Register Go module
	goModule := NewGoModule(s.logger)
	s.moduleRegistry.RegisterModule(goModule)

	// Register generic module for fallback
	genericModule := NewGenericModule(s.logger)
	s.moduleRegistry.RegisterModule(genericModule)
}

// DetectLanguage detects the language of a file
func (s *Service) DetectLanguage(filePath string) (string, error) {
	language, err := s.languageDetector.DetectLanguage(filePath)
	if err != nil {
		return "", fmt.Errorf("detecting language: %w", err)
	}
	return language, nil
}

// ParseFile parses a file and returns its chunks
func (s *Service) ParseFile(filePath string) ([]*RawChunk, string, error) {
	chunks, language, err := s.moduleRegistry.ParseFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("parsing file: %w", err)
	}
	return chunks, language, nil
}

// IsCodeFile checks if a file is a code file (not binary, vendored, or documentation)
func (s *Service) IsCodeFile(filePath string) bool {
	// Exclude go.mod and go.sum files
	filename := filepath.Base(filePath)
	if filename == "go.mod" || filename == "go.sum" {
		s.logger.Debug("File is a Go dependency file", "path", filePath, "type", filename)
		return false
	}

	// Check if the file is a binary, vendored, or documentation file
	if s.languageDetector.IsVendorFile(filePath) {
		s.logger.Debug("File is a vendor file", "path", filePath)
		return false
	}

	if s.languageDetector.IsDocumentationFile(filePath) {
		// Markdown files are considered code files for our purposes
		// ext := strings.ToLower(filepath.Ext(filePath))
		// if ext == ".md" || ext == ".markdown" {
		// 	s.logger.Debug("File is a markdown documentation file", "path", filePath)
		// 	return true
		// }
		// s.logger.Debug("File is a non-markdown documentation file", "path", filePath)
		return false
	}

	// Check if the file exists and is readable
	if _, err := os.Stat(filePath); err != nil {
		s.logger.Debug("File does not exist or cannot be accessed", "path", filePath, "error", err)
		return false
	}

	// Try to detect the language
	language, err := s.languageDetector.DetectLanguage(filePath)
	if err != nil {
		return false
	}

	// Files with no detected language or binary files are not code files
	if language == "" || language == "Binary" {
		return false
	}

	return true
}

// RegisterModule registers a new language module
func (s *Service) RegisterModule(module Module) {
	s.moduleRegistry.RegisterModule(module)
}

// ListModules returns a list of registered module names
func (s *Service) ListModules() []string {
	var names []string
	for _, module := range s.moduleRegistry.modules {
		names = append(names, module.GetName())
	}
	return names
}

// GetLanguageDetector returns the language detector
func (s *Service) GetLanguageDetector() *LanguageDetector {
	return s.languageDetector
}

// GetModuleRegistry returns the module registry
func (s *Service) GetModuleRegistry() *ModuleRegistry {
	return s.moduleRegistry
}
