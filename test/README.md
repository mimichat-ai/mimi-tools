# mimi-tools MCP Test Scripts

Automated test scripts to verify all mimi-tools MCP server tool functionality.

## Prerequisites

1. Ensure mimi-tools server is running at `http://localhost:2337/`
2. Or set environment variable `MIMI_BASE_URL` to specify the server URL (e.g., `http://localhost:2399/`)
3. The server's listen address can be configured via `MIMI_ADDR` (e.g., `MIMI_ADDR=127.0.0.1:2399`)

## Installation

```bash
cd test/
npm install
```

## Running Tests

```bash
npm test
# or
node test_mcp.mjs
```

## Verbose Mode

Add `-v` or `--verbose` flag to see detailed request and response information for failed tests, useful for debugging:

```bash
node test_mcp.mjs -v
# or
node test_mcp.mjs --verbose
```

Add `-vv` or `--verbose-all` to see detailed request and response information for all tests:

```bash
node test_mcp.mjs -vv
# or
node test_mcp.mjs --verbose-all
```

## Test Coverage

| Tool | Test Cases | Description |
|------|-----------|-------------|
| write | 5 | Write file, write directory error, missing content/path, reset file for edit tests |
| read | 8 | Read file, read directory, line range, error scenarios |
| edit | 8 | Replace, whitespace-insensitive match (×3), not found, same string, non-existent file, missing args |
| tree | 6 | Different depths, string-to-number conversion, non-existent/missing path |
| search_name | 8 | String match, regex match, glob, case sensitivity, missing args |
| search_content | 7 | String match, regex match, case sensitivity, missing args |
| exec | 10 | echo, ls, compound commands (&&, ;, \|\|, \|), background, failed commands, string conversion, missing command |
| cleanup | 1 | Remove temp file |
| **Total** | **54** | |

## Output Example

```
🚀 Go mimi-tools MCP Automated Batch Test
=============================================================
✅ Session handshake successful

⚡ Running test queue...
  write            write (file)             ✔ PASS
  write            write (directory) - expected error      ✔ PASS
  ...

=============================================================
📊 Test Summary
=============================================================
 ✅ Passed: 54
 ✅ Failed: 0
 📈 Total: 54
  ⏱️ Duration: 1.42s
=============================================================
```