// Copyright (c) 2025-2026 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package file

import (
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// resultText extracts the text content from a CallToolResult.
func resultText(r *mcp.CallToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	if tc, ok := r.Content[0].(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

// containsErrorType checks if the error text contains the expected error type.
func containsErrorType(text, expectedType string) bool {
	return strings.Contains(text, "**Type:** `"+expectedType+"`")
}

// containsErrorReason checks if the error text contains the expected reason.
func containsErrorReason(text, expectedReason string) bool {
	return strings.Contains(text, "**Reason:** "+expectedReason)
}
