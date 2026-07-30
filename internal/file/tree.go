// Copyright (c) 2025-2026 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package file provides tree listing functionality for the mimi-tools MCP server.
package file

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	margs "github.com/mimichat-ai/mimi-tools/internal/args"
	"github.com/mimichat-ai/mimi-tools/internal/output"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TreeArgs contains the parsed arguments for the tree tool.
type TreeArgs struct {
	Path     string
	MaxDepth *int
}

// TreeResult contains the result of a tree listing operation.
type TreeResult struct {
	Root         string
	MaxDepth     int
	TotalDirs    int
	TotalFiles   int
	Truncated    bool
	SkippedCount int
	Tree         []output.Node
}

// ignoreCache caches ignored paths per git root directory with a TTL.
// The cached map is read-only after creation, so it is safe to share by reference.
// Expired entries are periodically swept by ignoreCacheSweeper to prevent unbounded growth.
var (
	ignoreCache         = make(map[string]*ignoreCacheEntry)
	ignoreMu            sync.RWMutex
	ignoreCacheTTL      = 30 * time.Second
	ignoreCacheOnce     sync.Once
	ignoreCacheSweepInt = 5 * time.Minute
)

// ignoreCacheSweeper periodically removes expired entries from the cache.
func ignoreCacheSweeper() {
	ticker := time.NewTicker(ignoreCacheSweepInt)
	defer ticker.Stop()
	for range ticker.C {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ignoreCacheSweeper] panic recovered: %v", r)
				}
			}()
			ignoreMu.Lock()
			defer ignoreMu.Unlock()
			for root, e := range ignoreCache {
				if time.Since(e.loadedAt) >= ignoreCacheTTL {
					delete(ignoreCache, root)
				}
			}
		}()
	}
}

// ignoreCacheEntry holds the ignored paths map and the time it was loaded.
type ignoreCacheEntry struct {
	paths    map[string]bool
	loadedAt time.Time
}

const defaultTreeMaxDepth = 5

// DirError represents an error accessing a directory.
type DirError struct {
	Kind string // "not_found" | "permission" | "io"
	Path string
	Err  error
}

func (e *DirError) Error() string {
	switch e.Kind {
	case "not_found":
		return fmt.Sprintf("path does not exist: %s", e.Path)
	case "permission":
		return fmt.Sprintf("permission denied: %s", e.Path)
	default:
		return fmt.Sprintf("failed to access %s: %v", e.Path, e.Err)
	}
}

func (e *DirError) IsNotFound() bool {
	return e.Kind == "not_found"
}

func (e *DirError) IsPermission() bool {
	return e.Kind == "permission"
}

// HandleTree handles the tree tool call.
// Arguments are accepted as raw any to tolerate LLMs that send numbers
// or booleans as strings. Values are coerced via args helpers.
func HandleTree(_ context.Context, _ *mcp.CallToolRequest, args any) (*mcp.CallToolResult, any, error) {
	parsed := TreeArgs{
		Path:     margs.GetString(args, "path"),
		MaxDepth: margs.GetInt(args, "max_depth"),
	}

	if parsed.Path == "" {
		return output.NewTextResult(output.FormatError("tree", output.ToolError{
			Type:       "Validation Error",
			Reason:     "Missing required argument `path`.",
			Suggestion: "Provide the absolute directory path to list, e.g. `/home/user/project`.",
		}), true), nil, nil
	}

	// Resolve absolute path. Symlink evaluation is handled by getDirTree,
	// which is also called by read.go's directory fallback, so both call
	// sites benefit from a single canonicalization point.
	absPath, err := filepath.Abs(parsed.Path)
	if err != nil {
		return output.NewTextResult(output.FormatError("tree", output.ToolError{
			Type:   "Internal Error",
			Reason: fmt.Sprintf("Failed to resolve path: %v", err),
		}), true), nil, nil
	}

	var maxDepth int
	if parsed.MaxDepth != nil {
		maxDepth = *parsed.MaxDepth
		// maxDepth = 0  ->  current directory only (no recursion into subdirectories)
		// maxDepth < 0  ->  unlimited (full expansion)
		// maxDepth > 0  ->  limit recursion depth
	} else {
		maxDepth = defaultTreeMaxDepth // default to 5 when not provided
	}

	result, err := getDirTree(absPath, maxDepth)
	if err != nil {
		var dirErr *DirError
		errorType := "Internal Error"
		errorReason := err.Error()

		if errors.As(err, &dirErr) {
			switch dirErr.Kind {
			case "not_found":
				errorType = "Path Not Found"
				errorReason = "The path does not exist."
			case "permission":
				errorType = "Permission Denied"
				errorReason = dirErr.Error()
			default:
				errorType = "IO Error"
				errorReason = dirErr.Error()
			}
		}

		return output.NewTextResult(output.FormatError("tree", output.ToolError{
			Type:   errorType,
			Reason: errorReason,
			Target: absPath,
		}), true), nil, nil
	}

	// Build output directly from result (no conversion needed)
	treeOutput := output.TreeOutput{
		Root:         result.Root,
		MaxDepth:     result.MaxDepth,
		TotalDirs:    result.TotalDirs,
		TotalFiles:   result.TotalFiles,
		SkippedCount: result.SkippedCount,
		Nodes:        result.Tree,
	}

	if result.Truncated {
		treeOutput.Note = "> ⚠️  Depth limit reached — some directories were not expanded. Statistics reflect only the walked portion."
	}

	md := output.FormatTree(treeOutput)
	return output.NewTextResult(md, false), nil, nil
}

