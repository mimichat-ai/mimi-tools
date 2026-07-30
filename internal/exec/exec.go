// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package exec provides cross-platform command execution for the mimi-tools MCP server.
package exec

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync"
	"time"

	margs "github.com/mimichat-ai/mimi-tools/internal/args"
	"github.com/mimichat-ai/mimi-tools/internal/output"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultWaitSeconds = 60 // seconds
)

// ExecArgs holds the parsed arguments for the exec tool.
// Used to hold parsed argument values extracted via args helpers.
type ExecArgs struct {
	Command     string
	Path        string
	WaitSeconds *int
}

// ExecResult contains the result of a command execution.
type ExecResult struct {
	Stdout      string    `json:"stdout" jsonschema:"the standard output captured up to the return point"`
	Stderr      string    `json:"stderr" jsonschema:"the standard error captured up to the return point"`
	ExitCode    int       `json:"exit_code" jsonschema:"the exit code, or -1 if the process is still running"`
	Running     bool      `json:"running" jsonschema:"whether the process is still running after the wait window"`
	PID         int       `json:"pid,omitempty" jsonschema:"the OS process ID of the launched command"`
	StartTime   time.Time `json:"start_time" jsonschema:"the start time of the process (ISO8601)"`
	WaitSeconds float64   `json:"wait_seconds,omitempty" jsonschema:"the wait window used for this execution"`
}

// ExecError is returned when a command finishes with a non-zero exit code
// within the wait window. It carries the full output so callers can diagnose failures.
type ExecError struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Err      error
}

func (e *ExecError) Error() string {
	msg := fmt.Sprintf("command exited with code %d", e.ExitCode)

	if e.Stdout != "" {
		msg += fmt.Sprintf("\n--- Stdout ---\n%s", e.Stdout)
	}
	if e.Stderr != "" {
		msg += fmt.Sprintf("\n--- Stderr ---\n%s", e.Stderr)
	}

	return msg
}

// HandleExec handles the exec tool call.
// Arguments are accepted as raw any to tolerate LLMs that send numbers
// or booleans as strings. Values are coerced via margs helpers.
func HandleExec(ctx context.Context, _ *mcp.CallToolRequest, args any) (*mcp.CallToolResult, any, error) {
	parsed := ExecArgs{
		Command:     margs.GetString(args, "command"),
		Path:        margs.GetString(args, "path"),
		WaitSeconds: margs.GetInt(args, "wait_seconds"),
	}

	if parsed.Command == "" {
		return output.NewTextResult(output.FormatError("exec", output.ToolError{
			Type:       "Validation Error",
			Reason:     "Missing required argument `command`.",
			Suggestion: "Provide the command string to execute, e.g. `ls -la` or `echo hello`.",
		}), true), nil, nil
	}

	waitSeconds := defaultWaitSeconds
	if parsed.WaitSeconds != nil {
		waitSeconds = *parsed.WaitSeconds
	}

	result, err := execCommand(ctx, parsed.Command, parsed.Path, waitSeconds)
	if err != nil {
		if execErr, ok := err.(*ExecError); ok {
			outResult := &output.ExecOutput{
				Path:     parsed.Path,
				Stdout:   execErr.Stdout,
				Stderr:   execErr.Stderr,
				ExitCode: execErr.ExitCode,
				Running:  false,
			}
			md := output.FormatExec(*outResult)
			res := &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: md}},
				IsError: true,
			}
			res.SetError(execErr.Err)
			return res, nil, nil
		}
		// Generic error
		md := output.FormatError("exec", output.ToolError{
			Type:   "Execution Error",
			Reason: fmt.Sprintf("Failed to execute command: %v", err),
		})
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: md}},
			IsError: true,
		}
		res.SetError(err)
		return res, nil, nil
	}

	// Success path
	outResult := &output.ExecOutput{
		Path:        parsed.Path,
		Stdout:      result.Stdout,
		Stderr:      result.Stderr,
		ExitCode:    result.ExitCode,
		Running:     result.Running,
		PID:         result.PID,
		WaitSeconds: result.WaitSeconds,
	}
	md := output.FormatExec(*outResult)
	return output.NewTextResult(md, false), nil, nil
}

