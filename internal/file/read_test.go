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

func TestHandleRead_BasicFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("full file", func(t *testing.T) {
		result, _, err := HandleRead(context.Background(), nil, map[string]any{
			"path": path,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil || result.IsError {
			t.Fatal("expected success result")
		}
	})

	t.Run("line range 1-2", func(t *testing.T) {
		result, _, err := HandleRead(context.Background(), nil, map[string]any{
			"path":       path,
			"start_line": 1,
			"end_line":   2,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil || result.IsError {
			t.Fatal("expected success result")
		}
	})

	t.Run("string line numbers", func(t *testing.T) {
		result, _, err := HandleRead(context.Background(), nil, map[string]any{
			"path":       path,
			"start_line": "1",
			"end_line":   "2",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil || result.IsError {
			t.Fatal("expected success result")
		}
	})
}

func TestHandleRead_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	result, _, err := HandleRead(context.Background(), nil, map[string]any{
		"path": path,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result for empty file")
	}
}

func TestHandleRead_NonExistentFile(t *testing.T) {
	result, _, err := HandleRead(context.Background(), nil, map[string]any{
		"path": "/nonexistent/path/to/file.txt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result for non-existent file")
	}
}

func TestHandleRead_InvalidLineRange(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, _, err := HandleRead(context.Background(), nil, map[string]any{
		"path":       path,
		"start_line": 10,
		"end_line":   1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result for invalid line range")
	}
}

func TestHandleRead_LineOutOfBounds(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, _, err := HandleRead(context.Background(), nil, map[string]any{
		"path":       path,
		"start_line": 999,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result for line out of bounds")
	}
}

func TestHandleRead_DirectoryFallback(t *testing.T) {
	tmpDir := t.TempDir()

	result, _, err := HandleRead(context.Background(), nil, map[string]any{
		"path": tmpDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result for directory (tree fallback)")
	}
}

func TestHandleRead_MissingPath(t *testing.T) {
	result, _, err := HandleRead(context.Background(), nil, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result for missing path")
	}
}

func TestHandleRead_LongLine(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "longline.txt")
	// Create a file with a single line longer than 64KB (bufio.Scanner default limit)
	longLine := make([]byte, 100000)
	for i := range longLine {
		longLine[i] = 'a'
	}
	longLine = append(longLine, '\n')
	if err := os.WriteFile(path, longLine, 0644); err != nil {
		t.Fatal(err)
	}

	result, _, err := HandleRead(context.Background(), nil, map[string]any{
		"path": path,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result for file with long line (>64KB)")
	}
}
