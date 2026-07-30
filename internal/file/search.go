// Copyright (c) 2025-2026 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package file provides search functionality for the mimi-tools MCP server.
package file

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	margs "github.com/mimichat-ai/mimi-tools/internal/args"
	"github.com/mimichat-ai/mimi-tools/internal/output"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SearchNameArgs contains the parsed arguments for the search_name tool.
type SearchNameArgs struct {
	Path           string
	Pattern        string
	Mode           *string // "regex" (default), "glob", "substring"
	CaseSensitive  *bool
	IncludeIgnored *bool
}

// SearchContentArgs contains the parsed arguments for the search_content tool.
type SearchContentArgs struct {
	Path           string
	Pattern        string
	Mode           *string // "regex" (default), "substring"
	CaseSensitive  *bool
	IncludeIgnored *bool
}

// SearchContentMatch represents a single match in search_content results.
type SearchContentMatch struct {
	Path    string   `json:"path" jsonschema:"the absolute file path containing the match"`
	LineNum int      `json:"line_num" jsonschema:"line number of the match"`
	Line    string   `json:"line" jsonschema:"the matched line content"`
	Context []string `json:"context,omitempty" jsonschema:"context lines around the match (before + after)"`
}

const maxSearchResults = 100

// HandleSearchName handles the search_name tool call.
// Arguments are accepted as raw any to tolerate LLMs that send numbers
// or booleans as strings. Values are coerced via args helpers.
func HandleSearchName(_ context.Context, _ *mcp.CallToolRequest, args any) (*mcp.CallToolResult, any, error) {
	parsed := SearchNameArgs{
		Path:           margs.GetString(args, "path"),
		Pattern:        margs.GetString(args, "pattern"),
		Mode:           margs.GetStringPtr(args, "mode"),
		CaseSensitive:  margs.GetBool(args, "case_sensitive"),
		IncludeIgnored: margs.GetBool(args, "include_ignored"),
	}

	if parsed.Path == "" {
		return output.NewTextResult(output.FormatError("search_name", output.ToolError{
			Type:       "Validation Error",
			Reason:     "Missing required argument `path`.",
			Suggestion: "Provide the absolute path to search in, e.g. `/home/user/project`.",
		}), true), nil, nil
	}
	if parsed.Pattern == "" {
		return output.NewTextResult(output.FormatError("search_name", output.ToolError{
			Type:       "Validation Error",
			Reason:     "Missing required argument `pattern`.",
			Suggestion: "Provide the pattern to search for. Use mode 'glob' for glob patterns, 'regex' for regular expressions, or 'substring' for literal strings.",
		}), true), nil, nil
	}

	// Resolve absolute path and evaluate symlinks to get canonical path.
	// This is critical on macOS where /var/folders is a symlink to /private/var/folders,
	// and git rev-parse --show-toplevel returns the canonical path.
	absPath, err := filepath.Abs(parsed.Path)
	if err != nil {
		return output.NewTextResult(output.FormatError("search_name", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to resolve path: %v", err),
		}), true), nil, nil
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return output.NewTextResult(output.FormatError("search_name", output.ToolError{
				Type:   "Path Not Found",
				Reason: "The path does not exist.",
				Target: absPath,
			}), true), nil, nil
		}
		return output.NewTextResult(output.FormatError("search_name", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to resolve symlinks: %v", err),
		}), true), nil, nil
	}
	absPath = resolvedPath

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return output.NewTextResult(output.FormatError("search_name", output.ToolError{
			Type:   "Path Not Found",
			Reason: "The path does not exist.",
			Target: absPath,
		}), true), nil, nil
	}

	// Defaults: mode="glob", case_sensitive=true, include_ignored=false
	mode := "glob"
	if parsed.Mode != nil {
		mode = *parsed.Mode
	}
	caseSensitive := true
	if parsed.CaseSensitive != nil {
		caseSensitive = *parsed.CaseSensitive
	}
	includeIgnored := false
	if parsed.IncludeIgnored != nil {
		includeIgnored = *parsed.IncludeIgnored
	}

	// If path is a file, check if filename matches the pattern
	if !info.IsDir() {
		return searchSingleFileName(absPath, parsed.Pattern, mode, caseSensitive)
	}

	// Get git ignore info unless user wants to include ignored files
	var gitRoot string
	var ignoredPaths map[string]bool
	if !includeIgnored {
		gitRoot, ignoredPaths = getGitIgnoreInfo(absPath)
	} else {
		ignoredPaths = make(map[string]bool)
	}

	// Compile pattern
	matcher, compileErr := compileNameMatcher(parsed.Pattern, mode, caseSensitive)
	if compileErr != nil {
		return output.NewTextResult(output.FormatError("search_name", output.ToolError{
			Type:   "Validation Error",
			Reason: compileErr.Error(),
		}), true), nil, nil
	}

	// Walk the directory tree
	var matches []string
	totalCount := 0

	if err := filepath.Walk(absPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // skip files we can't access
		}
		// Built-in ignore (e.g., .git) applies regardless of git ignore settings
		if !includeIgnored {
			rel, _ := filepath.Rel(absPath, path)
			if shouldIgnoreBuiltIn(rel) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if info.IsDir() {
			// Check if this directory should be ignored.
			// The search root (path == absPath) is never skipped — the caller
			// explicitly chose it as the starting point, so git-ignore rules
			// apply only to directories *within* it, not to it itself.
			if !includeIgnored && path != absPath && shouldSearchIgnoreDir(path, gitRoot, ignoredPaths) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if this file should be ignored
		if !includeIgnored && shouldSearchIgnoreFile(path, gitRoot, ignoredPaths) {
			return nil
		}

		// Check filename
		if matcher(info.Name()) {
			totalCount++
			if len(matches) < maxSearchResults {
				matches = append(matches, path)
			}
		}
		return nil
	}); err != nil {
		return output.NewTextResult(output.FormatError("search_name", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to walk directory: %v", err),
		}), true), nil, nil
	}

	searchOutput := output.SearchNameOutput{
		Pattern:   parsed.Pattern,
		Files:     matches,
		Total:     totalCount,
		IsLimited: totalCount > maxSearchResults,
	}

	md := output.FormatSearchName(searchOutput)
	return output.NewTextResult(md, false), nil, nil
}

