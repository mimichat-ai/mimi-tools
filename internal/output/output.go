// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package output

import (
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MatchSuggestion represents a fuzzy-matched code snippet returned when exact edit fails.
type MatchSuggestion struct {
	LineStart  int     `json:"line_start" jsonschema:"1-based start line number"`
	LineEnd    int     `json:"line_end" jsonschema:"1-based end line number"`
	ByteStart  int     `json:"byte_start" jsonschema:"0-based byte start position"`
	ByteEnd    int     `json:"byte_end" jsonschema:"0-based byte end position"`
	Similarity float64 `json:"similarity" jsonschema:"normalized similarity to old_string, 1=identical"`
	Diff       string  `json:"diff" jsonschema:"diff between old_string and the matched content"`
}

// ToolError represents a structured error for display.
type ToolError struct {
	Type       string           // Required
	Reason     string           // Required
	Target     string           // Optional
	Suggestion string           // Optional
	BestMatch  *MatchSuggestion // Optional: single best fuzzy match when old_string is not found
}

// FormatError generates a unified Markdown error string.
func FormatError(toolName string, err ToolError) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "### ❌ Tool Error: `%s`\n\n", toolName)
	fmt.Fprintf(&sb, "* **Type:** `%s`\n", err.Type)
	fmt.Fprintf(&sb, "* **Reason:** %s", err.Reason)
	if err.Target != "" {
		fmt.Fprintf(&sb, "\n* **Target:** `%s`", err.Target)
	}
	if err.Suggestion != "" {
		fmt.Fprintf(&sb, "\n* **Suggestion:** %s", err.Suggestion)
	}
	if err.BestMatch != nil {
		sb.WriteString("\n\n### 🔍 Found Similar Code (Not Replaced)\n\n")
		fmt.Fprintf(&sb, "**Location:** Lines %d-%d | **Similarity:** %.1f%%\n\n", err.BestMatch.LineStart, err.BestMatch.LineEnd, err.BestMatch.Similarity*100)
		if err.Type == "Ambiguous Match" {
			sb.WriteString("The following code is one of multiple similar blocks found. The match is ambiguous because multiple non-overlapping regions have similarity > 95%.\n\n")
		} else {
			sb.WriteString("The following code in the file is similar to your `old_string`, but the similarity is below the 95% threshold for automatic replacement.\n\n")
		}
		sb.WriteString("**Difference between your `old_string` and the file content:**\n\n")
		sb.WriteString("```diff\n")
		sb.WriteString(err.BestMatch.Diff)
		sb.WriteString("\n```\n\n")
		sb.WriteString("**Suggested actions:**\n")
		sb.WriteString("1. Update your `old_string` to match the file content exactly, or\n")
		sb.WriteString("2. Use the above location to manually verify the code\n")
	} else if err.Type == "String Not Found" && err.Target != "" {
		// BestMatch is nil and it's a String Not Found error (edit tool), show troubleshooting tips
		sb.WriteString("\n\n### 🔍 No Similar Match Found\n")
		sb.WriteString("No code snippet similar to `old_string` was found. Please check:\n")
		sb.WriteString("- Whether the file path is correct\n")
		sb.WriteString("- Whether `old_string` has typos\n")
		sb.WriteString("- Whether the file content has been modified\n")
	}
	sb.WriteString("\n")
	return sb.String()
}

// NewTextResult creates a standard MCP text result.
func NewTextResult(text string, isError bool) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
		IsError: isError,
	}
}
