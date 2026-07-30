// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package output

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ReadOutput holds the result of a file read operation.
type ReadOutput struct {
	Content    string
	TotalLines int
	StartLine  int
	EndLine    int
	IsEmpty    bool
	Path       string
	IsDir      bool
	Tree       string // The ASCII tree string if IsDir is true.
}

// FormatRead generates the Markdown for read results.
func FormatRead(o ReadOutput) string {
	var sb strings.Builder

	sb.WriteString("### 📄 Tool Output: `read`\n\n")

	if o.IsDir {
		fmt.Fprintf(&sb, "Path `%s` is a directory. Showing tree structure.\n\n", o.Path)
		sb.WriteString("#### 🌿 Tree\n\n")
		sb.WriteString("```text\n")
		sb.WriteString(o.Tree)
		sb.WriteString("\n```\n")
		return sb.String()
	}

	fmt.Fprintf(&sb, "* **Path:** `%s`\n", o.Path)
	fmt.Fprintf(&sb, "* **Lines:** `%d-%d of %d`\n", o.StartLine, o.EndLine, o.TotalLines)
	fmt.Fprintf(&sb, "* **Size:** `%d bytes`\n\n", len(o.Content))

	if o.IsEmpty {
		sb.WriteString("(empty file)\n")
	} else {
		ext := filepath.Ext(o.Path)
		lang := LangFromExt(ext)
		sb.WriteString("```")
		sb.WriteString(lang)
		sb.WriteString("\n")
		sb.WriteString(o.Content)
		sb.WriteString("\n```\n")
	}

	return sb.String()
}
