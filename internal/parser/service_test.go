package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

func TestParserService(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "parser_test_*")
	require.NoError(t, err, "Failed to create temporary directory")
	defer os.RemoveAll(tempDir) // Clean up after test

	// Create a test Go file
	testGoFile := filepath.Join(tempDir, "main.go")
	testGoContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}
`
	err = os.WriteFile(testGoFile, []byte(testGoContent), 0644)
	require.NoError(t, err, "Failed to write test Go file")

	// Create a test text file
	testTextFile := filepath.Join(tempDir, "readme.txt")
	testTextContent := `This is a sample text file.
It has multiple lines.
This is used for testing.
`
	err = os.WriteFile(testTextFile, []byte(testTextContent), 0644)
	require.NoError(t, err, "Failed to write test text file")

	// Create a logger
	logger := loggy.NewNoopLogger()

	// Create the parser service
	service := NewService(logger)

	// Test ParseFile with Go file
	t.Run("ParseFile_Go", func(t *testing.T) {
		chunks, lang, err := service.ParseFile(testGoFile)
		assert.NoError(t, err, "ParseFile should not return an error for Go file")
		assert.Equal(t, "Go", lang, "Language should be detected as Go")
		assert.NotEmpty(t, chunks, "Should return non-empty chunks for Go file")

		// Skip detailed chunk verification as it depends on the specific parser implementation
	})

	// Test ParseFile with text file
	t.Run("ParseFile_Text", func(t *testing.T) {
		chunks, lang, err := service.ParseFile(testTextFile)
		assert.NoError(t, err, "ParseFile should not return an error for text file")
		assert.Equal(t, "Documentation", lang, "Language should be detected as Documentation")
		assert.NotEmpty(t, chunks, "Should return non-empty chunks for text file")

		// Verify there's at least a file chunk
		foundFile := false
		for _, chunk := range chunks {
			if chunk.Type == string(ChunkTypeFile) {
				foundFile = true
				assert.Contains(t, chunk.Content, "This is a sample text file", "File chunk should contain expected content")
			}
		}

		assert.True(t, foundFile, "Should have found a file chunk")
	})

	// Test ParseFile with non-existent file
	t.Run("ParseFile_NonExistent", func(t *testing.T) {
		chunks, lang, err := service.ParseFile(filepath.Join(tempDir, "non-existent.go"))
		assert.Error(t, err, "ParseFile should return an error for non-existent file")
		assert.Empty(t, lang, "Language should be empty for non-existent file")
		assert.Empty(t, chunks, "Should return empty chunks for non-existent file")
	})

	// Test DetectLanguage
	t.Run("DetectLanguage", func(t *testing.T) {
		// Test with Go file
		goLang, err := service.DetectLanguage(testGoFile)
		assert.NoError(t, err, "DetectLanguage should not return an error for Go file")
		assert.Equal(t, "Go", goLang, "Language should be detected as Go")

		// Test with text file
		textLang, err := service.DetectLanguage(testTextFile)
		assert.NoError(t, err, "DetectLanguage should not return an error for text file")
		assert.Equal(t, "Documentation", textLang, "Language should be detected as Documentation")

		// Test with non-existent file
		_, err = service.DetectLanguage(filepath.Join(tempDir, "non-existent.go"))
		assert.Error(t, err, "DetectLanguage should return an error for non-existent file")
	})

	// Test IsCodeFile
	t.Run("IsCodeFile", func(t *testing.T) {
		// Test with Go file
		isGoCode := service.IsCodeFile(testGoFile)
		assert.True(t, isGoCode, "Go file should be recognized as a code file")

		// Create a markdown file which should be treated as code
		mdFile := filepath.Join(tempDir, "readme.md")
		err := os.WriteFile(mdFile, []byte("# Test Markdown"), 0644)
		require.NoError(t, err, "Failed to write markdown file")

		isMdCode := service.IsCodeFile(mdFile)
		assert.False(t, isMdCode, "Markdown file should not be recognized as a code file")

		// Create a vendor file
		vendorDir := filepath.Join(tempDir, "vendor")
		err = os.MkdirAll(vendorDir, 0755)
		require.NoError(t, err, "Failed to create vendor directory")

		vendorFile := filepath.Join(vendorDir, "vendor.go")
		err = os.WriteFile(vendorFile, []byte("package vendor"), 0644)
		require.NoError(t, err, "Failed to write vendor file")

		isVendorCode := service.IsCodeFile(vendorFile)
		assert.False(t, isVendorCode, "Vendor file should not be recognized as a code file")
	})

	// Test ListModules
	t.Run("ListModules", func(t *testing.T) {
		modules := service.ListModules()
		assert.NotEmpty(t, modules, "Service should have registered modules")

		// Check if Go and Generic modules are registered
		assert.Contains(t, modules, "Go", "Go module should be registered")
		assert.Contains(t, modules, "Generic", "Generic module should be registered")
	})
}
