// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package file provides file reading functionality for the mimi-tools MCP server.
package file

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	margs "github.com/mimichat-ai/mimi-tools/internal/args"
	"github.com/mimichat-ai/mimi-tools/internal/output"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// maxReadFileSize is the maximum file size (100 MiB) that the read tool will
// attempt to load into memory. Files larger than this will return an error
// to prevent OOM.
const maxReadFileSize = 100 << 20 // 100 MiB

// ReadArgs contains the parsed arguments for the read tool.
type ReadArgs struct {
	Path      string
	StartLine *int
	EndLine   *int
}

// ReadResult contains the result of a file read operation.
type ReadResult struct {
	Content    string `json:"content" jsonschema:"the file content (empty if file is empty)"`
	TotalLines int    `json:"total_lines" jsonschema:"total number of lines in the file"`
	StartLine  int    `json:"start_line" jsonschema:"actual start line that was read"`
	EndLine    int    `json:"end_line" jsonschema:"actual end line that was read"`
	IsEmpty    bool   `json:"is_empty" jsonschema:"true if the file is empty"`
}

// HandleRead handles the read tool call.
// Arguments are accepted as raw any to tolerate LLMs that send numbers
// or booleans as strings. Values are coerced via args helpers.
func HandleRead(_ context.Context, _ *mcp.CallToolRequest, args any) (*mcp.CallToolResult, any, error) {
	parsed := ReadArgs{
		Path:      margs.GetString(args, "path"),
		StartLine: margs.GetInt(args, "start_line"),
		EndLine:   margs.GetInt(args, "end_line"),
	}

	if parsed.Path == "" {
		return output.NewTextResult(output.FormatError("read", output.ToolError{
			Type:       "Validation Error",
			Reason:     "Missing required argument `path`.",
			Suggestion: "Provide the absolute file path to read, e.g. `/home/user/project/main.go`.",
		}), true), nil, nil
	}

	// Resolve absolute path
	absPath, err := filepath.Abs(parsed.Path)
	if err != nil {
		return output.NewTextResult(output.FormatError("read", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to resolve path: %v", err),
		}), true), nil, nil
	}
	parsed.Path = absPath

	// Check if path is a directory
	info, err := os.Stat(parsed.Path)
	if err != nil {
		return output.NewTextResult(output.FormatError("read", output.ToolError{
			Type:   "File Not Found",
			Reason: "The file does not exist.",
			Target: parsed.Path,
		}), true), nil, nil
	}

	if info.IsDir() {
		// Directory fallback: render tree structure
		treeResult, err := getDirTree(parsed.Path, 5)
		if err != nil {
			return output.NewTextResult(output.FormatError("read", output.ToolError{
				Type:   "Internal Error",
				Reason: fmt.Sprintf("Failed to read directory tree: %v", err),
			}), true), nil, nil
		}

		treeOutput := output.TreeOutput{
			Root:       treeResult.Root,
			MaxDepth:   treeResult.MaxDepth,
			TotalDirs:  treeResult.TotalDirs,
			TotalFiles: treeResult.TotalFiles,
			Nodes:      treeResult.Tree,
		}

		readOutput := output.ReadOutput{
			IsDir: true,
			Path:  parsed.Path,
			Tree:  output.FormatTree(treeOutput), // FormatTree renders content block only
		}

		md := output.FormatRead(readOutput)
		return output.NewTextResult(md, false), nil, nil
	}

	// Check file size to prevent OOM
	if info.Size() > maxReadFileSize {
		sizeMiB := info.Size() / (1 << 20)
		return output.NewTextResult(output.FormatError("read", output.ToolError{
			Type:   "Validation Error",
			Reason: fmt.Sprintf("File is too large to read (%d MiB). Maximum allowed is 100 MiB.", sizeMiB),
			Target: parsed.Path,
		}), true), nil, nil
	}

	// Parse optional line range parameters with defaults
	startLine := 1
	if parsed.StartLine != nil {
		startLine = *parsed.StartLine
	}
	endLine := -1
	if parsed.EndLine != nil {
		endLine = *parsed.EndLine
	}

	result, err := readFile(parsed.Path, startLine, endLine)
	if err != nil {
		return output.NewTextResult(output.FormatError("read", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to read file: %v", err),
		}), true), nil, nil
	}

	readOutput := output.ReadOutput{
		Path:       parsed.Path,
		Content:    result.Content,
		TotalLines: result.TotalLines,
		StartLine:  result.StartLine,
		EndLine:    result.EndLine,
		IsEmpty:    result.IsEmpty,
	}

	md := output.FormatRead(readOutput)
	return output.NewTextResult(md, false), nil, nil
}

// readFile reads a file and returns content within the specified line range.
func readFile(path string, startLine, endLine int) (*ReadResult, error) {
	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %v", err)
	}
	defer func() { _ = file.Close() }()

	// Validate start line
	if startLine < 1 {
		return nil, fmt.Errorf("invalid start_line: must be greater than 0")
	}

	// Validate line range
	if endLine > 0 && startLine > endLine {
		return nil, fmt.Errorf("invalid line range: start_line (%d) cannot be greater than end_line (%d)", startLine, endLine)
	}

	// Set default end line to 0 (will be set to file length)
	if endLine == 0 {
		endLine = -1 // Use -1 to indicate "read to end"
	}

	// Read all lines using bufio.Reader to avoid the 64KB line limit of bufio.Scanner
	reader := bufio.NewReader(file)
	lines := make([]string, 0)
	totalLines := 0
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			// Strip trailing newline and carriage return
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			totalLines++
			if totalLines >= startLine && (endLine < 0 || totalLines <= endLine) {
				lines = append(lines, line)
			}
		}
		if err != nil {
			if err != io.EOF {
				return nil, fmt.Errorf("reading file: %v (file may be binary or have encoding issues)", err)
			}
			break
		}
	}

	// Handle empty file
	if totalLines == 0 {
		return &ReadResult{
			Content:    "",
			TotalLines: 0,
			StartLine:  1,
			EndLine:    0,
			IsEmpty:    true,
		}, nil
	}

	actualEnd := endLine
	if actualEnd == -1 {
		actualEnd = totalLines
	} else if actualEnd > totalLines {
		actualEnd = totalLines
	}

	// Adjust start if it exceeds file length - return error
	if startLine > totalLines {
		return nil, fmt.Errorf("start_line (%d) exceeds total lines (%d) in file", startLine, totalLines)
	}

	var content strings.Builder
	for i, line := range lines {
		if i > 0 {
			content.WriteString("\n")
		}
		fmt.Fprintf(&content, "%4d: %s", startLine+i, line)
	}

	return &ReadResult{
		Content:    content.String(),
		TotalLines: totalLines,
		StartLine:  startLine,
		EndLine:    actualEnd,
		IsEmpty:    false,
	}, nil
}
