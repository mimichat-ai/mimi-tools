// Copyright (c) 2025-2026 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package file

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mimichat-ai/mimi-tools/internal/output"
)

func TestHandleTree_DefaultDepth(t *testing.T) {
	tmpDir := t.TempDir()
	// Create some structure
	if err := os.MkdirAll(filepath.Join(tmpDir, "sub", "deep"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "sub", "file2.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	result, _, err := HandleTree(context.Background(), nil, map[string]any{
		"path": tmpDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result")
	}
}

func TestHandleTree_CustomDepth(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "a", "b", "c"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Run("depth 1", func(t *testing.T) {
		result, _, err := HandleTree(context.Background(), nil, map[string]any{
			"path":      tmpDir,
			"max_depth": 1,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil || result.IsError {
			t.Fatal("expected success result")
		}
	})

	t.Run("depth string", func(t *testing.T) {
		result, _, err := HandleTree(context.Background(), nil, map[string]any{
			"path":      tmpDir,
			"max_depth": "2",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil || result.IsError {
			t.Fatal("expected success result")
		}
	})
}

func TestHandleTree_NonExistentPath(t *testing.T) {
	result, _, err := HandleTree(context.Background(), nil, map[string]any{
		"path": "/nonexistent/path/to/dir",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result for non-existent path")
	}

	// Verify the error type is "Path Not Found" (not "Internal Error")
	text := resultText(result)
	if !containsErrorType(text, "Path Not Found") {
		t.Errorf("expected error type 'Path Not Found', got:\n%s", text)
	}
	if !containsErrorReason(text, "The path does not exist.") {
		t.Errorf("expected reason 'The path does not exist.', got:\n%s", text)
	}
}

func TestHandleTree_MissingPath(t *testing.T) {
	result, _, err := HandleTree(context.Background(), nil, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result for missing path")
	}
}

func TestHandleTree_FileNotDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, _, err := HandleTree(context.Background(), nil, map[string]any{
		"path": path,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result when path is a file, not a directory")
	}
}

// findNodeByName searches nodes recursively for a node with the given name.
func findNodeByName(nodes []output.Node, name string) *output.Node {
	for i := range nodes {
		if nodes[i].Name == name {
			return &nodes[i]
		}
		if found := findNodeByName(nodes[i].Children, name); found != nil {
			return found
		}
	}
	return nil
}

func TestGetDirTree_AncestorMarkingBug(t *testing.T) {
	// Skip if git is not available.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	// Initialise a git repository.
	if err := exec.Command("git", "init", tmpDir).Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run(); err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run(); err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}

	// Create .gitignore that ignores build/output/ but NOT build/input/.
	// This is a nested path — only one subdirectory under build/ is ignored.
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("build/output/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create directory structure:
	//   build/
	//     output/    ← ignored by .gitignore
	//       file.o
	//     input/     ← NOT ignored (sibling of output/)
	//       main.c
	dirs := []string{"build/output", "build/input"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	files := []string{"build/output/file.o", "build/input/main.c", "README.md"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Stage and commit so git tracks the non-ignored files.
	if err := exec.Command("git", "-C", tmpDir, "add", "-A").Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	result, err := getDirTree(tmpDir, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	children := result.Tree[0].Children

	// build/ itself should appear in the tree (it's not ignored as a whole).
	buildNode := findNodeByName(children, "build")
	if buildNode == nil {
		t.Fatal("build/ not found in tree")
	}
	if buildNode.Ignored {
		t.Error("build/ should NOT be marked as ignored — only build/output/ is ignored, not the entire build/ directory")
	}

	// build/output/ should be ignored.
	outputNode := findNodeByName(buildNode.Children, "output")
	if outputNode == nil {
		t.Fatal("build/output/ not found in tree")
	}
	if !outputNode.Ignored {
		t.Error("build/output/ should be marked as ignored")
	}
	if len(outputNode.Children) > 0 {
		t.Error("build/output/ should not have children (ignored, should not be recursed into)")
	}

	// build/input/ should NOT be ignored — it's a sibling of build/output/,
	// and .gitignore only ignores build/output/, not build/input/.
	inputNode := findNodeByName(buildNode.Children, "input")
	if inputNode == nil {
		t.Fatal("build/input/ not found in tree")
	}
	if inputNode.Ignored {
		t.Error("build/input/ should NOT be marked as ignored — it is a sibling of build/output/, not under it")
	}
	if len(inputNode.Children) == 0 {
		t.Error("build/input/ should have children (not ignored, should be recursed into)")
	}

	// TotalDirs should count build/ and build/input/ (2), but NOT build/output/.
	// build/output/ is ignored, so it should not be counted.
	if result.TotalDirs != 2 {
		t.Errorf("TotalDirs = %d, want 2 (build/, build/input/; build/output/ is ignored)", result.TotalDirs)
	}

	// TotalFiles should include .gitignore, README.md, and build/input/main.c (3).
	// build/output/file.o is inside an ignored directory and should not be counted.
	if result.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3 (.gitignore, README.md, build/input/main.c)", result.TotalFiles)
	}
}

func TestGetDirTree_IgnoredDirectories(t *testing.T) {
	// Skip if git is not available.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	// Initialise a git repository so that .gitignore rules are honoured.
	if err := exec.Command("git", "init", tmpDir).Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run(); err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run(); err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}

	// Create .gitignore that ignores node_modules/ and vendor/.
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("node_modules/\nvendor/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create directory structure.
	dirs := []string{"src", "node_modules/pkg", "vendor"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create files.
	files := []string{"src/main.go", "node_modules/pkg/index.js", "vendor/lib.js", "README.md"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Stage and commit so git tracks the non-ignored files.
	if err := exec.Command("git", "-C", tmpDir, "add", "-A").Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	result, err := getDirTree(tmpDir, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	children := result.Tree[0].Children

	// src/ should NOT be ignored and should have children (recursed into).
	srcNode := findNodeByName(children, "src")
	if srcNode == nil {
		t.Fatal("src/ not found in tree")
	}
	if srcNode.Ignored {
		t.Error("src/ should not be marked as ignored")
	}
	if len(srcNode.Children) == 0 {
		t.Error("src/ should have children (not ignored, should be recursed into)")
	}

	// node_modules/ should be ignored and should NOT have children.
	nodeModulesNode := findNodeByName(children, "node_modules")
	if nodeModulesNode == nil {
		t.Fatal("node_modules/ not found in tree")
	}
	if !nodeModulesNode.Ignored {
		t.Error("node_modules/ should be marked as ignored")
	}
	if len(nodeModulesNode.Children) > 0 {
		t.Error("node_modules/ should not have children (ignored, should not be recursed into)")
	}

	// vendor/ should be ignored and should NOT have children.
	vendorNode := findNodeByName(children, "vendor")
	if vendorNode == nil {
		t.Fatal("vendor/ not found in tree")
	}
	if !vendorNode.Ignored {
		t.Error("vendor/ should be marked as ignored")
	}
	if len(vendorNode.Children) > 0 {
		t.Error("vendor/ should not have children (ignored, should not be recursed into)")
	}

	// .git/ should be ignored (built-in) and should NOT have children.
	gitNode := findNodeByName(children, ".git")
	if gitNode == nil {
		t.Fatal(".git/ not found in tree")
	}
	if !gitNode.Ignored {
		t.Error(".git/ should be marked as ignored (built-in)")
	}
	if len(gitNode.Children) > 0 {
		t.Error(".git/ should not have children (ignored, should not be recursed into)")
	}

	// TotalDirs should NOT include ignored directories.
	// Only src/ is a traversed directory; node_modules/, vendor/, .git/ are ignored.
	if result.TotalDirs != 1 {
		t.Errorf("TotalDirs = %d, want 1 (ignored directories should not be counted)", result.TotalDirs)
	}

	// TotalFiles should only include files in non-ignored directories.
	// .gitignore and README.md are at the root; src/main.go is in src/.
	// Files inside node_modules/ and vendor/ are not traversed.
	if result.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3 (.gitignore, README.md, src/main.go)", result.TotalFiles)
	}
}
