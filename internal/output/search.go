// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package output

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const maxSearchDisplay = 10000

// SearchContentMatch represents a single match in search_content.
type SearchContentMatch struct {
	Path    string
	LineNum int
	Line    string
	Context []string // context lines around the match (before + after)
}

// SearchContentOutput holds the results for search_content.
type SearchContentOutput struct {
	Pattern   string
	Matches   []SearchContentMatch
	Total     int
	IsLimited bool
}

// FormatSearchContent generates the Markdown for search_content results.
func FormatSearchContent(o SearchContentOutput) string {
	var sb strings.Builder

	sb.WriteString("### 🔍 Tool Output: `search_content`\n\n")
	fmt.Fprintf(&sb, "* **Query:** `%s`\n", o.Pattern)
	fmt.Fprintf(&sb, "* **Total Matches:** `%d`\n\n", o.Total)

	// Group matches by path
	fileMatches := map[string][]SearchContentMatch{}
	for _, m := range o.Matches {
		// Normalize path to forward slashes for cross-platform consistency
		normPath := filepath.ToSlash(m.Path)
		fileMatches[normPath] = append(fileMatches[normPath], m)
	}

	// Sort file paths for deterministic output order
	sortedPaths := make([]string, 0, len(fileMatches))
	for path := range fileMatches {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Strings(sortedPaths)

	// Count unique files and render
	fileCount := 0
	for _, path := range sortedPaths {
		if fileCount >= maxSearchDisplay {
			continue
		}
		fileCount++
		matches := fileMatches[path]
		fmt.Fprintf(&sb, "* 📄 `%s` (%d matches)\n", path, len(matches))

		// Render individual matches with context for the first few matches per file
		maxShow := 10
		if len(matches) > maxShow {
			matches = matches[:maxShow]
		}
		for _, m := range matches {
			lang := LangFromExt(filepath.Ext(m.Path))
			fmt.Fprintf(&sb, "  - **Line %d:**\n", m.LineNum)
			if len(m.Context) > 0 {
				fmt.Fprintf(&sb, "    ```%s\n", lang)
				for _, ctxLine := range m.Context {
					fmt.Fprintf(&sb, "    %s\n", ctxLine)
				}
				fmt.Fprintf(&sb, "    ```\n")
			} else {
				fmt.Fprintf(&sb, "    `%s`\n", m.Line)
			}
		}
		if len(fileMatches[path]) > maxShow {
			fmt.Fprintf(&sb, "  - ... and %d more matches\n", len(fileMatches[path])-maxShow)
		}
	}

	if len(fileMatches) > maxSearchDisplay {
		fmt.Fprintf(&sb, "\n... and %d more files\n", len(fileMatches)-maxSearchDisplay)
	}

	if o.IsLimited {
		fmt.Fprintf(&sb, "\nShowing: %d (limit reached — use more specific pattern to narrow results)\n", maxSearchDisplay)
	}

	return sb.String()
}

// SearchNameOutput holds the results for search_name.
type SearchNameOutput struct {
	Pattern   string
	Files     []string
	Total     int
	IsLimited bool
}

// FormatSearchName generates the Markdown for search_name results.
func FormatSearchName(o SearchNameOutput) string {
	var sb strings.Builder

	sb.WriteString("### 📂 Tool Output: `search_name`\n\n")
	fmt.Fprintf(&sb, "* **Pattern:** `%s`\n", o.Pattern)
	fmt.Fprintf(&sb, "* **Total Found:** `%d` files\n\n", o.Total)

	for i, f := range o.Files {
		if i >= maxSearchDisplay {
			fmt.Fprintf(&sb, "\n... and %d more files\n", len(o.Files)-i)
			break
		}
		// Normalize path to forward slashes for cross-platform consistency
		normPath := filepath.ToSlash(f)
		fmt.Fprintf(&sb, "* 📁 `%s`\n", normPath)
	}

	return sb.String()
}