// HandleSearchContent handles the search_content tool call.
// Arguments are accepted as raw any to tolerate LLMs that send numbers
// or booleans as strings. Values are coerced via args helpers.
func HandleSearchContent(_ context.Context, _ *mcp.CallToolRequest, args any) (*mcp.CallToolResult, any, error) {
	parsed := SearchContentArgs{
		Path:           margs.GetString(args, "path"),
		Pattern:        margs.GetString(args, "pattern"),
		Mode:           margs.GetStringPtr(args, "mode"),
		CaseSensitive:  margs.GetBool(args, "case_sensitive"),
		IncludeIgnored: margs.GetBool(args, "include_ignored"),
	}

	if parsed.Path == "" {
		return output.NewTextResult(output.FormatError("search_content", output.ToolError{
			Type:       "Validation Error",
			Reason:     "Missing required argument `path`.",
			Suggestion: "Provide the absolute path to search in, e.g. `/home/user/project`.",
		}), true), nil, nil
	}
	if parsed.Pattern == "" {
		return output.NewTextResult(output.FormatError("search_content", output.ToolError{
			Type:       "Validation Error",
			Reason:     "Missing required argument `pattern`.",
			Suggestion: "Provide the pattern to search for. Use mode 'regex' for regular expressions or 'substring' for literal strings.",
		}), true), nil, nil
	}

	// Resolve absolute path and evaluate symlinks to get canonical path.
	// This is critical on macOS where /var/folders is a symlink to /private/var/folders,
	// and git rev-parse --show-toplevel returns the canonical path.
	absPath, err := filepath.Abs(parsed.Path)
	if err != nil {
		return output.NewTextResult(output.FormatError("search_content", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to resolve path: %v", err),
		}), true), nil, nil
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return output.NewTextResult(output.FormatError("search_content", output.ToolError{
				Type:   "Path Not Found",
				Reason: "The path does not exist.",
				Target: absPath,
			}), true), nil, nil
		}
		return output.NewTextResult(output.FormatError("search_content", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to resolve symlinks: %v", err),
		}), true), nil, nil
	}
	absPath = resolvedPath

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return output.NewTextResult(output.FormatError("search_content", output.ToolError{
			Type:   "Path Not Found",
			Reason: "The path does not exist.",
			Target: absPath,
		}), true), nil, nil
	}

	// Defaults: mode="regex", case_sensitive=true, include_ignored=false
	mode := "regex"
	if parsed.Mode != nil {
		mode = *parsed.Mode
	}
	caseSensitive := true
	if parsed.CaseSensitive != nil {
		caseSensitive = *parsed.CaseSensitive
	}
	includeIgnored := false
	if parsed.IncludeIgnored != nil {
		includeIgnored = *parsed.IncludeIgnored
	}

	// If path is a file, search within that single file
	if !info.IsDir() {
		return searchSingleFile(absPath, parsed.Pattern, mode, caseSensitive)
	}

	// Get git ignore info unless user wants to include ignored files
	var gitRoot string
	var ignoredPaths map[string]bool
	if !includeIgnored {
		gitRoot, ignoredPaths = getGitIgnoreInfo(absPath)
	} else {
		ignoredPaths = make(map[string]bool)
	}

	// Compile pattern
	matcher, compileErr := compileContentMatcher(parsed.Pattern, mode, caseSensitive)
	if compileErr != nil {
		return output.NewTextResult(output.FormatError("search_content", output.ToolError{
			Type:   "Validation Error",
			Reason: compileErr.Error(),
		}), true), nil, nil
	}

	// Walk the directory tree and ignore the error since we handle SkipDir specially
	var matches []output.SearchContentMatch
	totalCount := 0
	limitReached := false

	if err := filepath.Walk(absPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // skip files we can't access
		}
		// Built-in ignore (e.g., .git) applies regardless of git ignore settings
		if !includeIgnored {
			rel, _ := filepath.Rel(absPath, path)
			if shouldIgnoreBuiltIn(rel) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if info.IsDir() {
			// Check if this directory should be ignored.
			// The search root (path == absPath) is never skipped — the caller
			// explicitly chose it as the starting point, so git-ignore rules
			// apply only to directories *within* it, not to it itself.
			if !includeIgnored && path != absPath && shouldSearchIgnoreDir(path, gitRoot, ignoredPaths) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if this file should be ignored
		if !includeIgnored && shouldSearchIgnoreFile(path, gitRoot, ignoredPaths) {
			return nil
		}

		// Skip binary files
		if isBinaryFile(path) {
			return nil
		}

		// Search in file content
		fileMatches, readErr := searchInFile(path, matcher)
		if readErr != nil {
			return nil // skip files we can't read
		}

		totalCount += len(fileMatches)
		if len(matches) < maxSearchResults {
			remaining := maxSearchResults - len(matches)
			if len(fileMatches) > remaining {
				matches = append(matches, fileMatches[:remaining]...)
				limitReached = true
				return nil
			}
			matches = append(matches, fileMatches...)
		}
		return nil
	}); err != nil {
		return output.NewTextResult(output.FormatError("search_content", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to walk directory: %v", err),
		}), true), nil, nil
	}

	searchOutput := output.SearchContentOutput{
		Pattern:   parsed.Pattern,
		Matches:   matches,
		Total:     totalCount,
		IsLimited: limitReached,
	}

	md := output.FormatSearchContent(searchOutput)
	return output.NewTextResult(md, false), nil, nil
}

