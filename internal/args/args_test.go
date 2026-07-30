// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package args

import (
	"testing"
)

func TestGetString(t *testing.T) {
	tests := []struct {
		name string
		args any
		key  string
		want string
	}{
		{"existing string", map[string]any{"key": "value"}, "key", "value"},
		{"missing key", map[string]any{"other": "val"}, "key", ""},
		{"nil args", nil, "key", ""},
		{"non-map args", "not a map", "key", ""},
		{"int value converted", map[string]any{"key": 42}, "key", "42"},
		{"float value converted", map[string]any{"key": 3.14}, "key", "3.14"},
		{"bool value converted", map[string]any{"key": true}, "key", "true"},
		{"empty string value", map[string]any{"key": ""}, "key", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetString(tt.args, tt.key)
			if got != tt.want {
				t.Errorf("GetString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetStringPtr(t *testing.T) {
	t.Run("existing string", func(t *testing.T) {
		got := GetStringPtr(map[string]any{"key": "value"}, "key")
		if got == nil || *got != "value" {
			t.Errorf("GetStringPtr() = %v, want *value", got)
		}
	})
	t.Run("missing key returns nil", func(t *testing.T) {
		got := GetStringPtr(map[string]any{"other": "val"}, "key")
		if got != nil {
			t.Errorf("GetStringPtr() = %v, want nil", got)
		}
	})
	t.Run("nil args returns nil", func(t *testing.T) {
		got := GetStringPtr(nil, "key")
		if got != nil {
			t.Errorf("GetStringPtr() = %v, want nil", got)
		}
	})
	t.Run("int value converted to string", func(t *testing.T) {
		got := GetStringPtr(map[string]any{"key": 42}, "key")
		if got == nil || *got != "42" {
			t.Errorf("GetStringPtr() = %v, want *42", got)
		}
	})
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name string
		args any
		key  string
		want *int
	}{
		{"int value", map[string]any{"key": 42}, "key", intPtr(42)},
		{"float64 value", map[string]any{"key": float64(42)}, "key", intPtr(42)},
		{"string value", map[string]any{"key": "42"}, "key", intPtr(42)},
		{"empty string", map[string]any{"key": ""}, "key", nil},
		{"invalid string", map[string]any{"key": "abc"}, "key", nil},
		{"missing key", map[string]any{}, "key", nil},
		{"nil args", nil, "key", nil},
		{"zero int", map[string]any{"key": 0}, "key", intPtr(0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetInt(tt.args, tt.key)
			if (got == nil) != (tt.want == nil) {
				t.Errorf("GetInt() = %v, want %v", got, tt.want)
				return
			}
			if got != nil && *got != *tt.want {
				t.Errorf("GetInt() = %d, want %d", *got, *tt.want)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name string
		args any
		key  string
		want *bool
	}{
		{"bool true", map[string]any{"key": true}, "key", boolPtr(true)},
		{"bool false", map[string]any{"key": false}, "key", boolPtr(false)},
		{"string true", map[string]any{"key": "true"}, "key", boolPtr(true)},
		{"string false", map[string]any{"key": "false"}, "key", boolPtr(false)},
		{"string 1", map[string]any{"key": "1"}, "key", boolPtr(true)},
		{"string 0", map[string]any{"key": "0"}, "key", boolPtr(false)},
		{"invalid string", map[string]any{"key": "maybe"}, "key", nil},
		{"float64 non-zero", map[string]any{"key": float64(1)}, "key", boolPtr(true)},
		{"float64 zero", map[string]any{"key": float64(0)}, "key", boolPtr(false)},
		{"missing key", map[string]any{}, "key", nil},
		{"nil args", nil, "key", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetBool(tt.args, tt.key)
			if (got == nil) != (tt.want == nil) {
				t.Errorf("GetBool() = %v, want %v", got, tt.want)
				return
			}
			if got != nil && *got != *tt.want {
				t.Errorf("GetBool() = %v, want %v", *got, *tt.want)
			}
		})
	}
}

func TestGetFloat(t *testing.T) {
	tests := []struct {
		name string
		args any
		key  string
		want *float64
	}{
		{"float64 value", map[string]any{"key": float64(3.14)}, "key", floatPtr(3.14)},
		{"int value", map[string]any{"key": 42}, "key", floatPtr(42)},
		{"string value", map[string]any{"key": "3.14"}, "key", floatPtr(3.14)},
		{"empty string", map[string]any{"key": ""}, "key", nil},
		{"invalid string", map[string]any{"key": "abc"}, "key", nil},
		{"missing key", map[string]any{}, "key", nil},
		{"nil args", nil, "key", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetFloat(tt.args, tt.key)
			if (got == nil) != (tt.want == nil) {
				t.Errorf("GetFloat() = %v, want %v", got, tt.want)
				return
			}
			if got != nil && *got != *tt.want {
				t.Errorf("GetFloat() = %f, want %f", *got, *tt.want)
			}
		})
	}
}

// Helpers
func intPtr(v int) *int           { return &v }
func boolPtr(v bool) *bool        { return &v }
func floatPtr(v float64) *float64 { return &v }
