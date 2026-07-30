// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package exec

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestHandleExec_Echo(t *testing.T) {
	result, _, err := HandleExec(context.Background(), nil, map[string]any{
		"command": "echo hello",
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
}

func TestHandleExec_NonZeroExit(t *testing.T) {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "exit /b 1"
	} else {
		cmd = "exit 1"
	}

	result, _, err := HandleExec(context.Background(), nil, map[string]any{
		"command": cmd,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.IsError {
		t.Fatal("expected error result for non-zero exit code")
	}
}

func TestHandleExec_MissingCommand(t *testing.T) {
	result, _, err := HandleExec(context.Background(), nil, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result for missing command")
	}
}

func TestHandleExec_StringWaitSeconds(t *testing.T) {
	result, _, err := HandleExec(context.Background(), nil, map[string]any{
		"command":      "echo hello",
		"wait_seconds": "5",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result with string wait_seconds")
	}
}

func TestHandleExec_WorkingDirectory(t *testing.T) {
	var cmd string
	var dir string
	if runtime.GOOS == "windows" {
		cmd = "echo %cd%"
		dir = `C:\`
	} else {
		cmd = "pwd"
		dir = "/tmp"
	}

	result, _, err := HandleExec(context.Background(), nil, map[string]any{
		"command": cmd,
		"path":    dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result")
	}
}

func TestExecCommand_Timeout(t *testing.T) {
	// Use a command that sleeps longer than the wait window
	result, err := execCommand(context.Background(), "sleep 10", "", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Running {
		t.Fatal("expected Running=true for timed-out command")
	}
	if result.ExitCode != -1 {
		t.Errorf("expected ExitCode=-1, got %d", result.ExitCode)
	}
}

func TestExecCommand_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := execCommand(ctx, "sleep 30", "", 60)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
