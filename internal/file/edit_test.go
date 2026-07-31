package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHandleEdit_ExactMatch(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")

	// Helper to run a single case
	runCase := func(name, initial, oldStr, newStr, expected string, wantErr bool) {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
				t.Fatal(err)
			}
			_, _, err := HandleEdit(context.Background(), nil, map[string]any{
				"path":       path,
				"old_string": oldStr,
				"new_string": newStr,
			})
			if wantErr {
				if err == nil {
					t.Errorf("%s: expected error, got nil", name)
				}
				return
			}
			if err != nil {
				t.Errorf("%s: unexpected error: %v", name, err)
				return
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%s: failed to read file: %v", name, err)
			}
			if string(content) != expected {
				t.Errorf("%s:\nexpected: %q\ngot:      %q", name, expected, string(content))
			}
		})
	}

	// ------------------------------------------------------------
	// 1. Basic substring replacement
	// ------------------------------------------------------------
	runCase(
		"1_BasicSubstring",
		`"status": "success",`,
		`success`,
		`completed`,
		`"status": "completed",`,
		false,
	)

	// ------------------------------------------------------------
	// 2. Multiline match (exact match, no normalization)
	// ------------------------------------------------------------
	initial2 := `{
  "name": "test",
  "type": "module",
  "private": true
}
`
	old2 := ` "module",
  "private": true`
	new2 := `"commonjs",
  "private": true`
	expected2 := `{
  "name": "test",
  "type":"commonjs",
  "private": true
}
`
	runCase("2_MultilineExactMatch", initial2, old2, new2, expected2, false)

	// ------------------------------------------------------------
	// 3. Consecutive blank lines are not compressed
	// ------------------------------------------------------------
	runCase(
		"3_MultipleBlankLines",
		"line1\n\n\nline4\n",
		"line1\n\n\nline4",
		"line1\nline4",
		"line1\nline4\n",
		false,
	)

	t.Log("All test cases defined. Run `go test -v ./internal/file -run TestHandleEdit_ExactMatch` to verify.")
}

func TestHandleEdit_FuzzyMatch(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_fuzzy.txt")

	runCase := func(name, initial, oldStr, newStr, expected string, wantErr bool) {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
				t.Fatal(err)
			}
			result, _, err := HandleEdit(context.Background(), nil, map[string]any{
				"path":       path,
				"old_string": oldStr,
				"new_string": newStr,
			})
			if wantErr {
				if err == nil && (result == nil || !result.IsError) {
					t.Errorf("%s: expected error, got nil", name)
				}
				return
			}
			if err != nil {
				t.Errorf("%s: unexpected error: %v", name, err)
				return
			}
			if result != nil && result.IsError {
				t.Errorf("%s: unexpected error result", name)
				return
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%s: failed to read file: %v", name, err)
			}
			if string(content) != expected {
				t.Errorf("%s:\nexpected: %q\ngot:      %q", name, expected, string(content))
			}
		})
	}

	// Test case 1: Fuzzy match with whitespace difference (similarity > 0.95)
	runCase(
		"Fuzzy_WhitespaceDiff",
		`func main() {
    fmt.Println("Hello")
}`,
		`func main() {
    fmt.Println( "Hello")
}`, // Extra space before "Hello"
		`func main() {
    fmt.Println("Hi")
}`,
		`func main() {
    fmt.Println("Hi")
}`,
		false,
	)

	// Test case 2: Fuzzy match with minor typo (similarity < 0.95, should fail)
	runCase(
		"Fuzzy_MinorTypo",
		`const name = "John";`,
		`const name = "Jhon";`, // Typo: Jhon instead of John
		`const name = "Jane";`,
		`const name = "John";`, // Should not change
		true,                   // Expect error (similarity 0.882 < 0.95)
	)

	// Test case 3: Fuzzy match should fail when similarity <= 0.95
	runCase(
		"Fuzzy_LowSimilarity",
		`const name = "John";`,
		`const name = "Johnny";`, // Very different
		`const name = "Jane";`,
		`const name = "John";`, // Should not change
		true,                   // Expect error
	)

	t.Log("Fuzzy match tests defined. Run `go test -v ./internal/file -run TestHandleEdit_FuzzyMatch` to verify.")
}

