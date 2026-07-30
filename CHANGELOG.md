# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `--version` flag to print version, commit, and build time
- Build-time version injection via `-ldflags` (Makefile, CI, Dockerfile, deploy.sh)
- MCP tools: `exec`, `read`, `write`, `edit`, `tree`, `search_name`, `search_content`
- Transport modes: Streamable HTTP, SSE, stdio
- Fuzzy matching for `edit` tool (whitespace-insensitive, Levenshtein-based)
- Ambiguous match detection for `edit` tool (rejects multiple non-overlapping high-similarity matches)
- Atomic file writes via temp-file + rename
- Git-aware directory tree (respects .gitignore)
- Cross-platform shell command execution (sh -c / cmd /c)
- Bounded output buffers (100 MB per stream)
- Docker container with full Go/Node/Python development sandbox
- Automated MCP test suite (Node.js)
- `MIMI_BASE_URL` environment variable for test suite server URL configuration
- CI integration-test job (GitHub Actions): builds binary, starts HTTP server, runs Node.js test suite

### Fixed
- `readFile` `totalLines` now always returns the file's true total line count, no longer truncated by `endLine`
- `fuzzyMatch` returns `(*MatchSuggestion, bool)` with ambiguity flag for multiple non-overlapping high-score windows