// execCommand executes a command on the current platform.
// Behaviour:
//
// - Start the command immediately (no blocking on shell startup).
// - Spawn goroutines to drain stdout/stderr into bounded buffers (max 100 MB each).
// - Wait up to waitSeconds for cmd.Wait() to signal process exit.
// - If the process exits within the window → return the full result (including
// a non-zero exit code as *ExecError when applicable).
// - If the wait window expires → return immediately with Running:true and all
// output captured so far. The process continues running unmanaged; capture
// goroutines may remain blocked for the process's remaining lifetime.
// - If the context is cancelled (e.g., client disconnected) → kill the process
// and return partial output as *ExecError.
func execCommand(ctx context.Context, command, path string, waitSeconds int) (*ExecResult, error) {
	if waitSeconds <= 0 {
		waitSeconds = defaultWaitSeconds
	}
	timeout := time.Duration(waitSeconds) * time.Second

	// Build the shell command
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}

	if path != "" {
		cmd.Dir = path
	}

	// Capture stdout and stderr independently so both are always available
	// regardless of whether the command succeeds, fails, or times out.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %v", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = stdoutPipe.Close()
		return nil, fmt.Errorf("creating stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdoutPipe.Close()
		_ = stderrPipe.Close()
		return nil, fmt.Errorf("starting command: %v", err)
	}

	startTime := time.Now()
	pid := cmd.Process.Pid

	// Bounded buffers (100 MB each) to prevent unbounded memory growth for
	// long-running processes that generate large amounts of output.
	const maxBuffer = 100 << 20 // 100 MB
	var stdoutBuf, stderrBuf []byte
	var stdoutMu, stderrMu sync.Mutex

	// drain reads from r into buf until EOF or the buffer limit is reached,
	// then closes r to release the pipe file descriptor.
	drain := func(r io.ReadCloser, buf *[]byte, mu *sync.Mutex) {
		defer func() { _ = r.Close() }()
		limited := &io.LimitedReader{R: r, N: maxBuffer + 1}
		data, _ := io.ReadAll(limited)
		mu.Lock()
		defer mu.Unlock()
		*buf = append(*buf, data...)
		if len(*buf) > maxBuffer {
			*buf = (*buf)[:maxBuffer]
		}
	}

	var stdoutWG, stderrWG sync.WaitGroup
	stdoutWG.Go(func() { drain(stdoutPipe, &stdoutBuf, &stdoutMu) })
	stderrWG.Go(func() { drain(stderrPipe, &stderrBuf, &stderrMu) })

	// waitDone is closed by the wait goroutine when cmd.Wait() returns.
	// We use a chan struct{} (not chan error) so that multiple receives
	// don't race with the send.
	waitDone := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = cmd.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		// Process finished within the wait window.
		stdoutWG.Wait()
		stderrWG.Wait()
		stdoutMu.Lock()
		stderrMu.Lock()
		result := &ExecResult{
			Stdout:      string(stdoutBuf),
			Stderr:      string(stderrBuf),
			ExitCode:    extractExitCode(waitErr),
			Running:     false,
			PID:         pid,
			StartTime:   startTime,
			WaitSeconds: float64(waitSeconds),
		}
		stdoutMu.Unlock()
		stderrMu.Unlock()

		if result.ExitCode != 0 {
			return nil, &ExecError{
				ExitCode: result.ExitCode,
				Stdout:   result.Stdout,
				Stderr:   result.Stderr,
				Err:      fmt.Errorf("command exited with code %d", result.ExitCode),
			}
		}
		return result, nil

	case <-time.After(timeout):
		// Wait window expired; process may still be running.
		// Drain goroutines may remain blocked for the process's remaining lifetime.
		// This is unavoidable without explicitly killing the process.
		// Close the wait goroutine: it will block on cmd.Wait() until the
		// process exits (not a leak in the traditional sense; the goroutine
		// holds only a small stack and exits when cmd.Wait() returns).
		//
		// Lock the mutexes to safely read the buffers while drain goroutines
		// may still be writing. We do NOT wait for the drain goroutines because
		// the process is still running and they may block indefinitely.
		stdoutMu.Lock()
		stderrMu.Lock()
		result := &ExecResult{
			Stdout:      string(stdoutBuf),
			Stderr:      string(stderrBuf),
			ExitCode:    -1,
			Running:     true,
			PID:         pid,
			StartTime:   startTime,
			WaitSeconds: float64(waitSeconds),
		}
		stdoutMu.Unlock()
		stderrMu.Unlock()
		return result, nil

	case <-ctx.Done():
		// Context cancelled (e.g., client disconnected or request timed out).
		// Kill the process and return partial output.
		_ = cmd.Process.Kill()
		select {
		case <-waitDone:
			// Process exited; drain goroutines will finish on their own.
			stdoutWG.Wait()
			stderrWG.Wait()
		case <-time.After(5 * time.Second):
			// Kill failed and the process is still running.
			// Drain goroutines may be blocked indefinitely; skip them
			// and read whatever partial output is available.
		}
		stdoutMu.Lock()
		stderrMu.Lock()
		partialStdout := string(stdoutBuf)
		partialStderr := string(stderrBuf)
		stdoutMu.Unlock()
		stderrMu.Unlock()
		return nil, &ExecError{
			ExitCode: -1,
			Stdout:   partialStdout,
			Stderr:   partialStderr,
			Err:      fmt.Errorf("command cancelled: %w", ctx.Err()),
		}
	}
}

// extractExitCode interprets the error from cmd.Wait() into a numeric exit code.
// A nil error means exit 0. An *exec.ExitError carries the actual code.
// Any other error returns -1.
func extractExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
