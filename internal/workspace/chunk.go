// Package workspace provides workspace management for the Mindnest application
package workspace

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/tildaslashalef/mindnest/internal/parser"
	"github.com/tildaslashalef/mindnest/internal/ulid"
)

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

// Chunk represents a piece of code from a file
type Chunk struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspace_id"`
	FileID      string          `json:"file_id"`
	Name        string          `json:"name,omitempty"`
	Content     string          `json:"content"`
	StartPos    parser.Position `json:"start_pos"`
	EndPos      parser.Position `json:"end_pos"`
	ChunkType   ChunkType       `json:"chunk_type"`
	Signature   string          `json:"signature,omitempty"`
	ParentID    string          `json:"parent_id,omitempty"`
	ChildIDs    []string        `json:"child_ids,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// NewChunk creates a new code chunk
func NewChunk(workspaceID, fileID string, content string, chunkType ChunkType) *Chunk {
	now := time.Now()
	return &Chunk{
		ID:          ulid.ChunkID(),
		WorkspaceID: workspaceID,
		FileID:      fileID,
		Content:     content,
		ChunkType:   chunkType,
		StartPos: parser.Position{
			Filename: fileID,
			Line:     1,
			Column:   1,
		},
		EndPos: parser.Position{
			Filename: fileID,
		},
		ChildIDs:  make([]string, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewChunkFromRawChunk creates a new Chunk from a parser.RawChunk
func NewChunkFromRawChunk(workspaceID, fileID string, rawChunk *parser.RawChunk) *Chunk {
	now := time.Now()
	chunkType := ChunkType(rawChunk.Type)

	chunk := &Chunk{
		ID:          ulid.ChunkID(),
		WorkspaceID: workspaceID,
		FileID:      fileID,
		Name:        rawChunk.Name,
		Content:     rawChunk.Content,
		StartPos:    rawChunk.StartPos,
		EndPos:      rawChunk.EndPos,
		ChunkType:   chunkType,
		Signature:   rawChunk.Signature,
		ParentID:    rawChunk.ParentID,
		ChildIDs:    rawChunk.ChildIDs,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Set metadata if available
	if rawChunk.Metadata != nil && len(rawChunk.Metadata) > 0 {
		data, err := json.Marshal(rawChunk.Metadata)
		if err == nil {
			chunk.Metadata = data
		}
	}

	return chunk
}

// SetSignature sets the signature for the chunk (e.g., function signature)
func (c *Chunk) SetSignature(signature string) {
	c.Signature = signature
	c.UpdatedAt = time.Now()
}

// SetMetadata sets the metadata for the chunk
func (c *Chunk) SetMetadata(metadata interface{}) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	c.Metadata = data
	c.UpdatedAt = time.Now()
	return nil
}

// GetMetadata unmarshals the metadata into the provided struct
func (c *Chunk) GetMetadata(target interface{}) error {
	if c.Metadata == nil {
		return nil // No metadata to unmarshal
	}

	err := json.Unmarshal(c.Metadata, target)
	if err != nil {
		return fmt.Errorf("unmarshaling metadata: %w", err)
	}

	return nil
}

// StartLine returns the start line of the chunk (for compatibility)
func (c *Chunk) StartLine() int {
	return c.StartPos.Line
}

// EndLine returns the end line of the chunk (for compatibility)
func (c *Chunk) EndLine() int {
	return c.EndPos.Line
}

// StartOffset returns the start offset of the chunk (for compatibility)
func (c *Chunk) StartOffset() int {
	return c.StartPos.Offset
}

// EndOffset returns the end offset of the chunk (for compatibility)
func (c *Chunk) EndOffset() int {
	return c.EndPos.Offset
}

// AddChild adds a child chunk ID
func (c *Chunk) AddChild(childID string) {
	c.ChildIDs = append(c.ChildIDs, childID)
	c.UpdatedAt = time.Now()
}