func TestHandleEdit_FuzzyAmbiguousMatch(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ambiguous.txt")

	// File contains two non-overlapping blocks that are both similar to old_string.
	// Both blocks have the same indentation level but slightly different content,
	// so each should have similarity > 0.95, but they don't overlap.
	content := "func handlerA() {\n    fmt.Println(\"Hello\")\n}\n\nfunc handlerB() {\n    fmt.Println(\"Helloo\")\n}\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// old_string is slightly different from both blocks (extra "!" after Hello)
	// to trigger fuzzy matching. Both blocks should have similarity > 0.95.
	// Uses actual newlines, not literal \n.
	result, _, err := HandleEdit(context.Background(), nil, map[string]any{
		"path":       path,
		"old_string": "func handlerA() {\n    fmt.Println(\"Hello!\")\n}",
		"new_string": "func handlerA() {\n    fmt.Println(\"Hi!\")\n}",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.IsError {
		t.Fatal("expected error result for ambiguous fuzzy match")
	}

	// Verify the error type is "Ambiguous Match".
	// Check for the exact type string to avoid false positives from
	// the temp directory name which may contain "Ambiguous".
	found := false
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			if strings.Contains(tc.Text, "Ambiguous Match") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected error type 'Ambiguous Match', got: %v", result.Content)
	}
}

// TestFuzzyNormalize_Indentation verifies that fuzzyNormalize preserves
// leading whitespace (indentation) and converts tabs to spaces.
func TestFuzzyNormalize_Indentation(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "tab_converted_to_spaces",
			input: "\tif x {",
			want:  "    ifx{",
		},
		{
			name:  "double_tab_converted_to_8_spaces",
			input: "\t\tif x {",
			want:  "        ifx{",
		},
		{
			name:  "spaces_preserved_unchanged",
			input: "    if x {",
			want:  "    ifx{",
		},
		{
			name:  "tab_and_spaces_same_level_match",
			input: "\t  if x {",
			want:  "      ifx{",
		},
		{
			name:  "internal_whitespace_stripped",
			input: "    fmt.Println( \"Hello\" )",
			want:  "    fmt.println(\"hello\")",
		},
		{
			name:  "multiline_preserves_per_line_indentation",
			input: "if x {\n\t\treturn\n}",
			want:  "ifx{\n        return\n}",
		},
		{
			name:  "empty_string",
			input: "",
			want:  "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := fuzzyNormalize(c.input)
			if got != c.want {
				t.Errorf("fuzzyNormalize(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

// TestHandleEdit_FuzzyTabVsSpace verifies that when the LLM uses spaces
// but the file uses tabs (same indentation level), the fuzzy match
// succeeds and the edit is applied.
func TestHandleEdit_FuzzyTabVsSpace(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tabs.txt")

	// File uses tabs for indentation
	content := "func main() {\n\tfmt.Println(\"Hello\")\n}\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// old_string uses 4 spaces (same indentation level as 1 tab)
	// Exact match will fail (tabs != spaces), but fuzzy match should succeed
	// because fuzzyNormalize converts tabs to 4 spaces.
	// Include trailing newline to match the file's format.
	oldStr := "func main() {\n    fmt.Println(\"Hello\")\n}\n"
	newStr := "func main() {\n    fmt.Println(\"Hi\")\n}\n"

	result, _, err := HandleEdit(context.Background(), nil, map[string]any{
		"path":       path,
		"old_string": oldStr,
		"new_string": newStr,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content)
	}

	// Verify file was modified
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := "func main() {\n    fmt.Println(\"Hi\")\n}\n"
	if string(data) != expected {
		t.Errorf("expected %q, got %q", expected, string(data))
	}
}

// TestHandleEdit_FuzzyWrongIndentation verifies that when the old_string
// has a different indentation level (e.g., 1 tab vs 2 tabs), the fuzzy
// match does NOT auto-apply (similarity < 0.95) and instead returns an
// error with a diff so the LLM can self-correct.
func TestHandleEdit_FuzzyWrongIndentation(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "indent.txt")

	// File uses double-tab indentation (inside a nested block)
	content := "\t\tif isBinaryFile(path) {\n\t\t\tt.Error(\"Expected 513-byte file to not be binary\")\n\t\t}\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// old_string uses single-tab indentation (wrong level)
	// Exact match fails. Fuzzy match should find the block but with
	// similarity < 0.95 (indentation difference is visible after normalization).
	oldStr := "\tif isBinaryFile(path) {\n\t\tt.Error(\"Expected 513-byte file to not be binary\")\n\t}"
	newStr := "\tif isBinaryFile(path) {\n\t\tt.Error(\"Expected 512-byte file to not be binary\")\n\t}"

	result, _, err := HandleEdit(context.Background(), nil, map[string]any{
		"path":       path,
		"old_string": oldStr,
		"new_string": newStr,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should be an error (similarity < 0.95, not auto-applied)
	if !result.IsError {
		t.Fatalf("expected error result (similarity < 0.95), got success: %v", result.Content)
	}

	// Should contain a diff showing the indentation difference
	foundDiff := false
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			if strings.Contains(tc.Text, "```diff") {
				foundDiff = true
			}
		}
	}
	if !foundDiff {
		t.Errorf("expected diff in error output, got: %v", result.Content)
	}

	// File should be unchanged
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("file should be unchanged, expected %q, got %q", content, string(data))
	}
}

// TestHandleEdit_FuzzyNoFalseAmbiguity verifies that two code blocks at
// different indentation levels do NOT cause a false "Ambiguous Match"
// error. With the old fuzzyNormalize (strip all whitespace), both blocks
// would score 100% similarity, causing false ambiguity. With the new
// fuzzyNormalize (preserve indentation), only the closer block should
// score above 0.95, or both should score below 0.95.
func TestHandleEdit_FuzzyNoFalseAmbiguity(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "multi_indent.txt")

	// File has two similar blocks at different indentation levels.
	// Block 1: double-tab indentation (lines 1-3)
	// Block 2: triple-tab indentation (lines 5-7)
	content := "\t\tif isBinaryFile(path) {\n\t\t\tt.Error(\"Expected 513-byte file to not be binary\")\n\t\t}\n\n\t\t\tif isBinaryFile(path) {\n\t\t\t\tt.Error(\"Expected 513-byte file to not be binary\")\n\t\t\t}\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// old_string uses single-tab indentation (different from both blocks).
	// With the old fuzzyNormalize, both blocks would normalize to the same
	// string (whitespace stripped), causing false ambiguity.
	// With the new fuzzyNormalize, the blocks have different normalized
	// forms (different indentation), so they should NOT both score > 0.95.
	oldStr := "\tif isBinaryFile(path) {\n\t\tt.Error(\"Expected 513-byte file to not be binary\")\n\t}"
	newStr := "\tif isBinaryFile(path) {\n\t\tt.Error(\"Expected 512-byte file to not be binary\")\n\t}"

	result, _, err := HandleEdit(context.Background(), nil, map[string]any{
		"path":       path,
		"old_string": oldStr,
		"new_string": newStr,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Check that the error is NOT "Ambiguous Match"
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			if strings.Contains(tc.Text, "Ambiguous Match") {
				t.Errorf("should not be ambiguous (different indentation levels), got: %s", tc.Text)
			}
		}
	}

	// File should be unchanged (similarity < 0.95 for both blocks)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("file should be unchanged, expected %q, got %q", content, string(data))
	}
}

