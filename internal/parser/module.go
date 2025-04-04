// Package parser provides code parsing and language detection utilities for the Mindnest application
package parser

import (
	"fmt"
	"os"
	"strings"

	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// Module represents a language module that can parse code
type Module interface {
	// GetName returns the name of the language that this module handles
	GetName() string
	// CanHandle returns true if this module can handle the given language
	CanHandle(language string) bool
	// ParseFile parses a single file and returns a list of chunks
	ParseFile(filePath string) ([]*RawChunk, error)
}

// ModuleRegistry manages and provides access to language modules
type ModuleRegistry struct {
	logger           *loggy.Logger
	modules          []Module
	languageDetector *LanguageDetector
}

// NewModuleRegistry creates a new module registry
func NewModuleRegistry(logger *loggy.Logger, languageDetector *LanguageDetector) *ModuleRegistry {
	return &ModuleRegistry{
		logger:           logger,
		modules:          make([]Module, 0),
		languageDetector: languageDetector,
	}
}

// RegisterModule registers a language module
func (r *ModuleRegistry) RegisterModule(module Module) {
	r.modules = append(r.modules, module)
}

// GetModuleForLanguage returns the module that can handle the given language
func (r *ModuleRegistry) GetModuleForLanguage(language string) (Module, error) {
	for _, module := range r.modules {
		if module.CanHandle(language) {
			return module, nil
		}
	}
	return nil, fmt.Errorf("no module found for language: %s", language)
}

// GetModuleForFile returns the module that can handle the given file
func (r *ModuleRegistry) GetModuleForFile(filePath string) (Module, string, error) {
	// Detect the language of the file
	language, err := r.languageDetector.DetectLanguage(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("detecting language: %w", err)
	}

	// Get the module for the language
	module, err := r.GetModuleForLanguage(language)
	if err != nil {
		return nil, language, fmt.Errorf("getting module: %w", err)
	}

	return module, language, nil
}

// ParseFile parses a file using the appropriate module
func (r *ModuleRegistry) ParseFile(filePath string) ([]*RawChunk, string, error) {
	// Check if file exists
	_, err := os.Stat(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("checking file: %w", err)
	}

	// Get the module for the file
	module, language, err := r.GetModuleForFile(filePath)
	if err != nil {
		// If no module is found, return an empty list
		if strings.Contains(err.Error(), "no module found for language") {
			r.logger.Debug("No parser module found for file", "path", filePath, "language", language)
			return []*RawChunk{}, language, nil
		}
		return nil, "", fmt.Errorf("getting module: %w", err)
	}

	// Parse the file
	chunks, err := module.ParseFile(filePath)
	if err != nil {
		return nil, language, fmt.Errorf("parsing file: %w", err)
	}

	return chunks, language, nil
}
