// Copyright (c) 2025 mimi-tools authors
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// The mimi-tools MCP server provides cross-platform command execution.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	mexec "github.com/mimichat-ai/mimi-tools/internal/exec"
	mfile "github.com/mimichat-ai/mimi-tools/internal/file"

	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Build-time variables, overridden by -ldflags "-X main.version=... -X main.commit=... -X main.buildTime=..."
var (
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

func main() {
	// Parse command-line flags
	showVersion := flag.Bool("version", false, "Print version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("mimi-tools %s (commit: %s, built: %s)\n", version, commit, buildTime)
		return
	}

	// Get transport type from environment variable or use default
	transport := os.Getenv("MIMI_TRANSPORT")
	if transport != "http" && transport != "sse" && transport != "stdio" {
		transport = "http"
	}

	// Get listen address from environment variable or use default
	addr := os.Getenv("MIMI_ADDR")
	if addr == "" {
		addr = "127.0.0.1:2333"
	}

	// Create the MCP server
	server := mcp.NewServer(&mcp.Implementation{Name: "mimi-tools"}, nil)

	// execArgsSchema accepts any value for wait_seconds to tolerate
	// LLMs that send numbers as strings. Primitive fields that should
	// remain strings (command, path) keep their strict type.
	var execArgsSchema json.RawMessage = []byte(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "the command string to execute"},
			"path": {"type": "string", "description": "working directory path for the command execution"},
			"wait_seconds": {"description": "seconds to wait for the command to finish (defaults to 60 when omitted). If the command finishes within the wait window, returns full output and exit code. If the wait window expires, returns partial output captured so far and the process continues running in the background."}
		}
	}`)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "exec",
		Description: "Execute a command on the system using shell (sh -c on Unix/Linux/macOS, cmd /c on Windows)",
		InputSchema: execArgsSchema,
	}, mexec.HandleExec)

	// readArgsSchema: start_line/end_line have no type so string numbers pass;
	// path stays strict string.
	var readArgsSchema json.RawMessage = []byte(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "the absolute file path to read"},
			"start_line": {"description": "starting line number (1-based, defaults to 1 when omitted)"},
			"end_line": {"description": "ending line number (1-based, defaults to end of file when omitted)"}
		}
	}`)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read",
		Description: "Read the content of a file at the given path. Supports optional line range via start_line and end_line.",
		InputSchema: readArgsSchema,
	}, mfile.HandleRead)

	// write args are all strings — no change needed but schema is explicit.
	var writeArgsSchema json.RawMessage = []byte(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "the absolute file path to write"},
			"content": {"type": "string", "description": "the content to write to the file (overwrites the entire file)"}
		}
	}`)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "write",
		Description: "Write content to a file at the given path, creating parent directories if needed",
		InputSchema: writeArgsSchema,
	}, mfile.HandleWrite)

	// editArgs are all strings — no numeric/bool fields.
	var editArgsSchema json.RawMessage = []byte(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "the absolute file path to edit"},
			"old_string": {"type": "string", "description": "the exact string to match and replace. Include surrounding lines of context to ensure the match is unique."},
			"new_string": {"type": "string", "description": "the string to replace the old_string with. Must be different from old_string (use an empty string to delete the matched text)."}
		}
	}`)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "edit",
		Description: "Edit a file at the given path by replacing a matched string. First tries exact substring matching; if no exact match is found, attempts fuzzy matching (ignoring whitespace differences). If old_string matches multiple locations, an error is returned with a suggestion — include surrounding lines of context in old_string to make the match unique.",
		InputSchema: editArgsSchema,
	}, mfile.HandleEdit)

	// treeArgsSchema: max_depth has no type so string numbers pass; path is strict.
	var treeArgsSchema json.RawMessage = []byte(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "the absolute directory path to list"},
			"max_depth": {"description": "maximum depth to traverse (defaults to 5 when omitted)"}
		}
	}`)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tree",
		Description: "Show the directory tree structure for the given path. Respects .gitignore by default; set include_ignored=true to show ignored files.",
		InputSchema: treeArgsSchema,
	}, mfile.HandleTree)

	// searchArgsSchema: bool flags have no type so "true"/"false" pass;
	// path and pattern stay strict strings.
	var searchNameArgsSchema json.RawMessage = []byte(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "the absolute path to search for filenames (file or directory)"},
			"pattern": {"type": "string", "description": "the pattern to search for in filenames. When mode is \"regex\", treated as a regular expression. When mode is \"glob\", treated as a glob pattern. When mode is \"substring\", treated as a literal string."},
			"mode": {"type": "string", "description": "Pattern matching mode. \"glob\" (default): pattern is a glob pattern (e.g., *.go). \"regex\": pattern is a regular expression. \"substring\": pattern is a literal string; matches if the filename contains the pattern as a substring.", "enum": ["glob", "regex", "substring"]},
			"case_sensitive": {"description": "case sensitive search (default: true)"},
			"include_ignored": {"description": "include files/directories ignored by .gitignore (default: false)"}
		}
	}`)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_name",
		Description: "Search for files by name pattern starting from the given path. Supports glob, string and regex matching modes.",
		InputSchema: searchNameArgsSchema,
	}, mfile.HandleSearchName)

	var searchContentArgsSchema json.RawMessage = []byte(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "the absolute path to search in file contents (file or directory)"},
			"pattern": {"type": "string", "description": "the pattern to search for in file contents. When mode is \"regex\", treated as a regular expression. When mode is \"substring\", treated as a literal string."},
			"mode": {"type": "string", "description": "Pattern matching mode. \"regex\" (default): pattern is a regular expression. \"substring\": pattern is a literal string; matches if the file content contains the pattern as a substring.", "enum": ["regex", "substring"]},
			"case_sensitive": {"description": "case sensitive search (default: true)"},
			"include_ignored": {"description": "include files/directories ignored by .gitignore (default: false)"}
		}
	}`)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_content",
		Description: "Search for text content in files under the given path. Supports string and regex matching. When path is a file, searches that single file with context.",
		InputSchema: searchContentArgsSchema,
	}, mfile.HandleSearchContent)

	// Start server based on transport type
	switch transport {
	case "http":
		handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server { return server }, nil)
		runHTTPServer("HTTP", handler, addr)

	case "sse":
		handler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server { return server }, nil)
		runHTTPServer("SSE", handler, addr)

	case "stdio":
		// Stdio mode
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle shutdown signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			log.Println("Received shutdown signal, stopping stdio server...")
			cancel()
		}()

		log.Println("Starting mimi-tools MCP server (stdio)")
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
			log.Printf("Server failed: %v", err)
		}
	}
}

// runHTTPServer starts an HTTP server with graceful shutdown.
// Shared by HTTP and SSE transport modes to eliminate duplicate shutdown logic.
func runHTTPServer(name string, handler http.Handler, addr string) {
	srv := &http.Server{Addr: addr, Handler: handler}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		log.Printf("Starting mimi-tools MCP server (%s) on %s", name, addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("Received shutdown signal, stopping %s server...", name)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("%s server shutdown error: %v", name, err)
	}
}
