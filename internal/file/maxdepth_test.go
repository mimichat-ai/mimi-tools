// Copyright (c) 2026 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package file

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGetDirTree_IgnoredAtMaxDepth verifies that an ignored directory at
// the maxDepth boundary is correctly marked as (ignored) rather than
// truncated (...).
//
// In the previous approach (without --directory), ignored directories at
// maxDepth were shown as "..." because the propagation logic required
// recursion, which was skipped at maxDepth. With --directory, isIgnored
// detects the directory directly, so the ignored check happens before the
// maxDepth check.
func TestGetDirTree_IgnoredAtMaxDepth(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	if err := exec.Command("git", "init", tmpDir).Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "config", "user.email", "t@t.com").Run(); err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "config", "user.name", "T").Run(); err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}

	// .gitignore ignores node_modules/
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("node_modules/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create node_modules/ with content and src/ with content.
	if err := os.MkdirAll(filepath.Join(tmpDir, "node_modules", "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "node_modules", "pkg", "index.js"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "-C", tmpDir, "add", "-A").Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// maxDepth=1: only root's immediate children are shown, no recursion.
	result, err := getDirTree(tmpDir, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	children := result.Tree[0].Children

	// node_modules/ should be marked as Ignored, NOT Truncated.
	nmNode := findNodeByName(children, "node_modules")
	if nmNode == nil {
		t.Fatal("node_modules/ not found in tree")
	}
	if !nmNode.Ignored {
		t.Error("node_modules/ should be marked as Ignored even at maxDepth=1")
	}
	if nmNode.Truncated {
		t.Error("node_modules/ should NOT be Truncated — it is ignored, so maxDepth is irrelevant")
	}

	// src/ should be Truncated (not ignored, at maxDepth boundary).
	srcNode := findNodeByName(children, "src")
	if srcNode == nil {
		t.Fatal("src/ not found in tree")
	}
	if srcNode.Ignored {
		t.Error("src/ should NOT be marked as Ignored")
	}
	if !srcNode.Truncated {
		t.Error("src/ should be marked as Truncated at maxDepth=1")
	}

	// TotalDirs should NOT include ignored directories.
	// node_modules/ is ignored (not counted). src/ is truncated at maxDepth=1
	// but still counted (it's a real directory, just not expanded).
	// .git/ is ignored (not counted).
	if result.TotalDirs != 1 {
		t.Errorf("TotalDirs = %d, want 1 (only src/; node_modules/ and .git/ are ignored)", result.TotalDirs)
	}
}
