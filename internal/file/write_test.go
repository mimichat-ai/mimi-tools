// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleWrite_CreateNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "new.txt")

	result, _, err := HandleWrite(context.Background(), nil, map[string]any{
		"path":    path,
		"content": "hello world\n",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result")
	}

	// Verify file was created
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	if string(content) != "hello world\n" {
		t.Errorf("file content = %q, want %q", string(content), "hello world\n")
	}
}

func TestHandleWrite_OverwriteExisting(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(path, []byte("old content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, _, err := HandleWrite(context.Background(), nil, map[string]any{
		"path":    path,
		"content": "new content\n",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "new content\n" {
		t.Errorf("file content = %q, want %q", string(content), "new content\n")
	}
}

func TestHandleWrite_CreateParentDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sub", "dir", "file.txt")

	result, _, err := HandleWrite(context.Background(), nil, map[string]any{
		"path":    path,
		"content": "nested\n",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "nested\n" {
		t.Errorf("file content = %q, want %q", string(content), "nested\n")
	}
}

func TestHandleWrite_DirectoryPath(t *testing.T) {
	tmpDir := t.TempDir()

	result, _, err := HandleWrite(context.Background(), nil, map[string]any{
		"path":    tmpDir,
		"content": "content",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result when writing to a directory")
	}
}

func TestHandleWrite_MissingPath(t *testing.T) {
	result, _, err := HandleWrite(context.Background(), nil, map[string]any{
		"content": "content",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result for missing path")
	}
}

func TestHandleWrite_MissingContent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")

	result, _, err := HandleWrite(context.Background(), nil, map[string]any{
		"path": path,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Missing content (key not provided) should result in a validation error
	if result == nil || !result.IsError {
		t.Fatal("expected error result for missing content")
	}
}

func TestHandleWrite_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.txt")

	result, _, err := HandleWrite(context.Background(), nil, map[string]any{
		"path":    path,
		"content": "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result for empty content")
	}

	// Verify file was created and is empty
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	if len(content) != 0 {
		t.Errorf("file should be empty, got %q", string(content))
	}
}
