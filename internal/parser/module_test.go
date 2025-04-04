package parser

import (
	"testing"

	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// mockModule implements Module for testing
type mockModule struct {
	name      string
	canHandle func(string) bool
	parseFile func(string) ([]*RawChunk, error)
}

func (m *mockModule) GetName() string {
	return m.name
}

func (m *mockModule) CanHandle(language string) bool {
	if m.canHandle != nil {
		return m.canHandle(language)
	}
	return m.name == language
}

func (m *mockModule) ParseFile(filePath string) ([]*RawChunk, error) {
	if m.parseFile != nil {
		return m.parseFile(filePath)
	}
	return []*RawChunk{}, nil
}

func TestModuleRegistry_RegisterModule(t *testing.T) {
	// Create a test logger
	logger := loggy.NewNoopLogger()

	// Create a language detector
	detector := NewLanguageDetector(logger)

	// Create a module registry
	registry := NewModuleRegistry(logger, detector)

	// Create a mock module
	mockModule := &mockModule{
		name: "TestLang",
	}

	// Register the module
	registry.RegisterModule(mockModule)

	// Check that the module was registered
	found := false
	for _, module := range registry.modules {
		if module.GetName() == "TestLang" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Module not registered")
	}
}

func TestModuleRegistry_GetModuleForLanguage(t *testing.T) {
	// Create a test logger
	logger := loggy.NewNoopLogger()

	// Create a language detector
	detector := NewLanguageDetector(logger)

	// Create a module registry
	registry := NewModuleRegistry(logger, detector)

	// Create mock modules
	goModule := &mockModule{
		name: "Go",
		canHandle: func(language string) bool {
			return language == "Go" || language == "go"
		},
	}

	jsModule := &mockModule{
		name: "JavaScript",
		canHandle: func(language string) bool {
			return language == "JavaScript" || language == "javascript" || language == "js"
		},
	}

	// Register the modules
	registry.RegisterModule(goModule)
	registry.RegisterModule(jsModule)

	tests := []struct {
		name     string
		language string
		want     string
		wantErr  bool
	}{
		{
			name:     "Get Go module",
			language: "Go",
			want:     "Go",
			wantErr:  false,
		},
		{
			name:     "Get Go module (lowercase)",
			language: "go",
			want:     "Go",
			wantErr:  false,
		},
		{
			name:     "Get JavaScript module",
			language: "JavaScript",
			want:     "JavaScript",
			wantErr:  false,
		},
		{
			name:     "Get JavaScript module (js alias)",
			language: "js",
			want:     "JavaScript",
			wantErr:  false,
		},
		{
			name:     "Unknown language",
			language: "PHP",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, err := registry.GetModuleForLanguage(tt.language)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetModuleForLanguage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if (module == nil) != tt.wantErr {
				t.Errorf("GetModuleForLanguage() nil check = %v, want %v", module == nil, tt.wantErr)
				return
			}

			if !tt.wantErr && module.GetName() != tt.want {
				t.Errorf("GetModuleForLanguage() = %v, want %v", module.GetName(), tt.want)
			}
		})
	}
}

func TestGoModule_Implementation(t *testing.T) {
	// Create a test logger
	logger := loggy.NewNoopLogger()

	// Create a Go module
	goModule := NewGoModule(logger)

	// Test GetName()
	if name := goModule.GetName(); name != "Go" {
		t.Errorf("GetName() = %v, want %v", name, "Go")
	}

	// Test CanHandle()
	canHandleTests := []struct {
		name     string
		language string
		want     bool
	}{
		{
			name:     "Go language",
			language: "Go",
			want:     true,
		},
		{
			name:     "Lowercase go language",
			language: "go",
			want:     true,
		},
		{
			name:     "Invalid language",
			language: "JavaScript",
			want:     false,
		},
	}

	for _, tt := range canHandleTests {
		t.Run(tt.name, func(t *testing.T) {
			if got := goModule.CanHandle(tt.language); got != tt.want {
				t.Errorf("CanHandle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenericModule_Implementation(t *testing.T) {
	// Create a test logger
	logger := loggy.NewNoopLogger()

	// Create a Generic module
	genericModule := NewGenericModule(logger)

	// Test Name()
	if name := genericModule.GetName(); name != "Generic" {
		t.Errorf("GetName() = %v, want %v", name, "Generic")
	}

	// Test CanHandle() - should handle many languages for text-based files
	canHandleTests := []struct {
		name     string
		language string
		want     bool
	}{
		{
			name:     "Text",
			language: "Text",
			want:     true,
		},
		{
			name:     "Markdown",
			language: "Markdown",
			want:     true,
		},
		{
			name:     "Documentation",
			language: "Documentation",
			want:     true,
		},
		{
			name:     "Go language",
			language: "Go",
			want:     false, // Generic module doesn't handle Go
		},
	}

	for _, tt := range canHandleTests {
		t.Run(tt.name, func(t *testing.T) {
			if got := genericModule.CanHandle(tt.language); got != tt.want {
				t.Errorf("CanHandle() = %v, want %v", got, tt.want)
			}
		})
	}
}
