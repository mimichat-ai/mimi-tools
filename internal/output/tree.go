// Copyright (c) 2025-2026 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package output

import (
	"fmt"
	"strings"
)

// Node represents a single entry in the directory tree.
type Node struct {
	Name       string
	Path       string
	IsDir      bool
	Depth      int
	ParentPath string
	Children   []Node
	IsLast     bool
	Truncated  bool
	Skipped    bool
	Ignored    bool // true when the directory is git-ignored or built-in ignored
}

// TreeOutput holds the result of a tree operation.
type TreeOutput struct {
	Root         string
	MaxDepth     int
	TotalDirs    int
	TotalFiles   int
	SkippedCount int
	Nodes        []Node
	Note         string
}

// FormatTree generates the Markdown for tree results.
func FormatTree(o TreeOutput) string {
	var sb strings.Builder
	sb.WriteString("### 🌲 Tool Output: `tree`\n\n")
	fmt.Fprintf(&sb, "* **Root Path:** `%s`\n", o.Root)

	switch {
	case o.MaxDepth < 0:
		sb.WriteString("* **Max Depth:** `unlimited`\n")
	case o.MaxDepth == 0:
		sb.WriteString("* **Max Depth:** `0` (current directory only)\n")
	default:
		fmt.Fprintf(&sb, "* **Max Depth:** `%d`\n", o.MaxDepth)
	}

	fmt.Fprintf(&sb, "* **Total Directories:** `%d`\n", o.TotalDirs)
	fmt.Fprintf(&sb, "* **Total Files:** `%d`\n", o.TotalFiles)
	if o.SkippedCount > 0 {
		fmt.Fprintf(&sb, "* **Skipped Directories:** `%d`\n", o.SkippedCount)
	}
	sb.WriteString("\n```text\n")
	if len(o.Nodes) > 0 {
		sb.WriteString(renderTree(o.Nodes[0].Children, ""))
	}
	sb.WriteString("\n```\n")
	if o.Note != "" {
		sb.WriteString("\n")
		sb.WriteString(o.Note)
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderTree(nodes []Node, prefix string) string {
	var sb strings.Builder
	for i, node := range nodes {
		isLast := i == len(nodes)-1
		indicator := "├── "
		if isLast {
			indicator = "└── "
		}
		sb.WriteString(prefix)
		sb.WriteString(indicator)
		sb.WriteString(node.Name)
		if node.IsDir {
			sb.WriteString("/")
			// Add status markers
			if node.Skipped {
				sb.WriteString(" 🔒")
			} else if node.Truncated {
				sb.WriteString(" ...")
			} else if node.Ignored {
				sb.WriteString(" (ignored)")
			}
		}
		sb.WriteString("\n")
		if node.IsDir && len(node.Children) > 0 {
			// Recurse
			extension := prefix
			if isLast {
				extension += "    "
			} else {
				extension += "│   "
			}
			sb.WriteString(renderTree(node.Children, extension))
		}
	}
	return sb.String()
}
