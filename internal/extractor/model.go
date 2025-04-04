// Package extractor provides utilities for extracting and sanitizing JSON from LLM responses
package extractor

// LLMReviewOutput represents the raw structure of a code review response from an LLM
type LLMReviewOutput struct {
	Summary           string     `json:"summary"`
	Issues            []LLMIssue `json:"issues"`
	OverallAssessment string     `json:"overall_assessment"`
}

// LLMIssue represents a single issue found during code review in LLM response
type LLMIssue struct {
	Type         string `json:"type"`
	Severity     string `json:"severity"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	LineStart    int    `json:"line_start"`
	LineEnd      int    `json:"line_end"`
	Suggestion   string `json:"suggestion"`
	AffectedCode string `json:"affected_code"`
	CodeSnippet  string `json:"code_snippet"`
}