// shouldSearchIgnoreDir checks if a directory should be ignored.
func shouldSearchIgnoreDir(fullPath, gitRoot string, ignoredPaths map[string]bool) bool {
	// Check git ignore
	if gitRoot != "" {
		relToRepo, err := filepath.Rel(gitRoot, fullPath)
		if err == nil {
			// Check if directory itself is ignored
			if ignoredPaths[relToRepo] || ignoredPaths["/"+relToRepo] {
				return true
			}
		}
	}

	return false
}

// shouldSearchIgnoreFile checks if a file should be ignored based on git ignore rules.
func shouldSearchIgnoreFile(fullPath, gitRoot string, ignoredPaths map[string]bool) bool {
	// Check git ignore
	if gitRoot != "" {
		relToRepo, err := filepath.Rel(gitRoot, fullPath)
		if err == nil {
			if ignoredPaths[relToRepo] || ignoredPaths["/"+relToRepo] {
				return true
			}
		}
	}

	return false
}

// isBinaryFile checks if a file is binary by reading the first 512 bytes.
func isBinaryFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()

	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	if n == 0 {
		return false // empty file is not binary
	}

	// 1. NUL byte -> binary
	for i := range n {
		if buf[i] == 0 {
			return true
		}
	}

	// 2. Not valid UTF-8 -> binary
	// Trim incomplete UTF-8 sequence at the buffer boundary to avoid false positives
	// when a multi-byte character is split across the 512-byte chunk.
	// We only trim up to 3 bytes (max UTF-8 character is 4 bytes).
	validLen := n
	if !utf8.Valid(buf[:n]) {
		// Try trimming up to 3 bytes from the end to handle split multi-byte chars
		for trim := 1; trim <= 3 && trim < validLen; trim++ {
			if utf8.Valid(buf[:n-trim]) {
				validLen = n - trim
				break
			}
		}
	}
	if validLen > 0 && !utf8.Valid(buf[:validLen]) {
		return true
	}

	// 3. High ratio of non-printable control characters -> binary
	nonPrint := 0
	for i := range n {
		b := buf[i]
		if b < 0x20 && b != '\n' && b != '\r' && b != '\t' {
			nonPrint++
		}
	}
	if float64(nonPrint)/float64(n) > 0.30 { // >30% control chars
		return true
	}
	return false
}

