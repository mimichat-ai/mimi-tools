// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package output

import (
	"fmt"
	"strings"
)

// WriteOutput holds the result of a file write operation.
type WriteOutput struct {
	Path       string
	Bytes      int
	TotalLines int
	Created    bool
}

// FormatWrite generates the Markdown for write results.
func FormatWrite(o WriteOutput) string {
	var sb strings.Builder
	sb.WriteString("### 💾 Tool Output: `write`\n\n")

	if o.Created {
		fmt.Fprintf(&sb, "* **File Created:** `%s`\n", o.Path)
	} else {
		fmt.Fprintf(&sb, "* **File Overwritten:** `%s`\n", o.Path)
	}

	fmt.Fprintf(&sb, "* **Size:** `%d bytes`\n", o.Bytes)
	fmt.Fprintf(&sb, "* **Lines:** `%d`\n", o.TotalLines)

	return sb.String()
}
