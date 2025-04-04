package parser

import (
	"fmt"
	"os"
	"strings"

	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// GenericModule is a module for handling any text-based file
type GenericModule struct {
	logger *loggy.Logger
}

// NewGenericModule creates a new generic module
func NewGenericModule(logger *loggy.Logger) *GenericModule {
	return &GenericModule{
		logger: logger,
	}
}

// GetName returns the name of the language this module handles
func (m *GenericModule) GetName() string {
	return LanguageGeneric
}

// CanHandle returns true if this module can handle the given language
func (m *GenericModule) CanHandle(language string) bool {
	// The generic module handles specific text-based languages
	genericLanguages := []string{
		"Text", "Markdown", "HTML", "CSS", "JSON", "YAML", "XML",
		"Documentation", "Shell", "PowerShell", "Dockerfile",
		"JavaScript", "Python",
	}

	for _, generic := range genericLanguages {
		if language == generic {
			return true
		}
	}

	return false
}

// ParseFile parses a single file as text and returns a single chunk
func (m *GenericModule) ParseFile(filePath string) ([]*RawChunk, error) {
	// Read the file
	data, err := readFileContents(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	// Create a single chunk for the whole file
	chunks := []*RawChunk{
		{
			Type:    string(ChunkTypeFile),
			Name:    filePath,
			Content: string(data),
			StartPos: Position{
				Filename: filePath,
				Offset:   0,
				Line:     1,
				Column:   1,
			},
			EndPos: Position{
				Filename: filePath,
				Offset:   len(data),
				Line:     countLines(data),
				Column:   1,
			},
			Metadata: map[string]interface{}{
				"language": LanguageGeneric,
			},
		},
	}

	return chunks, nil
}

// readFileContents reads a file and returns its contents
func readFileContents(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

// countLines counts the number of lines in the data
func countLines(data []byte) int {
	return strings.Count(string(data), "\n") + 1
}