// getDirTree generates a tree structure for the given directory path.
func getDirTree(rootPath string, maxDepth int) (*TreeResult, error) {
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolving absolute path: %v", err)
	}
	// Evaluate symlinks to get canonical path. This is critical on macOS where
	// /var/folders is a symlink to /private/var/folders, and git rev-parse
	// --show-toplevel returns the canonical path.
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &DirError{Kind: "not_found", Path: absPath}
		}
		return nil, fmt.Errorf("resolving symlinks: %v", err)
	}
	absPath = resolvedPath

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &DirError{Kind: "not_found", Path: absPath}
		}
		if os.IsPermission(err) {
			return nil, &DirError{Kind: "permission", Path: absPath}
		}
		return nil, &DirError{Kind: "io", Path: absPath, Err: err}
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Get git root and ignored paths
	gitRoot, ignoredPaths := getGitIgnoreInfo(absPath)

	walker := &treeWalker{
		root:         absPath,
		gitRoot:      gitRoot,
		ignoredPaths: ignoredPaths,
		maxDepth:     maxDepth,
		skippedPaths: make(map[string]bool),
	}

	children, err := walker.walkDir(absPath, 0)
	if err != nil {
		return nil, err
	}

	rootNode := output.Node{
		Name:     filepath.Base(absPath),
		Path:     absPath,
		IsDir:    true,
		Depth:    0,
		Children: children,
	}

	dirCount, fileCount := walker.countStats()

	return &TreeResult{
		Root:       absPath,
		MaxDepth:   maxDepth,
		TotalDirs:  dirCount,
		TotalFiles: fileCount,
		Truncated:  walker.truncated,
		Tree:       []output.Node{rootNode},
	}, nil
}

type treeWalker struct {
	root         string
	gitRoot      string
	ignoredPaths map[string]bool
	maxDepth     int
	dirCount     int
	fileCount    int
	truncated    bool
	skippedCount int
	skippedPaths map[string]bool
}

func (t *treeWalker) countStats() (int, int) {
	return t.dirCount, t.fileCount
}

// filteredEntry pairs a directory entry with its ignored status,
// so the node-building loop can skip recursion for ignored directories.
type filteredEntry struct {
	entry   os.DirEntry
	ignored bool
}