// TestGenerateFragmentDiff_Direct verifies the diff output format directly.
func TestGenerateFragmentDiff_Direct(t *testing.T) {
	cases := []struct {
		name            string
		content         string
		matchStart      int
		matchEnd        int
		newString       string
		wantContains    []string
		wantNotContains []string
	}{
		{name: "single_line_replacement",
			content:    "hello world goodbye world",
			matchStart: 6, matchEnd: 11,
			newString:    "EARTH",
			wantContains: []string{"@@ -1,3 +1,3 @@ Col 7", "-    2     : world", "+         2: EARTH", "    1    1: hello"},
		},
		{name: "multiline_with_context",
			content:    "line1\nline2\nline3\nline4\nline5",
			matchStart: 6, matchEnd: 12,
			newString:    "new_line2\n",
			wantContains: []string{"@@ -1,5 +1,5 @@ Col 1", "-    2     : line2", "+         2: new_line2", "    1    1: line1", "    5    5: line5"},
		},
		{name: "context_limited_to_3_lines",
			content:    "a\nb\nc\nd\ne\nf\ng\nold\nh",
			matchStart: 14, matchEnd: 18,
			newString:       "new\n",
			wantContains:    []string{"@@ -5,5 +5,5 @@ Col 1", "    6    6: f", "    7    7: g", "-    8     : old", "+         8: new", "    9    9: h"},
			wantNotContains: []string{"eee"},
		},
		{name: "every_line_has_a_prefix",
			content:    "aaa\nbbb\nccc\nddd\neee",
			matchStart: 4, matchEnd: 7,
			newString:    "XXX\nYYY",
			wantContains: []string{"@@ -1,5 +1,6 @@ Col 1", "    1    1: aaa", "-    2     : bbb", "+         2: XXX", "+         3: YYY", "    3    4: ccc", "    4    5: ddd", "    5    6: eee"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diff := generateFragmentDiff(c.content, c.matchStart, c.matchEnd, c.newString)

			// Every line after the header must have a valid prefix
			lines := strings.Split(diff, "\n")
			for i, line := range lines {
				if i == 0 {
					if !strings.HasPrefix(line, "@@ ") {
						t.Errorf("Bad header: %q", line)
					}
					continue
				}
				if line == "" {
					continue
				}
				prefix := rune(line[0])
				if prefix != ' ' && prefix != '-' && prefix != '+' {
					t.Errorf("Line %d missing valid prefix: %q", i, line)
				}
			}

			for _, want := range c.wantContains {
				if !strings.Contains(diff, want) {
					t.Errorf("Diff missing %q:\n%s", want, diff)
				}
			}
			for _, want := range c.wantNotContains {
				if strings.Contains(diff, want) {
					t.Errorf("Diff should not contain %q:\n%s", want, diff)
				}
			}
		})
	}
}
