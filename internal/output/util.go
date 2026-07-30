// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package output

import (
	"strings"
)

// langMaps maps file extensions to language identifiers for code highlighting.
// Allocated once at package initialization to avoid repeated heap allocation.
var langMaps = map[string]string{
	// --- Common Scripts and Shell ---
	"sh":   "bash",
	"bash": "bash",
	"zsh":  "bash",
	"ps1":  "powershell",
	"bat":  "batch",
	"cmd":  "batch",

	// --- C / C++ Family ---
	"c":   "c",
	"h":   "c",
	"cpp": "cpp",
	"cc":  "cpp",
	"cxx": "cpp",
	"hpp": "cpp",
	"cs":  "csharp",

	// --- Core Backend Languages ---
	"go":    "go",
	"rs":    "rust",
	"py":    "python",
	"pyw":   "python",
	"java":  "java",
	"kt":    "kotlin",
	"kts":   "kotlin",
	"rb":    "ruby",
	"php":   "php",
	"pl":    "perl",
	"pm":    "perl",
	"swift": "swift",
	"scala": "scala",
	"erl":   "erlang",
	"hrl":   "erlang",
	"hs":    "haskell",

	// --- Frontend and Mainstream Scripts ---
	"js":   "javascript",
	"jsx":  "javascript",
	"mjs":  "javascript",
	"cjs":  "javascript",
	"ts":   "typescript",
	"tsx":  "typescript",
	"mts":  "typescript",
	"cts":  "typescript",
	"dart": "dart",
	"vue":  "vue",
	"html": "html",
	"htm":  "html",
	"css":  "css",
	"less": "less",
	"scss": "scss",
	"sass": "sass",

	// --- Data, Config, and Markup Languages ---
	"md":       "markdown",
	"markdown": "markdown",
	"json":     "json",
	"jsonc":    "json",
	"yaml":     "yaml",
	"yml":      "yaml",
	"toml":     "toml",
	"xml":      "xml",
	"xsd":      "xml",
	"sql":      "sql",
	"ini":      "ini",
	"conf":     "ini",
	"csv":      "csv",

	// --- Other Common Tech Stacks ---
	"dockerfile": "dockerfile",
	"makefile":   "makefile",
	"mk":         "makefile",
	"cmake":      "cmake",
	"proto":      "protobuf",
	"graphql":    "graphql",
	"gql":        "graphql",
	"wasm":       "wasm",
}

// LangFromExt maps file extensions to language identifiers for code highlighting.
func LangFromExt(ext string) string {
	// Remove leading dot and normalize to lowercase (to prevent match failures when user passes ".ZIP" or ".Go")
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))

	if lang, exists := langMaps[ext]; exists {
		return lang
	}

	return ""
}

// CountLines counts the number of lines in a string.
// A non-empty string that does not end with a newline is counted as having
// one more line than the number of newline characters.
func CountLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// SplitLines splits a string into lines.
func SplitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
