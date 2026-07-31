// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package file provides file editing functionality for the mimi-tools MCP server.
package file

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	margs "github.com/mimichat-ai/mimi-tools/internal/args"
	"github.com/mimichat-ai/mimi-tools/internal/output"

	"github.com/agnivade/levenshtein"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// EditArgs contains the parsed arguments for the edit tool.
type EditArgs struct {
	Path      string
	OldString string
	NewString string
}

// findOccurrences performs exact substring matching (no normalization).
func findOccurrences(content, oldString string) (count, firstStart, firstEnd int, found bool) {
	if oldString == "" {
		return 0, -1, -1, false
	}
	start := 0
	for {
		idx := strings.Index(content[start:], oldString)
		if idx == -1 {
			break
		}
		count++
		if !found {
			firstStart = start + idx
			firstEnd = start + idx + len(oldString)
			found = true
		}
		start += idx + len(oldString)
		if start >= len(content) {
			break
		}
	}
	return count, firstStart, firstEnd, found
}

const contextLinesCount = 3 // number of context lines before/after the change

func generateFragmentDiff(content string, matchStart int, matchEnd int, newString string) string {

	// --- Line / column for the change header ---
	lineNum := 1
	for i := range matchStart {
		if content[i] == '\n' {
			lineNum++
		}
	}
	colNum := 1
	for i := matchStart - 1; i >= 0; i-- {
		if content[i] == '\n' {
			break
		}
		colNum++
	}

	// --- Context window (byte indices) ---
	ctxStart := 0
	ctxEnd := len(content)
	// Start searching from the newline BEFORE matchStart's line
	searchStart := matchStart
	idx := strings.LastIndexByte(content[:searchStart], '\n')
	if idx != -1 {
		searchStart = idx
	}
	for i := 0; i < contextLinesCount && searchStart > 0; i++ {
		idx := strings.LastIndexByte(content[:searchStart], '\n')
		if idx == -1 {
			ctxStart = 0
			break
		}
		ctxStart = idx + 1
		searchStart = idx
	}
	// Start searching from AFTER matchEnd's newline (if present)
	cur := matchEnd
	if cur < len(content) && content[cur] == '\n' {
		cur++
	}
	for i := 0; i < contextLinesCount && cur < len(content); i++ {
		idx := strings.IndexByte(content[cur:], '\n')
		if idx == -1 {
			ctxEnd = len(content)
			break
		}
		ctxEnd = cur + idx + 1
		cur += idx + 1
	}

	// Build old/new fragments for diff comparison.
	ctxBefore := content[ctxStart:matchStart]
	oldFragment := ctxBefore + content[matchStart:matchEnd] + content[matchEnd:ctxEnd]
	newFragment := ctxBefore + newString + content[matchEnd:ctxEnd]

	var diffs []diffmatchpatch.Diff
	dmp := diffmatchpatch.New()
	if strings.Contains(oldFragment, "\n") || strings.Contains(newFragment, "\n") {
		text1, text2, lineArray := dmp.DiffLinesToChars(oldFragment, newFragment)
		diffs = dmp.DiffMain(text1, text2, false)
		diffs = dmp.DiffCleanupSemantic(diffs)
		diffs = dmp.DiffCharsToLines(diffs, lineArray)
	} else {
		diffs = dmp.DiffMain(oldFragment, newFragment, false)
		diffs = dmp.DiffCleanupSemantic(diffs)
	}

	ctxBeforeLineCount := strings.Count(ctxBefore, "\n")
	oldStart := lineNum - ctxBeforeLineCount
	return renderDiffHunk(diffs, oldStart, oldStart, colNum)
}

