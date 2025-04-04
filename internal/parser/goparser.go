package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/ulid"
)

// GoModule is a module for parsing Go code
type GoModule struct {
	logger *loggy.Logger
	fset   *token.FileSet
}

// NewGoModule creates a new Go module
func NewGoModule(logger *loggy.Logger) *GoModule {
	return &GoModule{
		logger: logger,
		fset:   token.NewFileSet(),
	}
}

// GetName returns the name of the language this module handles
func (m *GoModule) GetName() string {
	return LanguageGo
}

// CanHandle returns whether this module can handle the given language
func (m *GoModule) CanHandle(language string) bool {
	return language == "Go" || strings.ToLower(language) == "go"
}

// ParseFile parses a single Go file and returns a list of chunks
func (m *GoModule) ParseFile(filePath string) ([]*RawChunk, error) {
	m.logger.Debug("Parsing Go file", "path", filePath)

	// Read the file
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	// Parse the file
	astFile, err := parser.ParseFile(m.fset, filePath, fileContent, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}

	// Create a visitor to collect chunks
	visitor := newChunkVisitor(m.fset, fileContent, filePath, m.logger)

	// Visit the AST
	ast.Walk(visitor, astFile)

	// Get the collected chunks
	chunks := visitor.Chunks()

	return chunks, nil
}

// GetImports returns the imports from a Go file
func (m *GoModule) GetImports(filePath string) ([]string, error) {
	// Parse the file
	astFile, err := parser.ParseFile(m.fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("parsing file for imports: %w", err)
	}

	var imports []string
	for _, imp := range astFile.Imports {
		// Remove quotes from the import path
		path := strings.Trim(imp.Path.Value, "\"")
		imports = append(imports, path)
	}

	return imports, nil
}

// GetPackageName returns the package name from a Go file
func (m *GoModule) GetPackageName(filePath string) (string, error) {
	// Parse the file
	astFile, err := parser.ParseFile(m.fset, filePath, nil, parser.PackageClauseOnly)
	if err != nil {
		return "", fmt.Errorf("parsing file for package name: %w", err)
	}

	return astFile.Name.Name, nil
}

// FindGoFiles finds all Go files in a directory and its subdirectories
func (m *GoModule) FindGoFiles(dirPath string) ([]string, error) {
	var goFiles []string

	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories that start with a dot
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}

		// Skip vendor directories
		if d.IsDir() && d.Name() == "vendor" {
			return filepath.SkipDir
		}

		// Check if this is a Go file
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".go") {
			goFiles = append(goFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	return goFiles, nil
}

// chunkVisitor collects code chunks while traversing the AST
type chunkVisitor struct {
	fset        *token.FileSet
	fileContent []byte
	filePath    string
	chunks      []*RawChunk
	parentStack []*RawChunk
	logger      *loggy.Logger
}

// newChunkVisitor creates a new chunk visitor
func newChunkVisitor(fset *token.FileSet, fileContent []byte, filePath string, logger *loggy.Logger) *chunkVisitor {
	return &chunkVisitor{
		fset:        fset,
		fileContent: fileContent,
		filePath:    filePath,
		chunks:      []*RawChunk{},
		parentStack: []*RawChunk{},
		logger:      logger,
	}
}

// Chunks returns the collected chunks
func (v *chunkVisitor) Chunks() []*RawChunk {
	return v.chunks
}

// Visit implements the ast.Visitor interface
func (v *chunkVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		// We're done with the current node, pop the parent stack if not empty
		if len(v.parentStack) > 0 {
			v.parentStack = v.parentStack[:len(v.parentStack)-1]
		}
		return v
	}

	switch n := node.(type) {
	case *ast.File:
		// Create a chunk for the entire file
		fileChunk := v.createChunk(n.Name.Name, string(ChunkTypeFile), n.Pos(), n.End())
		v.addChunk(fileChunk)
		v.pushParent(fileChunk)

		// Visit all declarations and statements in the file
		for _, decl := range n.Decls {
			ast.Walk(v, decl)
		}

		// Return nil to avoid the automatic recursive traversal
		// since we manually traversed the nodes above
		return nil

	case *ast.FuncDecl:
		var chunkType string
		if n.Recv != nil {
			chunkType = string(ChunkTypeMethod)
		} else {
			chunkType = string(ChunkTypeFunction)
		}

		// Get function name
		name := n.Name.Name

		// Create chunk
		chunk := v.createChunk(name, chunkType, n.Pos(), n.End())

		// Add signature for functions and methods
		if n.Type != nil && n.Type.Params != nil {
			signature := v.getNodeText(n.Type)
			chunk.Signature = signature
		}

		// Add receiver info for methods
		if n.Recv != nil && len(n.Recv.List) > 0 {
			recvType := v.getNodeText(n.Recv.List[0].Type)
			if chunk.Metadata == nil {
				chunk.Metadata = make(map[string]interface{})
			}
			chunk.Metadata["receiver"] = recvType
		}

		v.addChunk(chunk)
		v.pushParent(chunk)

		// Manually visit the function body
		if n.Body != nil {
			// Create a block chunk for the function body
			blockChunk := v.createChunk("block", string(ChunkTypeBlock), n.Body.Pos(), n.Body.End())
			v.addChunk(blockChunk)
			v.pushParent(blockChunk)

			// Process statements inside the block
			for _, stmt := range n.Body.List {
				ast.Walk(v, stmt)
			}

			// Pop the block parent
			if len(v.parentStack) > 0 {
				v.parentStack = v.parentStack[:len(v.parentStack)-1]
			}
		}

		// Return nil to avoid automatic recursive traversal
		return nil

	case *ast.GenDecl:
		// Process type, var, const, and import declarations
		if n.Tok == token.TYPE {
			for _, spec := range n.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					var chunkType string
					// Determine the chunk type based on the underlying type
					if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
						chunkType = string(ChunkTypeStruct)
					} else if _, isInterface := typeSpec.Type.(*ast.InterfaceType); isInterface {
						chunkType = string(ChunkTypeInterface)
					} else {
						chunkType = string(ChunkTypeType)
					}

					// Create a chunk for the type declaration
					chunk := v.createChunk(typeSpec.Name.Name, chunkType, typeSpec.Pos(), typeSpec.End())

					// Add type information
					if chunk.Metadata == nil {
						chunk.Metadata = make(map[string]interface{})
					}
					chunk.Metadata["type_def"] = v.getNodeText(typeSpec.Type)

					v.addChunk(chunk)
					v.pushParent(chunk)

					// Visit the type definition
					ast.Walk(v, typeSpec.Type)

					// Pop the parent after processing the type
					if len(v.parentStack) > 0 {
						v.parentStack = v.parentStack[:len(v.parentStack)-1]
					}
				}
			}
		} else if n.Tok == token.CONST {
			// Process constant declarations
			chunk := v.createChunk("const", string(ChunkTypeConst), n.Pos(), n.End())
			v.addChunk(chunk)
			v.pushParent(chunk)

			// Visit all const specs
			for _, spec := range n.Specs {
				ast.Walk(v, spec)
			}

			// Pop the parent after processing
			if len(v.parentStack) > 0 {
				v.parentStack = v.parentStack[:len(v.parentStack)-1]
			}
		} else if n.Tok == token.VAR {
			// Process variable declarations
			chunk := v.createChunk("var", string(ChunkTypeVar), n.Pos(), n.End())
			v.addChunk(chunk)
			v.pushParent(chunk)

			// Visit all var specs
			for _, spec := range n.Specs {
				ast.Walk(v, spec)
			}

			// Pop the parent after processing
			if len(v.parentStack) > 0 {
				v.parentStack = v.parentStack[:len(v.parentStack)-1]
			}
		} else if n.Tok == token.IMPORT {
			// Process import declarations - each import spec needs its own chunk
			for _, spec := range n.Specs {
				if importSpec, ok := spec.(*ast.ImportSpec); ok {
					var importName string
					if importSpec.Path != nil {
						importName = importSpec.Path.Value
					}
					chunk := v.createChunk(importName, string(ChunkTypeImport), importSpec.Pos(), importSpec.End())
					v.addChunk(chunk)
				}
			}
		}
		return nil

	case *ast.BlockStmt:
		// Don't create chunks for function/method blocks, they are already covered
		parent := v.currentParent()
		if parent != nil && (parent.Type == string(ChunkTypeFunction) || parent.Type == string(ChunkTypeMethod)) {
			// Let the normal traversal continue for function blocks
			return v
		}

		// For other blocks (if, for, etc.), create a chunk
		blockChunk := v.createChunk("block", string(ChunkTypeBlock), n.Pos(), n.End())
		v.addChunk(blockChunk)
		v.pushParent(blockChunk)

		// Process statements inside the block
		for _, stmt := range n.List {
			ast.Walk(v, stmt)
		}

		// Pop the parent after processing the block
		if len(v.parentStack) > 0 {
			v.parentStack = v.parentStack[:len(v.parentStack)-1]
		}

		return nil
	}

	// For other node types, continue regular traversal
	return v
}

