// Copyright (c) 2026 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package file

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupSearchTestRepo creates a git repo with the following structure:
//
//	.gitignore          → "node_modules/\n"
//	README.md           → "hello world"
//	src/main.go         → "hello world"
//	node_modules/pkg-a/index.js → "hello world"
//	node_modules/pkg-b/index.js → "hello world"
//
// node_modules/ is git-ignored.
func setupSearchTestRepo(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	for _, cmd := range [][]string{
		{"git", "init", tmpDir},
		{"git", "-C", tmpDir, "config", "user.email", "t@t.com"},
		{"git", "-C", tmpDir, "config", "user.name", "T"},
	} {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			t.Fatalf("%s failed: %v", cmd[0], err)
		}
	}

	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("node_modules/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	dirs := []string{"src", "node_modules/pkg-a", "node_modules/pkg-b"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	files := map[string]string{
		"README.md":                   "hello world",
		"src/main.go":                 "hello world",
		"node_modules/pkg-a/index.js": "hello world",
		"node_modules/pkg-b/index.js": "hello world",
	}
	for f, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	for _, cmd := range [][]string{
		{"git", "-C", tmpDir, "add", "-A"},
		{"git", "-C", tmpDir, "commit", "-m", "init"},
	} {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			t.Fatalf("%s failed: %v", cmd[0], err)
		}
	}

	return tmpDir
}

// ---------------------------------------------------------------------------
// HandleSearchContent
// ---------------------------------------------------------------------------

// TestSearchContent_FromRepoRoot_SkipsIgnoredDir verifies that searching from
// the repo root with include_ignored=false still skips node_modules/.
func TestSearchContent_FromRepoRoot_SkipsIgnoredDir(t *testing.T) {
	tmpDir := setupSearchTestRepo(t)

	result, _, err := HandleSearchContent(context.Background(), nil, map[string]any{
		"path":    tmpDir,
		"pattern": "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", resultText(result))
	}

	text := resultText(result)

	// Files in non-ignored directories must be found.
	if !strings.Contains(text, "main.go") {
		t.Errorf("expected to find src/main.go, got:\n%s", text)
	}
	if !strings.Contains(text, "README.md") {
		t.Errorf("expected to find README.md, got:\n%s", text)
	}

	// Files inside node_modules/ must NOT be found.
	if strings.Contains(text, "pkg-a/index.js") {
		t.Errorf("node_modules/pkg-a/index.js should be skipped, got:\n%s", text)
	}
	if strings.Contains(text, "pkg-b/index.js") {
		t.Errorf("node_modules/pkg-b/index.js should be skipped, got:\n%s", text)
	}
}

// TestSearchContent_FromIgnoredDirRoot_SearchesContent verifies that when the
// caller explicitly specifies an ignored directory as the search path, its
// contents are searched (the root is not skipped).
func TestSearchContent_FromIgnoredDirRoot_SearchesContent(t *testing.T) {
	tmpDir := setupSearchTestRepo(t)

	result, _, err := HandleSearchContent(context.Background(), nil, map[string]any{
		"path":    filepath.Join(tmpDir, "node_modules"),
		"pattern": "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", resultText(result))
	}

	text := resultText(result)

	// Files inside node_modules/ MUST be found — the caller explicitly
	// chose node_modules/ as the search root.
	if !strings.Contains(text, "pkg-a/index.js") {
		t.Errorf("expected to find node_modules/pkg-a/index.js when searching from node_modules/, got:\n%s", text)
	}
	if !strings.Contains(text, "pkg-b/index.js") {
		t.Errorf("expected to find node_modules/pkg-b/index.js when searching from node_modules/, got:\n%s", text)
	}
}

// TestSearchContent_FromIgnoredSubDir_SearchesContent verifies that searching
// from a subdirectory inside an ignored directory also works.
func TestSearchContent_FromIgnoredSubDir_SearchesContent(t *testing.T) {
	tmpDir := setupSearchTestRepo(t)

	result, _, err := HandleSearchContent(context.Background(), nil, map[string]any{
		"path":    filepath.Join(tmpDir, "node_modules", "pkg-a"),
		"pattern": "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", resultText(result))
	}

	text := resultText(result)

	if !strings.Contains(text, "index.js") {
		t.Errorf("expected to find index.js when searching from node_modules/pkg-a/, got:\n%s", text)
	}
}

// ---------------------------------------------------------------------------
// HandleSearchName
// ---------------------------------------------------------------------------

// TestSearchName_FromRepoRoot_SkipsIgnoredDir verifies that searching from
// the repo root with include_ignored=false still skips node_modules/.
func TestSearchName_FromRepoRoot_SkipsIgnoredDir(t *testing.T) {
	tmpDir := setupSearchTestRepo(t)

	result, _, err := HandleSearchName(context.Background(), nil, map[string]any{
		"path":    tmpDir,
		"pattern": "index.js",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", resultText(result))
	}

	text := resultText(result)

	// index.js files are only inside node_modules/, which is ignored.
	// They must NOT be found when searching from the repo root.
	// Check for the file path (not just "index.js" which appears in the
	// Pattern header line).
	if strings.Contains(text, "node_modules/pkg-a/index.js") {
		t.Errorf("pkg-a/index.js should not be found (inside ignored node_modules/), got:\n%s", text)
	}
	if strings.Contains(text, "node_modules/pkg-b/index.js") {
		t.Errorf("pkg-b/index.js should not be found (inside ignored node_modules/), got:\n%s", text)
	}
}

// TestSearchName_FromIgnoredDirRoot_SearchesFiles verifies that when the
// caller explicitly specifies an ignored directory as the search path, its
// files are found by name.
func TestSearchName_FromIgnoredDirRoot_SearchesFiles(t *testing.T) {
	tmpDir := setupSearchTestRepo(t)

	result, _, err := HandleSearchName(context.Background(), nil, map[string]any{
		"path":    filepath.Join(tmpDir, "node_modules"),
		"pattern": "index.js",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", resultText(result))
	}

	text := resultText(result)

	// index.js files inside node_modules/ MUST be found.
	if !strings.Contains(text, "pkg-a/index.js") {
		t.Errorf("expected to find pkg-a/index.js when searching from node_modules/, got:\n%s", text)
	}
	if !strings.Contains(text, "pkg-b/index.js") {
		t.Errorf("expected to find pkg-b/index.js when searching from node_modules/, got:\n%s", text)
	}
}
