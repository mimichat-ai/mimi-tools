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

// TestGetDirTree_IgnoredDirNotRecursed verifies that ignored directories
// (like node_modules/) are marked as (ignored) WITHOUT recursing into
// them. With the --directory flag, git ls-files lists ignored directories
// directly, so isIgnored returns true without needing to inspect their
// contents.
func TestGetDirTree_IgnoredDirNotRecursed(t *testing.T) {
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

	// Create a deeply nested structure inside node_modules/ to simulate
	// a real project with many dependencies.
	for i := 0; i < 5; i++ {
		pkgDir := filepath.Join(tmpDir, "node_modules", "pkg-a", "node_modules", "pkg-b",
			"node_modules", "pkg-c")
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, "index.js"), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Also create some non-ignored files
	if err := os.MkdirAll(filepath.Join(tmpDir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main"), 0644); err != nil {
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

	// Check what git lists as ignored (with --directory)
	out, _ := exec.Command("git", "-C", tmpDir, "ls-files", "--others", "--ignored", "--exclude-standard", "--directory").Output()
	t.Logf("git ignored (with --directory, %d bytes):\n%s", len(out), string(out))

	result, err := getDirTree(tmpDir, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// node_modules/ should appear with (ignored) marker
	nmNode := findNodeByName(result.Tree[0].Children, "node_modules")
	if nmNode == nil {
		t.Fatal("node_modules/ not found in tree")
	}
	if !nmNode.Ignored {
		t.Error("node_modules/ should be marked as ignored")
	}
	// Children should be empty — the directory was NOT recursed into.
	if len(nmNode.Children) > 0 {
		t.Errorf("node_modules/ should have no children (not recursed into), got %d", len(nmNode.Children))
	}

	// src/ should NOT be ignored
	srcNode := findNodeByName(result.Tree[0].Children, "src")
	if srcNode == nil {
		t.Fatal("src/ not found in tree")
	}
	if srcNode.Ignored {
		t.Error("src/ should not be marked as ignored")
	}

	// TotalDirs should only include non-ignored directories.
	// src/ is the only non-ignored directory; node_modules/ and .git/ are
	// ignored and should NOT be counted.
	if result.TotalDirs != 1 {
		t.Errorf("TotalDirs = %d, want 1 (only src/; ignored dirs not counted)", result.TotalDirs)
	}

	// TotalFiles should only include non-ignored files.
	// .gitignore, README.md, and src/main.go = 3.
	if result.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3 (.gitignore, README.md, src/main.go)", result.TotalFiles)
	}
}
