// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package output

import (
	"fmt"
	"strings"
)

// EditOutput holds the result of a file edit operation.
type EditOutput struct {
	Path       string
	Diff       string
	Applied    bool
	MatchScore float64 // 0 for exact match, similarity score for fuzzy match (0.7-1.0)
}

// FormatEdit generates the Markdown for edit results.
func FormatEdit(o EditOutput) string {
	var sb strings.Builder
	sb.WriteString("### 📝 Tool Output: `edit`\n\n")
	fmt.Fprintf(&sb, "* **File Modified:** `%s`\n", o.Path)

	if o.Applied {
		if o.MatchScore > 0 && o.MatchScore < 1.0 {
			// Fuzzy match
			fmt.Fprintf(&sb, "* **Status:** `Applied via fuzzy match (%.3f)` ✅\n", o.MatchScore)
			fmt.Fprintf(&sb, "* **Match Mode:** `Fuzzy (%.3f)`\n\n", o.MatchScore)
		} else {
			sb.WriteString("* **Status:** `Applied successfully (exact match)` ✅\n\n")
		}
	} else {
		sb.WriteString("* **Status:** `No changes` ⏹️\n\n")
		return sb.String()
	}

	sb.WriteString("#### 🔍 Changes (Diff)\n\n")
	sb.WriteString("```diff\n")
	sb.WriteString(o.Diff)
	sb.WriteString("\n```\n")

	return sb.String()
}
