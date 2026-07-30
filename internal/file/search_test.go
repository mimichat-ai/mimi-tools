package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestIsBinaryFile(t *testing.T) {
	tmpDir := t.TempDir()

	runCase := func(name string, content []byte, wantBinary bool) {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(tmpDir, name)
			if err := os.WriteFile(path, content, 0644); err != nil {
				t.Fatal(err)
			}
			got := isBinaryFile(path)
			if got != wantBinary {
				t.Errorf("isBinaryFile(%q) = %v, want %v", name, got, wantBinary)
			}
		})
	}

	// Test 1: Empty file is not binary
	runCase("empty_file", []byte{}, false)

	// Test 2: Plain ASCII text is not binary
	runCase("plain_ascii", []byte("hello world"), false)

	// Test 3: File with NUL byte is binary
	runCase("with_nul_byte", []byte("hello\x00world"), true)

	// Test 4: File with valid UTF-8 multi-byte characters is not binary
	runCase("valid_utf8_chinese", []byte("你好世界"), false)
	runCase("valid_utf8_emoji", []byte("Hello 🌍 World"), false)

	// Test 5: File with invalid UTF-8 sequence is binary
	runCase("invalid_utf8", []byte{0xFF, 0xFE, 0xFD}, true)

	// Test 6: File with high ratio of control characters is binary
	controlHeavy := make([]byte, 100)
	for i := range controlHeavy {
		if i%3 == 0 {
			controlHeavy[i] = '\x01' // control char
		} else {
			controlHeavy[i] = 'a'
		}
	}
	runCase("high_control_ratio", controlHeavy, true)

	// Test 7: File with low ratio of control characters is not binary
	controlLight := make([]byte, 100)
	for i := range controlLight {
		if i%10 == 0 {
			controlLight[i] = '\x01' // 10% control chars
		} else {
			controlLight[i] = 'a'
		}
	}
	runCase("low_control_ratio", controlLight, false)

	// Test 8: TSX file content (like AdminFormDialog.tsx) should not be binary
	tsxContent := []byte(`import { Listbox } from "@headlessui/react";
export default function Component() {
  return <Listbox>Test</Listbox>;
}
`)
	runCase("tsx_file", tsxContent, false)

	// Test 9: File where 512-byte boundary splits a UTF-8 character
	// Create content that's exactly 510 bytes + a 3-byte UTF-8 character
	// This tests the buffer boundary trimming logic
	prefix := make([]byte, 510)
	for i := range prefix {
		prefix[i] = 'a'
	}
	// Append a 3-byte UTF-8 character (e.g., some emoji or CJK character)
	// The character "中" is 3 bytes in UTF-8: 0xE4 0xB8 0xAD
	splitUtf8 := append(prefix, []byte{0xE4, 0xB8, 0xAD}...)
	runCase("utf8_split_at_boundary", splitUtf8, false)

	// Test 10: File with 4-byte UTF-8 character at boundary
	// The emoji "🌍" is 4 bytes: 0xF0 0x9F 0x8C 0x8D
	prefix4 := make([]byte, 509)
	for i := range prefix4 {
		prefix4[i] = 'b'
	}
	splitUtf84 := append(prefix4, []byte{0xF0, 0x9F, 0x8C, 0x8D}...)
	runCase("utf8_4byte_at_boundary", splitUtf84, false)

	t.Log("All isBinaryFile test cases defined. Run `go test -v ./internal/file -run TestIsBinaryFile` to verify.")
}

// TestHandleSearchName_NonExistentPath verifies that searching a non-existent
// path returns a "Path Not Found" error (not "Internal Error").
func TestHandleSearchName_NonExistentPath(t *testing.T) {
	result, _, err := HandleSearchName(context.Background(), nil, map[string]any{
		"path":    "/nonexistent/path/to/dir",
		"pattern": "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result for non-existent path")
	}

	text := resultText(result)
	if !containsErrorType(text, "Path Not Found") {
		t.Errorf("expected error type 'Path Not Found', got:\n%s", text)
	}
	if !containsErrorReason(text, "The path does not exist.") {
		t.Errorf("expected reason 'The path does not exist.', got:\n%s", text)
	}
}

// TestHandleSearchContent_NonExistentPath verifies that searching content in a
// non-existent path returns a "Path Not Found" error (not "Internal Error").
func TestHandleSearchContent_NonExistentPath(t *testing.T) {
	result, _, err := HandleSearchContent(context.Background(), nil, map[string]any{
		"path":    "/nonexistent/path/to/dir",
		"pattern": "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result for non-existent path")
	}

	text := resultText(result)
	if !containsErrorType(text, "Path Not Found") {
		t.Errorf("expected error type 'Path Not Found', got:\n%s", text)
	}
	if !containsErrorReason(text, "The path does not exist.") {
		t.Errorf("expected reason 'The path does not exist.', got:\n%s", text)
	}
}

func TestIsBinaryFile_BoundaryConditions(t *testing.T) {
	tmpDir := t.TempDir()

	// Test that we correctly handle files smaller than 512 bytes
	t.Run("small_file_exact_100_bytes", func(t *testing.T) {
		content := make([]byte, 100)
		for i := range content {
			content[i] = byte('a' + (i % 26))
		}
		path := filepath.Join(tmpDir, "small_100.txt")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
		if isBinaryFile(path) {
			t.Error("Expected small file to not be binary")
		}
	})

	// Test file exactly at 512 bytes with valid UTF-8
	t.Run("exact_512_bytes_valid_utf8", func(t *testing.T) {
		content := make([]byte, 512)
		for i := range content {
			content[i] = byte('a' + (i % 26))
		}
		path := filepath.Join(tmpDir, "exact_512.txt")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
		if isBinaryFile(path) {
			t.Error("Expected 512-byte file to not be binary")
		}
	})

	// Test file slightly over 512 bytes
	t.Run("513_bytes_valid_utf8", func(t *testing.T) {
		content := make([]byte, 513)
		for i := range content {
			content[i] = byte('a' + (i % 26))
		}
		path := filepath.Join(tmpDir, "513_bytes.txt")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
		if isBinaryFile(path) {
			t.Error("Expected 513-byte file to not be binary")
		}
	})
}