func (t *treeWalker) walkDir(path string, depth int) ([]output.Node, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsPermission(err) || os.IsNotExist(err) {
			t.skippedCount++
			t.skippedPaths[path] = true
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory %s: %v", path, err)
	}

	// Filter entries: ignored files are dropped entirely; ignored
	// directories are kept so they appear in the tree with the
	// (ignored) marker, but they are NOT recursed into.
	var filtered []filteredEntry
	for _, entry := range entries {
		relPath, err := filepath.Rel(t.root, filepath.Join(path, entry.Name()))
		if err != nil {
			continue
		}
		fullPath := filepath.Join(path, entry.Name())
		if t.isIgnored(fullPath, relPath) {
			if entry.IsDir() {
				filtered = append(filtered, filteredEntry{entry: entry, ignored: true})
			}
			continue
		}
		filtered = append(filtered, filteredEntry{entry: entry, ignored: false})
	}

	sort.Slice(filtered, func(i, j int) bool {
		return shouldSortBefore(filtered[i].entry, filtered[j].entry)
	})

	var nodes []output.Node
	for i, fe := range filtered {
		isLast := i == len(filtered)-1
		absPath := filepath.Join(path, fe.entry.Name())

		node := output.Node{
			Name:       fe.entry.Name(),
			Path:       absPath,
			IsDir:      fe.entry.IsDir(),
			Depth:      depth,
			ParentPath: path,
			IsLast:     isLast,
		}

		if fe.entry.IsDir() {
			if fe.ignored {
				// Ignored directory: mark and skip recursion entirely.
				// With --directory, git ls-files lists ignored directories
				// directly, so isIgnored returns true without needing to
				// recurse into the directory to discover its contents.
				node.Ignored = true
			} else {
				t.dirCount++
				childDepth := depth + 1
				if t.maxDepth < 0 || childDepth < t.maxDepth {
					children, err := t.walkDir(absPath, childDepth)
					if err != nil {
						return nil, err
					}
					node.Children = children

					if t.skippedPaths[absPath] {
						node.Skipped = true
					}
				} else if t.maxDepth >= 0 && childDepth >= t.maxDepth {
					t.truncated = true
					node.Truncated = true
				}
			}
		} else {
			t.fileCount++
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// isIgnored checks if a path should be ignored based on .gitignore rules.
func (t *treeWalker) isIgnored(fullPath, relPath string) bool {
	if shouldIgnoreBuiltIn(relPath) {
		return true
	}

	if t.gitRoot == "" {
		return false
	}

	relToRepo, err := filepath.Rel(t.gitRoot, fullPath)
	if err != nil {
		return false
	}

	// Check the path itself and all parent directories.
	for check := relToRepo; check != "." && check != "/"; check = filepath.Dir(check) {
		if t.ignoredPaths[check] || t.ignoredPaths["/"+check] {
			return true
		}
	}

	return false
}

// shouldIgnoreBuiltIn checks built-in ignore patterns for non-git directories.
func shouldIgnoreBuiltIn(name string) bool {
	ignoreDirs := []string{".git"}
	parts := strings.SplitSeq(name, string(filepath.Separator))
	for part := range parts {
		if slices.Contains(ignoreDirs, part) {
			return true
		}
	}
	return false
}

// shouldSortBefore returns true if entry a should come before entry b.
func shouldSortBefore(a, b os.DirEntry) bool {
	aIsDir := a.IsDir()
	bIsDir := b.IsDir()

	if aIsDir != bIsDir {
		return aIsDir
	}
	return a.Name() < b.Name()
}

// getGitIgnoreInfo returns the git root and ignored paths for a given directory.
// Results are cached per git root with a 30-second TTL. A copy of the cached
// map is returned to prevent concurrent modification by callers.
// Expired entries are swept by a background goroutine and opportunistically
// on each cache update to prevent unbounded growth.
func getGitIgnoreInfo(path string) (string, map[string]bool) {
	gitRoot, err := findGitRoot(path)
	if err != nil {
		return "", make(map[string]bool)
	}

	ignoreMu.RLock()
	entry, ok := ignoreCache[gitRoot]
	ignoreMu.RUnlock()
	if ok && time.Since(entry.loadedAt) < ignoreCacheTTL {
		// Return a defensive copy to prevent callers from corrupting the cache
		cpy := make(map[string]bool, len(entry.paths))
		maps.Copy(cpy, entry.paths)
		return gitRoot, cpy
	}

	ignoredPaths, err := loadIgnoredPaths(gitRoot)
	if err != nil {
		// If we have a stale cache entry, return a copy instead of an empty map
		if ok {
			cpy := make(map[string]bool, len(entry.paths))
			maps.Copy(cpy, entry.paths)
			return gitRoot, cpy
		}
		return "", make(map[string]bool)
	}

	// Start background sweeper on first cache miss
	ignoreCacheOnce.Do(func() {
		go ignoreCacheSweeper()
	})

	ignoreMu.Lock()
	ignoreCache[gitRoot] = &ignoreCacheEntry{paths: ignoredPaths, loadedAt: time.Now()}
	ignoreMu.Unlock()

	// Return a defensive copy to prevent callers from corrupting the cache
	cpy := make(map[string]bool, len(ignoredPaths))
	maps.Copy(cpy, ignoredPaths)
	return gitRoot, cpy
}

// findGitRoot finds the git repository root using git rev-parse.
func findGitRoot(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository")
	}
	return strings.TrimSpace(string(output)), nil
}

// loadIgnoredPaths loads the list of ignored paths from git.
func loadIgnoredPaths(gitRoot string) (map[string]bool, error) {
	cmd := exec.Command("git", "-C", gitRoot, "ls-files", "--others", "--ignored", "--exclude-standard", "--directory")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("loading git ignore list: %v", err)
	}

	ignored := make(map[string]bool)
	lines := strings.SplitSeq(strings.TrimSpace(string(output)), "\n")
	for line := range lines {
		line = strings.TrimSuffix(line, "/")
		if line != "" {
			// Convert git's forward-slash paths to OS-native separators
			// so that lookups via filepath.Rel (which uses OS separators)
			// work correctly on Windows.
			line = filepath.FromSlash(line)
			ignored[line] = true
		}
	}
	return ignored, nil
}