// searchSingleFile searches for pattern matches in a single file.
func searchSingleFile(path, pattern, mode string, caseSensitive bool) (*mcp.CallToolResult, any, error) {
	// Compile pattern
	matcher, compileErr := compileContentMatcher(pattern, mode, caseSensitive)
	if compileErr != nil {
		return output.NewTextResult(output.FormatError("search_content", output.ToolError{
			Type:   "Validation Error",
			Reason: compileErr.Error(),
		}), true), nil, nil
	}

	// Skip binary files
	if isBinaryFile(path) {
		return output.NewTextResult(output.FormatError("search_content", output.ToolError{
			Type:   "Validation Error",
			Reason: "Cannot search binary file.",
			Target: path,
		}), true), nil, nil
	}

	fileMatches, readErr := searchInFile(path, matcher)
	if readErr != nil {
		return output.NewTextResult(output.FormatError("search_content", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to read file: %v", readErr),
			Target: path,
		}), true), nil, nil
	}

	totalCount := len(fileMatches)
	limitReached := false
	if totalCount > maxSearchResults {
		fileMatches = fileMatches[:maxSearchResults]
		limitReached = true
	}

	searchOutput := output.SearchContentOutput{
		Pattern:   pattern,
		Matches:   fileMatches,
		Total:     totalCount,
		IsLimited: limitReached,
	}

	md := output.FormatSearchContent(searchOutput)
	return output.NewTextResult(md, false), nil, nil
}

// searchSingleFileName checks if a single file's name matches the pattern.
func searchSingleFileName(path, pattern, mode string, caseSensitive bool) (*mcp.CallToolResult, any, error) {
	// Compile pattern
	matcher, compileErr := compileNameMatcher(pattern, mode, caseSensitive)
	if compileErr != nil {
		return output.NewTextResult(output.FormatError("search_name", output.ToolError{
			Type:   "Validation Error",
			Reason: compileErr.Error(),
		}), true), nil, nil
	}

	fileName := filepath.Base(path)
	var matches []string
	totalCount := 0
	if matcher(fileName) {
		totalCount = 1
		matches = append(matches, path)
	}

	searchOutput := output.SearchNameOutput{
		Pattern:   pattern,
		Files:     matches,
		Total:     totalCount,
		IsLimited: false,
	}

	md := output.FormatSearchName(searchOutput)
	return output.NewTextResult(md, false), nil, nil
}

// globToRegex converts a shell glob pattern to a regular expression.
// It supports *, ?, and [...] glob syntax (same as filepath.Match).
// The returned regex is anchored (^...$) for full-string matching.
func globToRegex(pattern string) string {
	var buf strings.Builder
	buf.WriteByte('^')

	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '\\':
			// Escape next character
			if i+1 < len(pattern) {
				next := pattern[i+1]
				if isRegexSpecial(next) {
					buf.WriteByte('\\')
				}
				buf.WriteByte(next)
				i++
			} else {
				buf.WriteString("\\\\") // trailing backslash, escape it
			}
		case '*':
			buf.WriteString(".*")
		case '?':
			buf.WriteString(".")
		case '[':
			// Character class
			j := i + 1
			if j >= len(pattern) {
				buf.WriteString("\\[")
				continue
			}
			buf.WriteByte('[')
			// Handle negation
			if pattern[j] == '!' || pattern[j] == '^' {
				buf.WriteByte('^')
				j++
			}
			// Copy until ']'
			for j < len(pattern) && pattern[j] != ']' {
				if pattern[j] == '\\' && j+1 < len(pattern) {
					buf.WriteByte(pattern[j])
					j++
					buf.WriteByte(pattern[j])
					j++
				} else {
					buf.WriteByte(pattern[j])
					j++
				}
			}
			if j >= len(pattern) {
				buf.WriteString("\\]") // unclosed, escape the bracket
				continue
			}
			buf.WriteByte(']')
			i = j
		default:
			if isRegexSpecial(c) {
				buf.WriteByte('\\')
			}
			buf.WriteByte(c)
		}
	}

	buf.WriteByte('$')
	return buf.String()
}

