// Copyright (c) 2026 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package file

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mimichat-ai/mimi-tools/internal/output"
)

// TestGetDirTree_EmptyDirectoryNotIgnored verifies that an empty directory
// (one with no entries at all) that is NOT in .gitignore is NOT marked as
// ignored.
func TestGetDirTree_EmptyDirectoryNotIgnored(t *testing.T) {
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

	// Create an empty directory (no files inside) and a regular file.
	if err := os.MkdirAll(filepath.Join(tmpDir, "empty_dir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
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

	// Find empty_dir in the tree.
	var emptyDirNode *output.Node
	for i := range children {
		if children[i].Name == "empty_dir" {
			emptyDirNode = &children[i]
			break
		}
	}
	if emptyDirNode == nil {
		t.Fatal("empty_dir/ not found in tree")
	}

	// BUG: empty_dir is just an empty directory, not git-ignored.
	// It must NOT be marked as ignored.
	if emptyDirNode.Ignored {
		t.Error("BUG: empty_dir/ should NOT be marked as ignored — it is an empty directory, not git-ignored")
	}

	// An empty directory should still appear in the tree (not hidden).
	if emptyDirNode.IsDir != true {
		t.Error("empty_dir/ should be marked as a directory")
	}
}

// TestGetDirTree_EmptyIgnoredDir verifies that an empty directory that IS
// in .gitignore is correctly marked as (ignored).
//
// With the --directory flag, git ls-files lists empty ignored directories
// directly (as "empty_ignored/"), so isIgnored returns true without needing
// to recurse into the directory. This fixes a bug in the previous approach
// where empty ignored directories were missed because git ls-files (without
// --directory) only lists files, and an empty directory has no files.
func TestGetDirTree_EmptyIgnoredDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	if err := exec.Command("git", "init", tmpDir).Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run(); err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run(); err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}

	// .gitignore ignores empty_ignored/
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("empty_ignored/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an empty ignored directory and a regular file.
	if err := os.MkdirAll(filepath.Join(tmpDir, "empty_ignored"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

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

	ignoredNode := findNodeByName(children, "empty_ignored")
	if ignoredNode == nil {
		t.Fatal("empty_ignored/ not found in tree")
	}

	// The empty ignored directory MUST be marked as ignored.
	if !ignoredNode.Ignored {
		t.Error("empty_ignored/ should be marked as ignored — it is in .gitignore")
	}

	// It should not have children (not recursed into).
	if len(ignoredNode.Children) > 0 {
		t.Errorf("empty_ignored/ should have no children, got %d", len(ignoredNode.Children))
	}

	// It should not be counted in TotalDirs.
	if result.TotalDirs != 0 {
		t.Errorf("TotalDirs = %d, want 0 (empty_ignored/ is ignored and not counted)", result.TotalDirs)
	}
}
