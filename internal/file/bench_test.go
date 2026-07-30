// Copyright (c) 2026 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package file

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// BenchmarkGetDirTree_LargeIgnoredTree measures the performance of walking
// a directory tree with a large ignored node_modules/ directory.
//
// With the --directory flag, git ls-files lists ignored directories directly,
// so the walker detects node_modules/ as ignored without recursing into it.
// This benchmark verifies that performance is O(1) with respect to the size
// of the ignored tree (i.e., the number of directories inside node_modules/
// should not affect the time).
func BenchmarkGetDirTree_LargeIgnoredTree(b *testing.B) {
	// Setup: create a git repo with a large node_modules/ tree
	tmpDir := b.TempDir()

	if err := exec.Command("git", "init", tmpDir).Run(); err != nil {
		b.Fatal(err)
	}
	if err := exec.Command("git", "-C", tmpDir, "config", "user.email", "t@t.com").Run(); err != nil {
		b.Fatal(err)
	}
	if err := exec.Command("git", "-C", tmpDir, "config", "user.name", "T").Run(); err != nil {
		b.Fatal(err)
	}

	// .gitignore ignores node_modules/
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("node_modules/\n"), 0644); err != nil {
		b.Fatal(err)
	}

	// Create a large ignored tree: 100 packages, each with a nested structure
	// This simulates a realistic node_modules/ with ~500 directories
	for i := 0; i < 100; i++ {
		pkgDir := filepath.Join(tmpDir, "node_modules", fmt.Sprintf("pkg-%d", i),
			"src", "internal", "util")
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, "index.js"), []byte("content"), 0644); err != nil {
			b.Fatal(err)
		}
	}

	// Create some non-ignored files
	if err := os.MkdirAll(filepath.Join(tmpDir, "src"), 0755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main"), 0644); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("content"), 0644); err != nil {
		b.Fatal(err)
	}

	if err := exec.Command("git", "-C", tmpDir, "add", "-A").Run(); err != nil {
		b.Fatal(err)
	}
	if err := exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run(); err != nil {
		b.Fatal(err)
	}

	// Count total directories created (for reference)
	var dirCount int
	if err := filepath.WalkDir(tmpDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirCount++
		}
		return nil
	}); err != nil {
		b.Fatal(err)
	}
	b.Logf("Total directories on disk: %d (including .git/)", dirCount)

	// Reset timer and run benchmark

	for b.Loop() {
		_, err := getDirTree(tmpDir, 10)
		if err != nil {
			b.Fatal(err)
		}
	}
}