// renderDiffHunk renders diffs with unified-diff-style header and dual line numbers
// (old file / new file). col is the column number of the change; 0 means omit.
func renderDiffHunk(diffs []diffmatchpatch.Diff, oldStart, newStart, col int) string {
	// First pass: count lines in old and new file
	oldCount, newCount := 0, 0
	for _, d := range diffs {
		if strings.TrimSpace(d.Text) == "" && d.Type == diffmatchpatch.DiffEqual {
			continue
		}
		lines := strings.Split(d.Text, "\n")
		for j, line := range lines {
			if line == "" && j == len(lines)-1 {
				continue
			}
			switch d.Type {
			case diffmatchpatch.DiffEqual:
				oldCount++
				newCount++
			case diffmatchpatch.DiffDelete:
				oldCount++
			case diffmatchpatch.DiffInsert:
				newCount++
			}
		}
	}

	var sb strings.Builder

	// Header
	if col > 0 {
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@ Col %d\n", oldStart, oldCount, newStart, newCount, col)
	} else {
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
	}

	// Second pass: render lines with dual line numbers
	oldLine, newLine := oldStart, newStart
	for _, d := range diffs {
		if strings.TrimSpace(d.Text) == "" && d.Type == diffmatchpatch.DiffEqual {
			continue
		}
		marker := " "
		switch d.Type {
		case diffmatchpatch.DiffDelete:
			marker = "-"
		case diffmatchpatch.DiffInsert:
			marker = "+"
		}
		lines := strings.Split(d.Text, "\n")
		for j, line := range lines {
			if line == "" && j == len(lines)-1 {
				continue
			}
			sb.WriteString(marker)
			sb.WriteString(" ")
			switch d.Type {
			case diffmatchpatch.DiffDelete:
				fmt.Fprintf(&sb, "%4d %4s: %s", oldLine, "", line)
				oldLine++
			case diffmatchpatch.DiffInsert:
				fmt.Fprintf(&sb, "%4s %4d: %s", "", newLine, line)
				newLine++
			case diffmatchpatch.DiffEqual:
				fmt.Fprintf(&sb, "%4d %4d: %s", oldLine, newLine, line)
				oldLine++
				newLine++
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func HandleEdit(_ context.Context, _ *mcp.CallToolRequest, args any) (*mcp.CallToolResult, any, error) {
	parsed := EditArgs{
		Path:      margs.GetString(args, "path"),
		OldString: margs.GetString(args, "old_string"),
	}

	if parsed.Path == "" {
		return output.NewTextResult(output.FormatError("edit", output.ToolError{
			Type:       "Validation Error",
			Reason:     "Missing required argument `path`.",
			Suggestion: "Provide the absolute file path to edit, e.g. `/home/user/project/main.go`.",
		}), true), nil, nil
	}

	newStringPtr := margs.GetStringPtr(args, "new_string")
	if newStringPtr == nil {
		return output.NewTextResult(output.FormatError("edit", output.ToolError{
			Type:       "Validation Error",
			Reason:     "Missing required argument `new_string`.",
			Suggestion: "Provide the replacement text. Use an empty string to delete the matched text.",
		}), true), nil, nil
	}
	parsed.NewString = *newStringPtr

	// Resolve absolute path for consistent locking
	absPath, err := filepath.Abs(parsed.Path)
	if err != nil {
		return output.NewTextResult(output.FormatError("edit", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to resolve path: %v", err),
		}), true), nil, nil
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return output.NewTextResult(output.FormatError("edit", output.ToolError{
			Type:   "File Not Found",
			Reason: "The file does not exist.",
			Target: parsed.Path,
		}), true), nil, nil
	}

	if info.IsDir() {
		return output.NewTextResult(output.FormatError("edit", output.ToolError{
			Type:   "Validation Error",
			Reason: "Path is a directory.",
			Target: parsed.Path,
		}), true), nil, nil
	}

	// Acquire file lock
	fl := getMutex(absPath)
	fl.Lock()
	defer releaseMutex(fl)

	rawContent, err := os.ReadFile(absPath)
	if err != nil {
		return output.NewTextResult(output.FormatError("edit", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to read file: %v", err),
		}), true), nil, nil
	}

	originalContent := string(rawContent)

	if parsed.OldString == "" {
		return output.NewTextResult(output.FormatError("edit", output.ToolError{
			Type:       "Validation Error",
			Reason:     "old_string cannot be empty.",
			Suggestion: "Provide the exact text in the file that you want to replace.",
		}), true), nil, nil
	}

	if parsed.OldString == parsed.NewString {
		return output.NewTextResult(output.FormatError("edit", output.ToolError{
			Type:   "Validation Error",
			Reason: "old_string and new_string must be different.",
		}), true), nil, nil
	}

	occurrences, matchStart, matchEnd, found := findOccurrences(originalContent, parsed.OldString)
	var bestMatch *output.MatchSuggestion
	var ambiguous bool

	if !found {
		// Exact match failed, try fuzzy matching
		bestMatch, ambiguous = fuzzyMatch(originalContent, parsed.OldString)

		if bestMatch != nil && bestMatch.Similarity > 0.95 && !ambiguous {
			// Fuzzy match with high similarity - proceed with replacement
			matchStart = bestMatch.ByteStart
			matchEnd = bestMatch.ByteEnd
		} else if bestMatch != nil && bestMatch.Similarity > 0.95 && ambiguous {
			// Multiple non-overlapping fuzzy matches found
			return output.NewTextResult(output.FormatError("edit", output.ToolError{
				Type:       "Ambiguous Match",
				Reason:     "Found multiple non-overlapping fuzzy matches with similarity > 0.95.",
				Suggestion: "Please provide a more specific string with surrounding context.",
				BestMatch:  bestMatch,
			}), true), nil, nil
		} else {
			// No suitable fuzzy match found, return error
			return output.NewTextResult(output.FormatError("edit", output.ToolError{
				Type:      "String Not Found",
				Reason:    "String not found in file and no high-similarity match found.",
				Target:    parsed.Path,
				BestMatch: bestMatch,
			}), true), nil, nil
		}
	} else {
		// Exact match found - standard behavior
		if occurrences > 1 {
			return output.NewTextResult(output.FormatError("edit", output.ToolError{
				Type:       "Ambiguous Match",
				Reason:     fmt.Sprintf("Found %d occurrences of the string.", occurrences),
				Suggestion: "Please provide a more specific string with surrounding context.",
			}), true), nil, nil
		}
	}

	// Generate diff and perform replacement
	var diff string
	var matchScore float64

	if bestMatch != nil && bestMatch.Similarity > 0.95 {
		// This is a fuzzy match replacement
		diff = generateFuzzyDiff(parsed.OldString, originalContent[matchStart:matchEnd],
			// Calculate line number for byteStart
			func() int {
				lines := strings.Split(originalContent[:matchStart], "\n")
				return len(lines)
			}())
		matchScore = bestMatch.Similarity
	} else {
		// Exact match replacement
		diff = generateFragmentDiff(originalContent, matchStart, matchEnd, parsed.NewString)
		matchScore = 0.0 // Indicates exact match
	}

	// Perform replacement
	newContent := originalContent[:matchStart] + parsed.NewString + originalContent[matchEnd:]

	// Atomic Write: Create temp file, write content, and rename
	if err := atomicWriteFile(absPath, newContent); err != nil {
		return output.NewTextResult(output.FormatError("edit", output.ToolError{
			Type:   "Internal Error",
			Reason: err.Error(),
		}), true), nil, nil
	}

	editOutput := output.EditOutput{
		Path:       absPath,
		Diff:       diff,
		Applied:    true,
		MatchScore: matchScore,
	}

	md := output.FormatEdit(editOutput)
	return output.NewTextResult(md, false), nil, nil
}

// atomicWriteFile writes content to a file atomically by creating a temporary
// file in the same directory, then renaming it to the target path.
// The original file's permissions are preserved if the file already exists.
// The temporary file is always cleaned up, even on failure.
func atomicWriteFile(targetPath, content string) error {
	dir := filepath.Dir(targetPath)

	// Preserve original file permissions before overwriting.
	var origMode os.FileMode
	if info, err := os.Stat(targetPath); err == nil {
		origMode = info.Mode().Perm()
	}

	tmpFile, err := os.CreateTemp(dir, ".mimi-tool-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	tmpPath := tmpFile.Name()
	success := false
	defer func() {
		_ = tmpFile.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.WriteString(content); err != nil {
		return fmt.Errorf("failed to write temporary file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %v", err)
	}

	// Set permissions on the temp file to match the original before rename,
	// so the renamed file has the correct permissions from the start.
	if origMode != 0 {
		if err := os.Chmod(tmpPath, origMode); err != nil {
			return fmt.Errorf("failed to set permissions on temporary file: %v", err)
		}
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to replace original file: %v", err)
	}
	success = true
	return nil
}

const tabWidth = 4

var tabSpaces = strings.Repeat(" ", tabWidth)

// fuzzyNormalize prepares text for fuzzy matching by:
// 1. Converting tabs to tabWidth spaces in leading whitespace (indentation)
// 2. Preserving leading whitespace to distinguish indentation levels
// 3. Stripping non-leading whitespace (internal and trailing)
// 4. Lowercasing all characters
//
// This ensures that tab-vs-space differences at the same indentation level
// produce identical normalized strings (high similarity), while different
// indentation levels produce different normalized strings (lower similarity),
// preventing false ambiguous matches.
func fuzzyNormalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	atLineStart := true
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '\n' {
			b.WriteRune('\n')
			atLineStart = true
		} else if unicode.IsSpace(r) {
			if atLineStart && r != '\r' {
				if r == '\t' {
					b.WriteString(tabSpaces)
				} else {
					b.WriteRune(r)
				}
			}
			// non-leading whitespace and \r are skipped
		} else {
			b.WriteRune(unicode.ToLower(r))
			atLineStart = false
		}
		i += size
	}
	return b.String()
}

// generateFuzzyDiff generates a diff between oldString and matchedContent.
// Both are assumed to have the same number of lines.
// startLine is the 1-based line number where matchedContent appears in the file.
func generateFuzzyDiff(oldString, matchedContent string, startLine int) string {
	dmp := diffmatchpatch.New()
	var diffs []diffmatchpatch.Diff

	if strings.Contains(oldString, "\n") || strings.Contains(matchedContent, "\n") {
		text1, text2, lineArray := dmp.DiffLinesToChars(oldString, matchedContent)
		diffs = dmp.DiffMain(text1, text2, false)
		diffs = dmp.DiffCleanupSemantic(diffs)
		diffs = dmp.DiffCharsToLines(diffs, lineArray)
	} else {
		diffs = dmp.DiffMain(oldString, matchedContent, false)
		diffs = dmp.DiffCleanupSemantic(diffs)
	}

	return renderDiffHunk(diffs, startLine, startLine, 0)
}

type matchCandidate struct {
	lineStart int
	lineEnd   int
	byteStart int
	byteEnd   int
	content   string
	score     float64
}

// fuzzyMatch returns the best matching multi-line window to oldString within content.
// The window size equals the number of lines in oldString.
// Returns nil if no match with similarity > 0.7 is found.
// The ambiguous return is true when there are multiple non-overlapping windows
// with similarity > 0.95, indicating the match is not unique.
func fuzzyMatch(content, oldString string) (*output.MatchSuggestion, bool) {
	lines := strings.Split(content, "\n")
	normOld := fuzzyNormalize(oldString)
	if normOld == "" {
		return nil, false
	}
	oldLineCount := strings.Count(oldString, "\n") + 1
	if oldLineCount > len(lines) {
		return nil, false
	}

	// Precompute byte offsets for each line start
	lineByteOffsets := make([]int, len(lines)+1)
	offset := 0
	for i, line := range lines {
		lineByteOffsets[i] = offset
		// Add line length + 1 for newline, but only if this is not the last line
		// OR if this is the last line and it's empty (indicating file ends with newline)
		if i < len(lines)-1 {
			// Not the last line, always add newline
			offset += len(line) + 1
		} else {
			// Last line
			if line == "" {
				// File ends with newline, but the newline was already counted
				// in the previous line. So just add the line length (0).
				offset += len(line)
			} else {
				// File does not end with newline
				offset += len(line)
			}
		}
	}
	lineByteOffsets[len(lines)] = offset // EOF position

	var bestCandidate *matchCandidate
	// Track all windows with score > 0.95 for ambiguity detection.
	// These are stored by value (not pointer) because candidate is a
	// loop variable that would be reused.
	var highScoreCandidates []matchCandidate

	for i := 0; i <= len(lines)-oldLineCount; i++ {
		j := i + oldLineCount
		window := strings.Join(lines[i:j], "\n")
		normWindow := fuzzyNormalize(window)
		dist := levenshtein.ComputeDistance(normOld, normWindow)
		maxLen := len(normOld)
		if len(normWindow) > maxLen {
			maxLen = len(normWindow)
		}
		var score float64
		if maxLen == 0 {
			score = 1.0
		} else {
			score = 1.0 - float64(dist)/float64(maxLen)
		}
		if score > 0.7 {
			// Calculate byte positions
			byteStart := lineByteOffsets[i]
			byteEnd := lineByteOffsets[j]

			candidate := matchCandidate{
				lineStart: i + 1,
				lineEnd:   j,
				byteStart: byteStart,
				byteEnd:   byteEnd,
				content:   window,
				score:     score,
			}
			if bestCandidate == nil || score > bestCandidate.score {
				bestCandidate = &candidate
			}
			if score > 0.95 {
				highScoreCandidates = append(highScoreCandidates, candidate)
			}
		}
	}
	if bestCandidate == nil {
		return nil, false
	}

	// Check for ambiguity: is there a high-scoring candidate (score > 0.95)
	// whose byte range does NOT overlap with the best candidate?
	// Overlapping windows are expected (sliding window shares lines),
	// so only non-overlapping windows indicate a true ambiguous match.
	ambiguous := false
	for _, c := range highScoreCandidates {
		if !rangesOverlap(bestCandidate.byteStart, bestCandidate.byteEnd, c.byteStart, c.byteEnd) {
			ambiguous = true
			break
		}
	}

	diff := generateFuzzyDiff(oldString, bestCandidate.content, bestCandidate.lineStart)
	return &output.MatchSuggestion{
		LineStart:  bestCandidate.lineStart,
		LineEnd:    bestCandidate.lineEnd,
		ByteStart:  bestCandidate.byteStart,
		ByteEnd:    bestCandidate.byteEnd,
		Similarity: math.Round(bestCandidate.score*1000) / 1000,
		Diff:       diff,
	}, ambiguous
}

// rangesOverlap returns true if [a1, a2) and [b1, b2) share at least one byte.
func rangesOverlap(a1, a2, b1, b2 int) bool {
	return a1 < b2 && b1 < a2
}
