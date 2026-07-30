// Copyright (c) 2025-2026 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package output

import (
	"strings"
	"testing"
)

func TestCountLines(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"empty string", "", 0},
		{"single line no newline", "hello", 1},
		{"single line with newline", "hello\n", 1},
		{"two lines no trailing newline", "line1\nline2", 2},
		{"two lines with trailing newline", "line1\nline2\n", 2},
		{"only newlines", "\n\n\n", 3},
		{"trailing empty line via newline", "a\n", 1},
		{"multiple blank lines", "\n\n", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountLines(tt.s)
			if got != tt.want {
				t.Errorf("CountLines(%q) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}

func TestLangFromExt(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		want string
	}{
		// With leading dot
		{"go with dot", ".go", "go"},
		{"py with dot", ".py", "python"},
		{"ts with dot", ".ts", "typescript"},
		{"js with dot", ".js", "javascript"},
		{"md with dot", ".md", "markdown"},
		{"json with dot", ".json", "json"},
		{"yaml with dot", ".yaml", "yaml"},
		{"yml with dot", ".yml", "yaml"},
		{"dockerfile with dot", ".dockerfile", "dockerfile"},
		// Without leading dot
		{"go without dot", "go", "go"},
		{"py without dot", "py", "python"},
		// Case insensitivity
		{"uppercase GO", ".GO", "go"},
		{"uppercase PY", ".PY", "python"},
		{"mixed case Ts", ".Ts", "typescript"},
		// Unknown / edge cases
		{"unknown extension", ".xyz", ""},
		{"empty string", "", ""},
		{"just a dot", ".", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LangFromExt(tt.ext)
			if got != tt.want {
				t.Errorf("LangFromExt(%q) = %q, want %q", tt.ext, got, tt.want)
			}
		})
	}
}

func TestFormatTree_IgnoredMarker(t *testing.T) {
	treeOutput := TreeOutput{
		Root:       "/tmp/project",
		MaxDepth:   5,
		TotalDirs:  1,
		TotalFiles: 1,
		Nodes: []Node{
			{
				Name:   "project",
				IsDir:  true,
				IsLast: true,
				Children: []Node{
					{Name: "src", IsDir: true, IsLast: false, Children: []Node{
						{Name: "main.go", IsLast: true},
					}},
					{Name: "node_modules", IsDir: true, IsLast: false, Ignored: true},
					{Name: "vendor", IsDir: true, IsLast: false, Ignored: true},
					{Name: "README.md", IsLast: true},
				},
			},
		},
	}

	result := FormatTree(treeOutput)

	// The (ignored) marker must appear for ignored directories.
	if !strings.Contains(result, "node_modules/ (ignored)") {
		t.Errorf("expected '(ignored)' marker for node_modules, got:\n%s", result)
	}
	if !strings.Contains(result, "vendor/ (ignored)") {
		t.Errorf("expected '(ignored)' marker for vendor, got:\n%s", result)
	}

	// Non-ignored directories must NOT have the marker.
	if strings.Contains(result, "src/ (ignored)") {
		t.Errorf("src/ should not have '(ignored)' marker, got:\n%s", result)
	}

	// Verify 4-space indentation is used (not the broken 1-space).
	// Level-1 children of the root are rendered with no prefix (by design).
	// Level-2 children under a non-last parent should use "│   " (4 chars).
	if !strings.Contains(result, "│   └── main.go") {
		t.Errorf("expected proper 4-char indentation for main.go, got:\n%s", result)
	}
	// The broken 2-char indentation ("│ └──") must NOT appear.
	if strings.Contains(result, "│ └──") {
		t.Errorf("should not use broken 2-char indentation, got:\n%s", result)
	}
}
