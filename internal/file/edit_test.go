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
	// Both blocks differ only by a single character (Hello vs Helloo),
	// so each should have similarity > 0.95, but they don't overlap.
	content := `func handlerA() {
    fmt.Println("Hello")
}

func handlerB() {
    fmt.Println("Helloo")
}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, _, err := HandleEdit(context.Background(), nil, map[string]any{
		"path":       path,
		"old_string": `func handlerA() {\n    fmt.Println("Hello")\n}`,
		"new_string": `func handlerA() {\n    fmt.Println("Hi")\n}`,
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

	// Verify the error message mentions "Ambiguous"
	found := false
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			if strings.Contains(tc.Text, "Ambiguous") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected error to mention 'Ambiguous', got: %v", result.Content)
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
