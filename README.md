# mimi-tools

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![CI](https://github.com/mimichat-ai/mimi-tools/actions/workflows/go.yml/badge.svg)](https://github.com/mimichat-ai/mimi-tools/actions/workflows/go.yml)

An MCP tool server that gives LLM agents filesystem and shell access — built to tolerate LLM quirks and deployed in containers for security isolation.

## Design Philosophy

### Built for LLM fallibility

- **Tolerant argument parsing.** LLMs frequently send numbers as strings (`"30"` instead of `30`) or booleans as strings (`"true"` instead of `true`). JSON schemas deliberately omit `type` on numeric and boolean fields, and the runtime coerces values automatically — tool calls don't fail on type mismatches.

- **Fuzzy edit matching.** When an LLM's `old_string` doesn't exactly match the file (e.g., wrong indentation), the edit tool falls back to whitespace-insensitive fuzzy matching using Levenshtein distance. Edits succeed even when the LLM's reconstruction of whitespace is imperfect.

- **Structured, actionable errors.** Errors include a type, reason, suggestion, and when relevant, a diff of the closest match found — giving the LLM the context to self-correct without human intervention.

### Full power, contained

- **No command restrictions.** The `exec` tool doesn't use blocklists or command filtering — they're trivial to bypass and hinder legitimate use. Security is handled at the container level: non-root user, read-only filesystem, all capabilities dropped.

- **Complete development environment.** The Docker image ships with Go, Node.js, Python, git, and development tools (golangci-lint, delve, etc.). LLM agents can build, test, lint, and debug code — not just read and write files.

- **Android variant.** A separate `Dockerfile.android` provides a JDK 21 + Android SDK (API 35) environment for building native Android apps. See [DEPLOYMENT-ANDROID.md](DEPLOYMENT-ANDROID.md) for details.

## Quick Start

```bash
# Build and run locally
go build -o mimi-tools ./cmd/mimi-tools
./mimi-tools
# → Server listening on http://127.0.0.1:2333
```

> Bare-metal mode runs without container isolation — use for local/trusted environments only. For remote deployment, use Docker (see below) or [DEPLOYMENT.md](DEPLOYMENT.md).

```bash
# Deploy in a container (recommended for remote access)
docker build -t mimi-tools .
docker run -p 2333:2333 -v /projects:/projects mimi-tools
```

<details>
<summary>📱 Android development variant</summary>

```bash
# Build the Android variant (JDK 21 + Android SDK API 35)
docker build -f Dockerfile.android -t mimi-tools-android:latest .
docker run -p 2334:2333 -v /projects:/projects mimi-tools-android:latest
```

See [DEPLOYMENT-ANDROID.md](DEPLOYMENT-ANDROID.md) for full details.

</details>

### Connect your MCP client

**stdio mode** (`claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "mimi-tools": {
      "command": "/path/to/mimi-tools",
      "args": [],
      "env": { "MIMI_TRANSPORT": "stdio" }
    }
  }
}
```

**HTTP mode** (for remote clients):
```json
{
  "mcpServers": {
    "mimi-tools": {
      "url": "http://localhost:2333/"
    }
  }
}
```

## Tools

- **exec** — Execute shell commands (sh -c / cmd /c)
- **read** — Read file content with optional line range
- **write** — Write file content, creating parent directories if needed
- **edit** — Replace a matched string; falls back to whitespace-insensitive fuzzy matching
- **tree** — Show directory tree structure (respects .gitignore)
- **search_name** — Search for files by name (glob / regex / substring)
- **search_content** — Search for text in files with context lines

## Transport Modes

| Mode  | `MIMI_TRANSPORT` | Use case |
|-------|-------------------|----------|
| HTTP  | `http` (default)  | Remote deployment |
| SSE   | `sse`             | Legacy clients |
| Stdio | `stdio`           | Local MCP clients (e.g., Claude Desktop) |

## Documentation

- [DEPLOYMENT.md](DEPLOYMENT.md) — Docker/Podman deployment, deploy.sh, environment variables, security hardening
- [DEPLOYMENT-ANDROID.md](DEPLOYMENT-ANDROID.md) — Android variant deployment (JDK 21 + Android SDK, deploy-android.sh)
- [CONTRIBUTING.md](CONTRIBUTING.md) — How to contribute
- [CHANGELOG.md](CHANGELOG.md) — Release history

## License

MIT
