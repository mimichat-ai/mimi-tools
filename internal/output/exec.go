// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package output

import (
	"fmt"
	"strings"
)

// ExecOutput holds the result of a command execution.
type ExecOutput struct {
	Path        string
	Stdout      string
	Stderr      string
	ExitCode    int
	Running     bool
	PID         int
	WaitSeconds float64
}

// FormatExec generates the Markdown for execution results.
func FormatExec(o ExecOutput) string {
	var sb strings.Builder

	sb.WriteString("### 🖥️ Tool Output: `exec`\n\n")
	if o.Path != "" {
		fmt.Fprintf(&sb, "* **Path:** `%s`\n", o.Path)
	}

	// Exit code status
	exitStatus := "🟢"
	if o.ExitCode != 0 {
		exitStatus = "🔴"
	}
	if o.Running {
		fmt.Fprintf(&sb, "* **Exit Code:** `-1` ⏳ (still running)\n")
	} else {
		fmt.Fprintf(&sb, "* **Exit Code:** `%d` %s\n", o.ExitCode, exitStatus)
	}

	// Running details
	if o.Running {
		fmt.Fprintf(&sb, "* **PID:** `%d`\n", o.PID)
		fmt.Fprintf(&sb, "* **Waited:** `%ds` (partial)\n", int(o.WaitSeconds))
	}

	// Stdout (only shown if non-empty)
	if o.Stdout != "" {
		sb.WriteString("\n#### 📝 Stdout\n\n")
		sb.WriteString(formatCodeBlock(o.Stdout, "text"))
	}

	// Stderr (only shown if non-empty)
	if o.Stderr != "" {
		sb.WriteString("\n#### ⚠️ Stderr\n\n")
		sb.WriteString(formatCodeBlock(o.Stderr, "text"))
	}

	return sb.String()
}

func formatCodeBlock(content, lang string) string {
	if lang == "" {
		lang = "text"
	}
	return fmt.Sprintf("```%s\n%s```", lang, content)
}
