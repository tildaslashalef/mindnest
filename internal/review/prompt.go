package review

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// Templates for building prompts
const systemInstructionTemplate = `You are a senior code reviewer analyzing {{.Language}} code. Your **PRIMARY GOAL** is to provide a **VALID JSON response**, even if it includes other text before it. The JSON response **MUST** be a complete, parseable JSON object as your final statement. It is more important to provide a valid JSON object than to avoid including extra text.

Follow this schema **EXACTLY** without adding any additional fields or arrays:

{
  "summary": "Brief findings overview",
  "issues": [
    {
      "type": "bug|security|performance|design|style|complexity|best_practice",
      "severity": "critical|high|medium|low",
      "title": "Issue title",
      "description": "Issue explanation",
      "line_start": 10,
      "line_end": 15,
      "suggestion": "Fix suggestion",
      "affected_code": "EXACT problematic code from the file",
      "code_snippet": "Complete corrected implementation of the affected function/section"
    }
  ],
  "overall_assessment": "Quality assessment"
}

IMPORTANT:
- **ONLY** include the fields specified above (summary, issues, overall_assessment).**
- DO **NOT** add additional fields or arrays.
- **INCLUDE** all three required fields even if empty.
- Look for all types of issues including bugs, performance, design, style, complexity, and best practices. Pay special attention to security issues like SQL injection, command injection, hardcoded credentials, etc.
- For "affected_code", you **MUST** copy the **EXACT** problematic code from the source file (do not paraphrase or describe it).
- For "code_snippet", provide the **complete** corrected implementation of the affected function or section, showing how to fix the issue. Make sure the code snippet is compilable and runnable.
- Include **accurate** line numbers for each issue.
- Choose appropriate severity levels (critical, high, medium, low) based on the issue's impact.

If no issues are found, the JSON response **MUST** be: {"summary": "No issues found", "issues": [], "overall_assessment": "Code is well-written"}.

Example Issue (for guidance - not to be directly used):

{
    "type": "bug",
    "severity": "medium",
    "title": "Potential Resource Leak: Unclosed HTTP Response Body",
    "description": "The fetchData function fetches data from a URL but does not always close the HTTP response body. If an error occurs after the request is made but before the body is fully consumed, the connection may remain open, leading to a resource leak, especially under heavy load.",
    "line_start": 65,
    "line_end": 75,
    "suggestion": "Ensure the HTTP response body is always closed using defer resp.Body.Close() immediately after checking for an error when creating the response.",
    "affected_code": "func fetchData(url string) ([]byte, error) {\n    resp, err := http.Get(url)\n    if err != nil {\n        return nil, err\n    }\n\n    body, err := io.ReadAll(resp.Body)\n    if err != nil {\n        return nil, err\n    }\n    return body, nil\n}",
    "code_snippet": "func fetchData(url string) ([]byte, error) {\n    resp, err := http.Get(url)\n    if err != nil {\n        return nil, err\n    }\n    defer resp.Body.Close()\n\n    body, err := io.ReadAll(resp.Body)\n    if err != nil {\n        return nil, err\n    }\n    return body, nil\n}"
}

Provide the **JSON** response as your **LAST** statement, even if you have other text before it.`

const fileContextTemplate = `## Code to Review:
{{.FileHeader}}

{{.Content}}

{{if .SimilarChunks}}
## Related Code:
{{.SimilarChunks}}
{{end}}
`

const fileHeaderTemplate = `File: {{.Path}} ({{.Language}}{{if .DiffInfo}}, {{.DiffInfo}}{{end}})`

const similarChunkTemplate = `### {{.Name}} ({{.ChunkType}})
{{.Content}}
`

// PromptOptions contains options for generating prompts
type PromptOptions struct {
	Language       string
	PrimaryChunk   bool
	IncludeContext bool
	ContextDepth   int
}

// DefaultPromptOptions returns default prompt options
func DefaultPromptOptions() *PromptOptions {
	return &PromptOptions{
		Language:       "Go",
		PrimaryChunk:   true,
		IncludeContext: true,
		ContextDepth:   3,
	}
}

// BuildSystemInstruction builds the system instruction for code review
func BuildSystemInstruction(language string) (string, error) {
	templateText := systemInstructionTemplate

	tmpl, err := template.New("system").Parse(templateText)
	if err != nil {
		return "", err
	}

	// Normalize language name
	lang := strings.ToUpper(language[:1]) + strings.ToLower(language[1:])

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{
		"Language": lang,
	}); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// BuildFileContext builds the context for a file review
func BuildFileContext(file *workspace.File, content string, diffInfo string, similarChunks []*workspace.Chunk) (string, error) {
	// Build file header
	headerTmpl, err := template.New("header").Parse(fileHeaderTemplate)
	if err != nil {
		return "", err
	}

	var headerBuf bytes.Buffer
	if err := headerTmpl.Execute(&headerBuf, map[string]string{
		"Path":     file.Path,
		"Language": file.Language,
		"DiffInfo": diffInfo,
	}); err != nil {
		return "", err
	}

	// Build similar chunks section if available
	var chunksBuf bytes.Buffer
	if len(similarChunks) > 0 {
		chunkTmpl, err := template.New("chunk").Parse(similarChunkTemplate)
		if err != nil {
			return "", err
		}

		for _, chunk := range similarChunks {
			if err := chunkTmpl.Execute(&chunksBuf, chunk); err != nil {
				return "", err
			}
		}
	}

	// Build final context
	contextTmpl, err := template.New("context").Parse(fileContextTemplate)
	if err != nil {
		return "", err
	}

	var contextBuf bytes.Buffer
	if err := contextTmpl.Execute(&contextBuf, map[string]string{
		"FileHeader":    headerBuf.String(),
		"Content":       content,
		"SimilarChunks": chunksBuf.String(),
	}); err != nil {
		return "", err
	}

	return contextBuf.String(), nil
}

// BuildMessageList builds a list of messages for the LLM chat API
func BuildMessageList(file *workspace.File, content string, diffInfo string, similarChunks []*workspace.Chunk, options *PromptOptions) ([]map[string]string, error) {
	if options == nil {
		options = DefaultPromptOptions()
	}

	// Build system instruction
	sysInstruction, err := BuildSystemInstruction(options.Language)
	if err != nil {
		return nil, fmt.Errorf("building system instruction: %w", err)
	}

	// Build file context
	fileContext, err := BuildFileContext(file, content, diffInfo, similarChunks)
	if err != nil {
		return nil, fmt.Errorf("building file context: %w", err)
	}

	// Create message list
	messages := []map[string]string{
		{
			"role":    "system",
			"content": sysInstruction,
		},
		{
			"role":    "user",
			"content": fmt.Sprintf("Please review the following code:\n\n%s", fileContext),
		},
	}

	return messages, nil
}
