// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package file provides file writing functionality for the mimi-tools MCP server.
package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	margs "github.com/mimichat-ai/mimi-tools/internal/args"
	"github.com/mimichat-ai/mimi-tools/internal/output"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fileLock wraps a mutex with a reference count for automatic cleanup of unused locks.
type fileLock struct {
	sync.Mutex
	inUse int32
	path  string
}

// fileLocks is a map of file paths to their corresponding locks for file-level locking.
// Unused entries are eagerly cleaned up by releaseMutex when inUse reaches 0,
// preventing memory leaks without needing a background goroutine.
var (
	fileLocks   = make(map[string]*fileLock)
	fileLocksMu sync.Mutex
)

// getMutex returns the lock for a specific file path, creating one if it doesn't exist.
// Increments the in-use counter; callers must call releaseMutex after Unlock.
func getMutex(path string) *fileLock {
	fileLocksMu.Lock()
	fl, ok := fileLocks[path]
	if !ok {
		fl = &fileLock{path: path}
		fileLocks[path] = fl
	}
	atomic.AddInt32(&fl.inUse, 1)
	fileLocksMu.Unlock()
	return fl
}

// releaseMutex unlocks the mutex, decrements the in-use counter, and eagerly
// removes the map entry if no goroutine holds a reference. The Unlock happens
// before the counter decrement so that no other goroutine sees inUse==0 while
// the mutex is still locked, preventing a race where a new getMutex caller
// would receive a different fileLock while this goroutine's Unlock hasn't
// completed.
func releaseMutex(fl *fileLock) {
	fl.Unlock()
	if atomic.AddInt32(&fl.inUse, -1) == 0 {
		fileLocksMu.Lock()
		// Only delete if the entry hasn't been replaced by a newer fileLock
		// for the same path (e.g. another goroutine called getMutex and
		// created a new entry before we acquired fileLocksMu).
		if current, ok := fileLocks[fl.path]; ok && current == fl {
			delete(fileLocks, fl.path)
		}
		fileLocksMu.Unlock()
	}
}

// WriteArgs contains the parsed arguments for the write tool.
type WriteArgs struct {
	Path    string
	Content string
}

// HandleWrite handles the write tool call.
// Arguments are accepted as raw any to tolerate LLMs that send numbers
// or booleans as strings. Values are coerced via args helpers.
func HandleWrite(_ context.Context, _ *mcp.CallToolRequest, args any) (*mcp.CallToolResult, any, error) {
	parsed := WriteArgs{
		Path: margs.GetString(args, "path"),
	}

	if parsed.Path == "" {
		return output.NewTextResult(output.FormatError("write", output.ToolError{
			Type:       "Validation Error",
			Reason:     "Missing required argument `path`.",
			Suggestion: "Provide the absolute file path to write, e.g. `/home/user/project/main.go`.",
		}), true), nil, nil
	}

	contentPtr := margs.GetStringPtr(args, "content")
	if contentPtr == nil {
		return output.NewTextResult(output.FormatError("write", output.ToolError{
			Type:       "Validation Error",
			Reason:     "Missing required argument `content`.",
			Suggestion: "Provide the content to write to the file.",
		}), true), nil, nil
	}
	parsed.Content = *contentPtr

	// Resolve absolute path
	absPath, err := filepath.Abs(parsed.Path)
	if err != nil {
		return output.NewTextResult(output.FormatError("write", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to resolve path: %v", err),
		}), true), nil, nil
	}

	// Check if path is a directory
	info, err := os.Stat(absPath)
	if err == nil && info.IsDir() {
		return output.NewTextResult(output.FormatError("write", output.ToolError{
			Type:   "Validation Error",
			Reason: "Path is a directory.",
			Target: absPath,
		}), true), nil, nil
	}

	// Check if file exists to determine Created status
	created := false
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		created = true
	}

	// Acquire file lock
	fl := getMutex(absPath)
	fl.Lock()
	defer releaseMutex(fl)

	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return output.NewTextResult(output.FormatError("write", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to create directories: %v", err),
		}), true), nil, nil
	}

	// Count lines before writing
	totalLines := output.CountLines(parsed.Content)

	// Atomic write via temp file + rename
	if err := atomicWriteFile(absPath, parsed.Content); err != nil {
		return output.NewTextResult(output.FormatError("write", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to write file: %v", err),
		}), true), nil, nil
	}

	writeOutput := output.WriteOutput{
		Path:       absPath,
		Bytes:      len(parsed.Content),
		TotalLines: totalLines,
		Created:    created,
	}

	md := output.FormatWrite(writeOutput)
	return output.NewTextResult(md, false), nil, nil
}
