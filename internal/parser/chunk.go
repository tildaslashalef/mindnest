// Package parser provides code parsing and language detection utilities for the Mindnest application
package parser

// Position represents a position in the source code
type Position struct {
	Filename string `json:"filename"`
	Offset   int    `json:"offset"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

// ChunkType represents the type of a code chunk
type ChunkType string

const (
	// ChunkTypeFunction represents a function
	ChunkTypeFunction ChunkType = "function"
	// ChunkTypeMethod represents a method
	ChunkTypeMethod ChunkType = "method"
	// ChunkTypeStruct represents a struct definition
	ChunkTypeStruct ChunkType = "struct"
	// ChunkTypeInterface represents an interface definition
	ChunkTypeInterface ChunkType = "interface"
	// ChunkTypeType represents a type definition
	ChunkTypeType ChunkType = "type"
	// ChunkTypeConst represents a constant declaration
	ChunkTypeConst ChunkType = "const"
	// ChunkTypeVar represents a variable declaration
	ChunkTypeVar ChunkType = "var"
	// ChunkTypeImport represents an import declaration
	ChunkTypeImport ChunkType = "import"
	// ChunkTypePackage represents a package declaration
	ChunkTypePackage ChunkType = "package"
	// ChunkTypeFile represents an entire file
	ChunkTypeFile ChunkType = "file"
	// ChunkTypeBlock represents a block of code (e.g., if, for, switch)
	ChunkTypeBlock ChunkType = "block"
)

// RawChunk represents raw parsing results before conversion to the workspace Chunk model
type RawChunk struct {
	ID        string
	Type      string
	Name      string
	Content   string
	StartPos  Position
	EndPos    Position
	Signature string
	ParentID  string
	ChildIDs  []string
	Metadata  map[string]interface{}
}