// isRegexSpecial returns true if the byte is a regex metacharacter that needs escaping.
func isRegexSpecial(c byte) bool {
	return strings.ContainsRune(`\.+*?()|[]{}^$`, rune(c))
}

// compileNameMatcher compiles a file name matcher for the given pattern and mode.
// Supports "glob", "regex", and "substring" modes.
func compileNameMatcher(pattern, mode string, caseSensitive bool) (func(string) bool, error) {
	switch mode {
	case "regex":
		p := pattern
		if !caseSensitive {
			p = "(?i)" + pattern
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %v", err)
		}
		return func(name string) bool { return re.MatchString(name) }, nil
	case "glob":
		regex := globToRegex(pattern)
		if !caseSensitive {
			regex = "(?i)" + regex
		}
		re, err := regexp.Compile(regex)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern: %v", err)
		}
		return func(name string) bool { return re.MatchString(name) }, nil
	case "substring":
		if caseSensitive {
			return func(name string) bool { return strings.Contains(name, pattern) }, nil
		}
		lp := strings.ToLower(pattern)
		return func(name string) bool { return strings.Contains(strings.ToLower(name), lp) }, nil
	default:
		return nil, fmt.Errorf("invalid mode %q: must be \"regex\", \"glob\", or \"substring\"", mode)
	}
}

// compileContentMatcher compiles a content matcher for the given pattern and mode.
// Supports "regex" and "substring" modes (not "glob").
func compileContentMatcher(pattern, mode string, caseSensitive bool) (func(string) bool, error) {
	switch mode {
	case "regex":
		p := pattern
		if !caseSensitive {
			p = "(?i)" + pattern
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %v", err)
		}
		return func(content string) bool { return re.MatchString(content) }, nil
	case "substring":
		if caseSensitive {
			return func(content string) bool { return strings.Contains(content, pattern) }, nil
		}
		lp := strings.ToLower(pattern)
		return func(content string) bool { return strings.Contains(strings.ToLower(content), lp) }, nil
	case "glob":
		return nil, fmt.Errorf("mode \"glob\" is not supported for search_content, use \"regex\" or \"substring\"")
	default:
		return nil, fmt.Errorf("invalid mode %q: must be \"regex\" or \"substring\"", mode)
	}
}

// searchInFile searches for pattern matches in a file and returns matches with context.
func searchInFile(path string, matcher func(string) bool) ([]output.SearchContentMatch, error) {
	// Skip files larger than 10 MB to avoid OOM on large text files
	const maxSearchFileSize = 10 << 20 // 10 MB
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxSearchFileSize {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	// Read all lines into memory so we can provide context.
	// Use bufio.Reader to avoid the 64KB line limit of bufio.Scanner.
	reader := bufio.NewReader(file)
	var allLines []string
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			if !utf8.ValidString(line) {
				// Not valid UTF-8, treat as binary and skip
				return nil, nil
			}
			allLines = append(allLines, line)
		}
		if readErr != nil {
			if readErr != io.EOF {
				return nil, readErr
			}
			break
		}
	}

	if len(allLines) == 0 {
		return nil, nil
	}

	contextLines := 3
	var matches []output.SearchContentMatch

	for lineNum, line := range allLines {
		if matcher(line) {
			// Build context: up to 3 lines before and after
			startCtx := lineNum - contextLines
			if startCtx < 0 {
				startCtx = 0
			}
			endCtx := lineNum + contextLines + 1 // +1 because slice end is exclusive
			if endCtx > len(allLines) {
				endCtx = len(allLines)
			}

			context := make([]string, 0, endCtx-startCtx)
			for i := startCtx; i < endCtx; i++ {
				prefix := "  "
				if i == lineNum {
					prefix = "> "
				}
				context = append(context, fmt.Sprintf("%s%d: %s", prefix, i+1, allLines[i]))
			}

			matches = append(matches, output.SearchContentMatch{
				Path:    path,
				LineNum: lineNum + 1,
				Line:    line,
				Context: context,
			})
		}
	}

	return matches, nil
}