// createChunk creates a new code chunk
func (v *chunkVisitor) createChunk(name string, chunkType string, start token.Pos, end token.Pos) *RawChunk {
	// Get position information
	startPos := v.fset.Position(start)
	endPos := v.fset.Position(end)

	// Get the content
	content := string(v.fileContent[startPos.Offset:endPos.Offset])

	// Create the chunk
	return &RawChunk{
		ID:      ulid.ChunkID(),
		Type:    chunkType,
		Name:    name,
		Content: content,
		StartPos: Position{
			Filename: v.filePath,
			Offset:   startPos.Offset,
			Line:     startPos.Line,
			Column:   startPos.Column,
		},
		EndPos: Position{
			Filename: v.filePath,
			Offset:   endPos.Offset,
			Line:     endPos.Line,
			Column:   endPos.Column,
		},
		ChildIDs: []string{},
		Metadata: make(map[string]interface{}),
	}
}

// addChunk adds a chunk to the collection and links it to its parent
func (v *chunkVisitor) addChunk(chunk *RawChunk) {
	// Get the current parent chunk
	parent := v.currentParent()

	// Set parent-child relationship if parent exists
	if parent != nil {
		// Set the parent ID on the chunk
		chunk.ParentID = parent.ID

		// Add the chunk to the parent's children list
		if parent.ChildIDs == nil {
			parent.ChildIDs = []string{}
		}
		parent.ChildIDs = append(parent.ChildIDs, chunk.ID)
	}
	// Add to the collection
	v.chunks = append(v.chunks, chunk)
}

// pushParent pushes a chunk onto the parent stack
func (v *chunkVisitor) pushParent(chunk *RawChunk) {
	v.parentStack = append(v.parentStack, chunk)
}

// currentParent returns the current parent chunk
func (v *chunkVisitor) currentParent() *RawChunk {
	if len(v.parentStack) == 0 {
		return nil
	}
	parent := v.parentStack[len(v.parentStack)-1]
	return parent
}

// getNodeText returns the text of an AST node
func (v *chunkVisitor) getNodeText(node ast.Node) string {
	if node == nil {
		return ""
	}

	startPos := v.fset.Position(node.Pos())
	endPos := v.fset.Position(node.End())

	return string(v.fileContent[startPos.Offset:endPos.Offset])
}
