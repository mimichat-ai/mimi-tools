// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package args provides helper functions for extracting and coercing
// typed values from arbitrary JSON-decoded arguments (any).
// This is used with MCP tool handlers that accept any input to tolerate
// LLMs that send numeric or boolean values as strings.
package args

import (
	"fmt"
	"strconv"
)

// GetString extracts a string value from args with the given key.
// If the key is missing, returns empty string.
// If the value is not a string, attempts to convert via fmt.Sprint.
func GetString(args any, key string) string {
	m, ok := args.(map[string]any)
	if !ok {
		return ""
	}
	val, exists := m[key]
	if !exists {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprint(val)
}

// GetStringPtr extracts a string pointer from args with the given key.
// Returns nil if the key is missing.
// This allows callers to distinguish between "not provided" and "provided as empty string".
func GetStringPtr(args any, key string) *string {
	m, ok := args.(map[string]any)
	if !ok {
		return nil
	}
	val, exists := m[key]
	if !exists {
		return nil
	}
	if s, ok := val.(string); ok {
		return &s
	}
	s := fmt.Sprint(val)
	return &s
}

// GetInt extracts an integer value from args with the given key.
// Returns nil if the key is missing.
// Accepts: int, float64 (JSON number), string (parsed as int).
// Returns nil (not zero) for missing keys — callers can distinguish
// "not provided" from "provided as zero".
func GetInt(args any, key string) *int {
	m, ok := args.(map[string]any)
	if !ok {
		return nil
	}
	val, exists := m[key]
	if !exists {
		return nil
	}
	switch v := val.(type) {
	case int:
		r := v
		return &r
	case float64:
		r := int(v)
		return &r
	case string:
		if v == "" {
			return nil
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil
		}
		return &n
	}
	return nil
}

// GetBool extracts a boolean value from args with the given key.
// Returns nil if the key is missing.
// Accepts: bool, string (parsed as "true"/"false"/"1"/"0").
// Returns nil for missing keys — callers can apply defaults.
func GetBool(args any, key string) *bool {
	m, ok := args.(map[string]any)
	if !ok {
		return nil
	}
	val, exists := m[key]
	if !exists {
		return nil
	}
	switch v := val.(type) {
	case bool:
		return &v
	case string:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil
		}
		return &b
	case float64:
		b := v != 0
		return &b
	}
	return nil
}

// GetFloat extracts a float64 value from args with the given key.
// Returns nil if the key is missing.
// Accepts: float64 (JSON number), int, string (parsed as float).
func GetFloat(args any, key string) *float64 {
	m, ok := args.(map[string]any)
	if !ok {
		return nil
	}
	val, exists := m[key]
	if !exists {
		return nil
	}
	switch v := val.(type) {
	case float64:
		return &v
	case int:
		r := float64(v)
		return &r
	case string:
		if v == "" {
			return nil
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil
		}
		return &f
	}
	return nil
}
