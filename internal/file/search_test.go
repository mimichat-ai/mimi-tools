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

func TestConvertUnicodeEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard \\uXXXX format",
			input:    `[\u4e00-\u9fff]`,
			expected: `[\x{4e00}-\x{9fff}]`,
		},
		{
			name:     "single \\uXXXX",
			input:    `\u4e00`,
			expected: `\x{4e00}`,
		},
		{
			name:     "brace format \\u{XXXX}",
			input:    `\u{4e00}`,
			expected: `\x{4e00}`,
		},
		{
			name:     "brace format with range",
			input:    `[\u{4e00}-\u{9fff}]`,
			expected: `[\x{4e00}-\x{9fff}]`,
		},
		{
			name:     "emoji \\UXXXXXXXX",
			input:    `\U0001F600`,
			expected: `\x{0001F600}`,
		},
		{
			name:     "emoji brace format",
			input:    `\u{1F600}`,
			expected: `\x{1F600}`,
		},
		{
			name:     "mixed formats",
			input:    `[\u4e00-\u9fff]\w+`,
			expected: `[\x{4e00}-\x{9fff}]\w+`,
		},
		{
			name:     "mixed brace and standard",
			input:    `[\u{4e00}-\u{9fff}]\w+`,
			expected: `[\x{4e00}-\x{9fff}]\w+`,
		},
		{
			name:     "incomplete \\u (no digits)",
			input:    `\u`,
			expected: `\u`,
		},
		{
			name:     "incomplete \\u (too short)",
			input:    `\u123`,
			expected: `\u123`,
		},
		{
			name:     "invalid hex in \\uXXXX",
			input:    `\u12G4`,
			expected: `\u12G4`,
		},
		{
			name:     "unclosed brace",
			input:    `\u{4e00`,
			expected: `\u{4e00`,
		},
		{
			name:     "empty braces",
			input:    `\u{}`,
			expected: `\u{}`,
		},
		{
			name:     "invalid hex in braces",
			input:    `\u{GGGG}`,
			expected: `\u{GGGG}`,
		},
		{
			name:     "already Go syntax \\x{XXXX}",
			input:    `\x{4e00}`,
			expected: `\x{4e00}`,
		},
		{
			name:     "literal backslash before u",
			input:    `\\u4e00`,
			expected: `\\u4e00`,
		},
		{
			name:     "literal backslash before brace u",
			input:    `\\u{4e00}`,
			expected: `\\u{4e00}`,
		},
		{
			name:     "literal backslash + unicode escape",
			input:    `\\\u4e00`,
			expected: `\\\x{4e00}`,
		},
		{
			name:     "literal backslash + brace unicode escape",
			input:    `\\\u{4e00}`,
			expected: `\\\x{4e00}`,
		},
		{
			name:     "two literal backslashes + u4e00",
			input:    `\\\\u4e00`,
			expected: `\\\\u4e00`,
		},
		{
			name:     "single hex digit brace \\u{X}",
			input:    `\u{4}`,
			expected: `\x{4}`,
		},
		{
			name:     "single hex digit brace \\u{a}",
			input:    `\u{a}`,
			expected: `\x{a}`,
		},
		{
			name:     "single hex digit brace \\u{F}",
			input:    `\u{F}`,
			expected: `\x{F}`,
		},
		{
			name:     "two hex digits brace \\u{XX}",
			input:    `\u{4e}`,
			expected: `\x{4e}`,
		},
		{
			name:     "multiple conversions",
			input:    `\u4e00\U0001F600`,
			expected: `\x{4e00}\x{0001F600}`,
		},
		{
			name:     "no unicode escapes",
			input:    `hello world`,
			expected: `hello world`,
		},
		{
			name:     "empty string",
			input:    ``,
			expected: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertUnicodeEscape(tt.input)
			if got != tt.expected {
				t.Errorf("convertUnicodeEscape(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsHex(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"", false},
		{"0", true},
		{"f", true},
		{"F", true},
		{"9", true},
		{"a", true},
		{"A", true},
		{"4e00", true},
		{"1F600", true},
		{"GGGG", false},
		{"123G", false},
		{"hello", false},
		{"4e0", true}, // 3 chars is still valid hex
		{"4e00g", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isHex(tt.input)
			if got != tt.expected {
				t.Errorf("isHex(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCompileContentMatcher_UnicodeRegex(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "chinese.txt")
	content := "你好世界\nHello World\n中文测试\n一\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		pattern       string
		mode          string
		caseSensitive bool
		wantMatches   int
		wantError     bool
	}{
		{
			name:          "standard \\uXXXX range",
			pattern:       `[\u4e00-\u9fff]`,
			mode:          "regex",
			caseSensitive: true,
			wantMatches:   3, // "你好世界", "中文测试", and "一"
		},
		{
			name:          "brace format \\u{XXXX} range",
			pattern:       `[\u{4e00}-\u{9fff}]`,
			mode:          "regex",
			caseSensitive: true,
			wantMatches:   3,
		},
		{
			name:          "single \\uXXXX",
			pattern:       `\u4e00`,
			mode:          "regex",
			caseSensitive: true,
			wantMatches:   1, // "一" (U+4E00)
		},
		{
			name:          "single brace \\u{XXXX}",
			pattern:       `\u{4e00}`,
			mode:          "regex",
			caseSensitive: true,
			wantMatches:   1,
		},
		{
			name:          "emoji \\UXXXXXXXX",
			pattern:       `\U0001F600`,
			mode:          "regex",
			caseSensitive: true,
			wantMatches:   0, // no emoji in content
		},
		{
			name:          "emoji brace format",
			pattern:       `\u{1F600}`,
			mode:          "regex",
			caseSensitive: true,
			wantMatches:   0,
		},
		{
			name:          "invalid unicode escape should error",
			pattern:       `\u{GGGG}`,
			mode:          "regex",
			caseSensitive: true,
			wantError:     true,
		},
		{
			name:          "single hex digit brace \\u{X}",
			pattern:       `\u{4}`,
			mode:          "regex",
			caseSensitive: true,
			wantMatches:   0, // U+0004 (EOT) not in content, but must compile
		},
		{
			name:          "double backslash \\u4e00 not converted",
			pattern:       `\\u4e00`,
			mode:          "regex",
			caseSensitive: true,
			wantMatches:   0, // literal "\u4e00" not in content, but must compile
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, err := compileContentMatcher(tt.pattern, tt.mode, tt.caseSensitive)
			if tt.wantError {
				if err == nil {
					t.Errorf("compileContentMatcher(%q) expected error, got nil", tt.pattern)
				}
				return
			}
			if err != nil {
				t.Fatalf("compileContentMatcher(%q) unexpected error: %v", tt.pattern, err)
			}

			matches, err := searchInFile(path, matcher)
			if err != nil {
				t.Fatalf("searchInFile unexpected error: %v", err)
			}
			if len(matches) != tt.wantMatches {
				t.Errorf("searchInFile got %d matches, want %d", len(matches), tt.wantMatches)
			}
		})
	}
}

func TestCompileNameMatcher_UnicodeRegex(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		mode          string
		caseSensitive bool
		fileName      string
		wantMatch     bool
		wantError     bool
	}{
		{
			name:          "standard \\uXXXX in filename",
			pattern:       `[\u4e00-\u9fff]`,
			mode:          "regex",
			caseSensitive: true,
			fileName:      "测试.txt",
			wantMatch:     true,
		},
		{
			name:          "brace format \\u{XXXX} in filename",
			pattern:       `[\u{4e00}-\u{9fff}]`,
			mode:          "regex",
			caseSensitive: true,
			fileName:      "测试.txt",
			wantMatch:     true,
		},
		{
			name:          "single \\uXXXX in filename",
			pattern:       `\u4e00`,
			mode:          "regex",
			caseSensitive: true,
			fileName:      "一.txt",
			wantMatch:     true,
		},
		{
			name:          "single brace \\u{XXXX} in filename",
			pattern:       `\u{4e00}`,
			mode:          "regex",
			caseSensitive: true,
			fileName:      "一.txt",
			wantMatch:     true,
		},
		{
			name:          "invalid unicode escape should error",
			pattern:       `\u{GGGG}`,
			mode:          "regex",
			caseSensitive: true,
			fileName:      "test.txt",
			wantError:     true,
		},
		{
			name:          "single hex digit brace \\u{X}",
			pattern:       `\u{4}`,
			mode:          "regex",
			caseSensitive: true,
			fileName:      "test.txt",
			wantMatch:     false, // U+0004 (EOT) not in filename, but must compile
		},
		{
			name:          "double backslash \\u4e00 not converted",
			pattern:       `\\u4e00`,
			mode:          "regex",
			caseSensitive: true,
			fileName:      "test.txt",
			wantMatch:     false, // literal "\u4e00" not in filename, but must compile
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, err := compileNameMatcher(tt.pattern, tt.mode, tt.caseSensitive)
			if tt.wantError {
				if err == nil {
					t.Errorf("compileNameMatcher(%q) expected error, got nil", tt.pattern)
				}
				return
			}
			if err != nil {
				t.Fatalf("compileNameMatcher(%q) unexpected error: %v", tt.pattern, err)
			}
			got := matcher(tt.fileName)
			if got != tt.wantMatch {
				t.Errorf("compileNameMatcher(%q) matched %q = %v, want %v",
					tt.pattern, tt.fileName, got, tt.wantMatch)
			}
		})
	}
}
