// Package extractor provides test utilities for extracting JSON from LLM responses
package extractor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

func TestExtractLLMReviewOutput(t *testing.T) {
	logger := loggy.NewNoopLogger()
	extractor := NewJSONExtractor(logger)

	t.Run("successful extraction from code block", func(t *testing.T) {
		input := `I've reviewed the code and found several issues.

Here's my review:

` + "```json" + `
{
  "summary": "Found security and performance issues",
  "issues": [
    {
      "type": "security",
      "severity": "high",
      "title": "SQL Injection",
      "description": "Direct string concatenation for SQL creates injection risk",
      "line_start": 23,
      "line_end": 25,
      "suggestion": "Use parameterized queries",
      "affected_code": "query := \"SELECT * FROM users WHERE id = \" + userID",
      "code_snippet": "query := \"SELECT * FROM users WHERE id = $1\"\nrows, err := db.Query(query, userID)"
    }
  ],
  "overall_assessment": "The code needs significant security improvements"
}
` + "```" + `

That's my analysis.`

		result, err := extractor.ExtractLLMReviewOutput(input)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Found security and performance issues", result.Summary)
		assert.Equal(t, "The code needs significant security improvements", result.OverallAssessment)
		assert.Len(t, result.Issues, 1)

		issue := result.Issues[0]
		assert.Equal(t, "security", issue.Type)
		assert.Equal(t, "high", issue.Severity)
		assert.Equal(t, "SQL Injection", issue.Title)
		assert.Equal(t, 23, issue.LineStart)
		assert.Equal(t, 25, issue.LineEnd)
		assert.Equal(t, "Use parameterized queries", issue.Suggestion)
		assert.Equal(t, "query := \"SELECT * FROM users WHERE id = \" + userID", issue.AffectedCode)
		assert.Equal(t, "query := \"SELECT * FROM users WHERE id = $1\"\nrows, err := db.Query(query, userID)", issue.CodeSnippet)
	})

	t.Run("successful extraction from raw text", func(t *testing.T) {
		input := `I've analyzed the code and here's what I found:

The main issue is a security vulnerability.

{
  "summary": "Code contains security vulnerabilities",
  "issues": [
    {
      "type": "security",
      "severity": "critical",
      "title": "Hardcoded credentials",
      "description": "API key is hardcoded in source code",
      "line_start": 45,
      "line_end": 45,
      "suggestion": "Use environment variables",
      "affected_code": "const apiKey = \"ABC123XYZ\"",
      "code_snippet": "apiKey := os.Getenv(\"API_KEY\")"
    }
  ],
  "overall_assessment": "Security issues need to be addressed"
}`

		result, err := extractor.ExtractLLMReviewOutput(input)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Code contains security vulnerabilities", result.Summary)
		assert.Equal(t, "Security issues need to be addressed", result.OverallAssessment)
		assert.Len(t, result.Issues, 1)

		issue := result.Issues[0]
		assert.Equal(t, "security", issue.Type)
		assert.Equal(t, "critical", issue.Severity)
		assert.Equal(t, 45, issue.LineStart)
	})

	t.Run("extraction with escaped code snippets", func(t *testing.T) {
		input := `{
  "summary": "Multiple issues found",
  "issues": [
    {
      "type": "security",
      "severity": "high",
      "title": "Unsafe code execution",
      "description": "Executing commands without validation",
      "line_start": 15,
      "line_end": 18,
      "suggestion": "Validate input before execution",
      "affected_code": "cmd := exec.Command(\"sh\", \"-c\", userInput)",
      "code_snippet": "// Validate input first\nif !isValidCommand(userInput) {\n  return fmt.Errorf(\"invalid command\")\n}\ncmd := exec.Command(\"sh\", \"-c\", userInput)"
    }
  ],
  "overall_assessment": "The code has security issues"
}`

		result, err := extractor.ExtractLLMReviewOutput(input)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Issues, 1)

		issue := result.Issues[0]
		assert.Equal(t, "security", issue.Type)
		assert.Contains(t, issue.AffectedCode, "cmd := exec.Command")
		assert.Contains(t, issue.CodeSnippet, "Validate input first")
	})

	t.Run("extraction with complex code snippets containing quotes and escapes", func(t *testing.T) {
		input := `{
  "summary": "Found issues in string handling",
  "issues": [
    {
      "type": "bug",
      "severity": "medium",
      "title": "Improper JSON escaping",
      "description": "The code doesn't properly escape JSON values",
      "line_start": 42,
      "line_end": 45,
      "suggestion": "Use json.Marshal instead of string concatenation",
      "affected_code": "jsonStr := \"{\\\"name\\\":\\\"\" + username + \"\\\"}\"",
      "code_snippet": "data := map[string]string{\"name\": username}\njsonBytes, err := json.Marshal(data)\nif err != nil {\n  return err\n}\njsonStr := string(jsonBytes)"
    }
  ],
  "overall_assessment": "The code needs careful review for string handling"
}`

		result, err := extractor.ExtractLLMReviewOutput(input)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Issues, 1)

		issue := result.Issues[0]
		assert.Equal(t, "bug", issue.Type)
		assert.Contains(t, issue.AffectedCode, "jsonStr :=")
		assert.Contains(t, issue.CodeSnippet, "json.Marshal")
	})

	t.Run("extraction with thinking tags", func(t *testing.T) {
		input := `<think>
First, I need to analyze this code for issues.
I see there's a potential SQL injection vulnerability.
Let me format this in proper JSON.
</think>

{
  "summary": "SQL injection vulnerability found",
  "issues": [
    {
      "type": "security",
      "severity": "high",
      "title": "SQL Injection Risk",
      "description": "String concatenation in SQL query",
      "line_start": 33,
      "line_end": 33,
      "suggestion": "Use prepared statements",
      "affected_code": "query := \"SELECT * FROM users WHERE name = '\" + userName + \"'\"",
      "code_snippet": "query := \"SELECT * FROM users WHERE name = ?\"\nrows, err := db.Query(query, userName)"
    }
  ],
  "overall_assessment": "Fix security issues before production"
}`

		result, err := extractor.ExtractLLMReviewOutput(input)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "SQL injection vulnerability found", result.Summary)
		assert.Len(t, result.Issues, 1)
	})

	t.Run("extraction with empty issues array", func(t *testing.T) {
		input := `{
  "summary": "No issues found",
  "issues": [],
  "overall_assessment": "Code is well-written"
}`

		result, err := extractor.ExtractLLMReviewOutput(input)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "No issues found", result.Summary)
		assert.Equal(t, "Code is well-written", result.OverallAssessment)
		assert.Len(t, result.Issues, 0)
	})

	t.Run("handle missing fields with defaults", func(t *testing.T) {
		input := `{
  "summary": "Review of code",
  "issues": [
    {
      "title": "Missing type field"
    }
  ]
}`

		// Extract the JSON and print for debugging
		jsonContent, err := extractJSON(input)
		if err != nil {
			t.Fatalf("Error extracting JSON: %v", err)
		}

		// Try to process manually for debugging
		placeholders := make(map[string]string)
		sanitized := extractAndReplaceCodeBlocks(jsonContent, placeholders)
		sanitized = applyBasicFixes(sanitized)

		result, err := extractor.ExtractLLMReviewOutput(input)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Issues, 1)

		// Skip the rest of the assertions if we don't have any issues
		if len(result.Issues) == 0 {
			t.FailNow()
		}

		issue := result.Issues[0]
		assert.Equal(t, "unknown", issue.Type)
		assert.Equal(t, "medium", issue.Severity)
		assert.Equal(t, "Missing type field", issue.Title)
	})

	t.Run("no JSON found", func(t *testing.T) {
		input := `This response doesn't contain any valid JSON structure.`

		result, err := extractor.ExtractLLMReviewOutput(input)

		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("manual extraction fallback", func(t *testing.T) {
		// This JSON is invalid (missing quotes around field names) but should be handled by manual extraction
		input := `{
  summary: "Manual extraction test",
  issues: [
    {
      type: "bug",
      severity: "low",
      title: "Code formatting issue",
      description: "Inconsistent indentation",
      line_start: 25,
      line_end: 30
    }
  ],
  overall_assessment: "Minor issues only"
}`

		result, err := extractor.ExtractLLMReviewOutput(input)

		// It should recover and use manual extraction
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Contains(t, result.Summary, "Manual extraction test")
	})

	t.Run("Claude response format extraction", func(t *testing.T) {
		// This is a typical Claude response with introduction text and then JSON
		input := `I'll review this Go code for issues and provide a detailed analysis.

Looking at the code, I can see several issues related to error handling, security, concurrency, and best practices. Let me analyze them systematically.

{
  "summary": "The code contains multiple critical and high-severity issues including hardcoded credentials, improper error handling, race conditions, path traversal vulnerabilities, and use of deprecated packages.",
  "issues": [
    {
      "type": "security",
      "severity": "critical",
      "title": "Hardcoded secret key",
      "description": "The code contains a hardcoded secret key which is a serious security vulnerability. Credentials should never be hardcoded in source code.",
      "line_start": 77,
      "line_end": 77,
      "suggestion": "Store secrets in environment variables or a secure secret management system.",
      "affected_code": "\tsecretKey = \"my-secret-key-12345\" // Issue: Hardcoded secret",
      "code_snippet": "\tsecretKey = os.Getenv(\"APP_SECRET_KEY\")"
    },
    {
      "type": "security",
      "severity": "high",
      "title": "Path traversal vulnerability",
      "description": "The ValidateInput function attempts to prevent path traversal but only checks for '../' which is insufficient. Attackers can use various techniques to bypass this check.",
      "line_start": 46,
      "line_end": 48,
      "suggestion": "Use filepath.Clean and proper path validation or use a whitelist approach.",
      "affected_code": "func ValidateInput(input string) bool {\n\treturn len(input) > 0 && !strings.Contains(input, \"../\")\n}",
      "code_snippet": "func ValidateInput(input string) bool {\n\t// Ensure input is not empty and doesn't contain any path traversal sequences\n\tcleanPath := filepath.Clean(input)\n\t// Check if cleaning the path changed it (indicating traversal attempt)\n\treturn len(input) > 0 && cleanPath == input\n}"
    }
  ],
  "overall_assessment": "The code has multiple critical and high-severity issues that need immediate attention. It contains security vulnerabilities (hardcoded credentials, weak hash function, path traversal), concurrency problems (race conditions), and error handling deficiencies. The code also uses deprecated packages and has performance inefficiencies. A thorough refactoring with a focus on security and proper error handling is strongly recommended."
}`

		result, err := extractor.ExtractLLMReviewOutput(input)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Contains(t, result.Summary, "multiple critical and high-severity issues")
		assert.Len(t, result.Issues, 2)

		// Check the first issue
		issue := result.Issues[0]
		assert.Equal(t, "security", issue.Type)
		assert.Equal(t, "critical", issue.Severity)
		assert.Equal(t, "Hardcoded secret key", issue.Title)
		assert.Equal(t, 77, issue.LineStart)
		assert.Equal(t, 77, issue.LineEnd)
		assert.Contains(t, issue.AffectedCode, "my-secret-key-12345")
		assert.Contains(t, issue.CodeSnippet, "os.Getenv")

		// Check the second issue
		issue = result.Issues[1]
		assert.Equal(t, "security", issue.Type)
		assert.Equal(t, "high", issue.Severity)
		assert.Equal(t, "Path traversal vulnerability", issue.Title)
		assert.Equal(t, 46, issue.LineStart)
		assert.Equal(t, 48, issue.LineEnd)
		assert.Contains(t, issue.AffectedCode, "ValidateInput")
		assert.Contains(t, issue.CodeSnippet, "filepath.Clean")
	})
}
