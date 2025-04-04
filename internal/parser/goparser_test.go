package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

func TestNewGoModule(t *testing.T) {
	// Create a test logger
	logger := loggy.NewNoopLogger()

	// Create a Go module
	module := NewGoModule(logger)

	if module == nil {
		t.Error("NewGoModule() returned nil")
	}

	if module.GetName() != "Go" {
		t.Errorf("Expected module name to be 'Go', got %s", module.GetName())
	}
}

func TestGoModule_ParseFile(t *testing.T) {
	// Create a test logger
	logger := loggy.NewNoopLogger()

	// Create a Go module
	module := NewGoModule(logger)

	tests := []struct {
		name        string
		fileContent string
		wantChunks  int
		wantErr     bool
	}{
		{
			name: "Simple Go file",
			fileContent: `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`,
			wantChunks: 4, // Package + Import + function + file
			wantErr:    false,
		},
		{
			name: "Go file with struct and method",
			fileContent: `package main

import "fmt"

type Person struct {
	Name string
	Age  int
}

func (p Person) Greet() string {
	return fmt.Sprintf("Hello, my name is %s and I am %d years old.", p.Name, p.Age)
}

func main() {
	person := Person{Name: "Alice", Age: 30}
	fmt.Println(person.Greet())
}
`,
			wantChunks: 7, // Package + Import + struct + method + function + var in main + file
			wantErr:    false,
		},
		{
			name: "Go file with syntax error",
			fileContent: `package main

import "fmt"

func main() {
	fmt.Println("Missing closing quote)
}
`,
			wantChunks: 0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file
			dir := t.TempDir()
			filePath := filepath.Join(dir, "test.go")
			err := os.WriteFile(filePath, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatal(err)
			}

			// Parse the file
			chunks, err := module.ParseFile(filePath)

			// Check results
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(chunks) != tt.wantChunks {
				t.Errorf("ParseFile() got %d chunks, want %d", len(chunks), tt.wantChunks)
				for i, chunk := range chunks {
					t.Logf("Chunk %d: Type=%s, Name=%s", i, chunk.Type, chunk.Name)
				}
			}
		})
	}
}

func TestGoModule_ChunkTypes(t *testing.T) {
	// Create a test logger
	logger := loggy.NewNoopLogger()

	// Create a Go module
	module := NewGoModule(logger)

	tests := []struct {
		name        string
		fileContent string
		chunkTypes  map[string]int // Map of chunk type to expected count
	}{
		{
			name: "Count various chunk types",
			fileContent: `package main

import (
	"fmt"
	"strings"
)

type User struct {
	Name string
	Age  int
}

type Service interface {
	Process(data string) error
}

func (u User) Greet() string {
	return fmt.Sprintf("Hello, %s!", u.Name)
}

const (
	MaxUsers = 100
	Version  = "1.0.0"
)

var (
	DefaultUser = User{Name: "Guest", Age: 0}
)

func main() {
	u := DefaultUser
	fmt.Println(u.Greet())
}
`,
			chunkTypes: map[string]int{
				"package":   0, // Package is now handled differently
				"import":    2, // Individual imports are counted
				"struct":    1,
				"interface": 1,
				"method":    1,
				"const":     1,
				"var":       1,
				"function":  1,
				"file":      1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file
			dir := t.TempDir()
			filePath := filepath.Join(dir, "test.go")
			err := os.WriteFile(filePath, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatal(err)
			}

			// Parse the file
			chunks, err := module.ParseFile(filePath)
			if err != nil {
				t.Fatal(err)
			}

			// Count chunks by type
			chunkCounts := make(map[string]int)
			for _, chunk := range chunks {
				chunkCounts[chunk.Type]++
			}

			// Verify counts match expectations
			for chunkType, expectedCount := range tt.chunkTypes {
				assert.Equal(t, expectedCount, chunkCounts[chunkType],
					"Expected %d chunks of type %s, got %d",
					expectedCount, chunkType, chunkCounts[chunkType])
			}
		})
	}
}

func TestGoModule_ChunkPositions(t *testing.T) {
	// Create a test logger
	logger := loggy.NewNoopLogger()

	// Create a Go module
	module := NewGoModule(logger)

	// Test file content
	fileContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`

	// Create a temporary file
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	err := os.WriteFile(filePath, []byte(fileContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Parse the file
	chunks, err := module.ParseFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that all chunks have valid positions
	for _, chunk := range chunks {
		assert.NotEmpty(t, chunk.StartPos.Filename, "StartPos.Filename should not be empty")
		assert.NotEmpty(t, chunk.EndPos.Filename, "EndPos.Filename should not be empty")
		assert.GreaterOrEqual(t, chunk.StartPos.Line, 1, "StartPos.Line should be >= 1")
		assert.GreaterOrEqual(t, chunk.EndPos.Line, 1, "EndPos.Line should be >= 1")
		assert.GreaterOrEqual(t, chunk.StartPos.Offset, 0, "StartPos.Offset should be >= 0")
		assert.GreaterOrEqual(t, chunk.EndPos.Offset, 0, "EndPos.Offset should be >= 0")
		assert.LessOrEqual(t, chunk.StartPos.Offset, chunk.EndPos.Offset, "StartPos.Offset should be <= EndPos.Offset")
	}
}
