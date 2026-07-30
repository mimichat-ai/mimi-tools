# Contributing to mimi-tools

Thank you for considering contributing to mimi-tools!

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for everyone.

## How to Contribute

### Report Bugs

Open an issue with:
- A clear title and description
- Steps to reproduce
- Expected vs actual behavior
- Environment details (OS, Go version, transport mode)

### Suggest Features

Open an issue with:
- A clear description of the feature
- Use case(s) and motivation
- Any prior art or alternatives considered

### Submit Pull Requests

1. **Fork** the repository
2. **Create a branch** for your change (`git checkout -b feature/your-feature`)
3. **Make your changes**
4. **Run tests**: `make ci` (runs lint, test, build)
5. **Commit** with a clear message
6. **Push** and open a PR against the `main` branch

## Development Setup

```bash
# Clone and build
git clone https://github.com/mimichat-ai/mimi-tools.git
cd mimi-tools
go build -o bin/mimi-tools ./cmd/mimi-tools

# Run tests
go test -v -count=1 ./...

# Run lint (requires golangci-lint)
make lint

# Quick CI check
make ci
```

## Code Style

- Run `go fmt ./...` before committing
- Follow standard Go conventions
- Add tests for new functionality
- Keep the `internal/` package boundary — the public API is the MCP tool interface

## License

By contributing, you agree that your contributions will be licensed under the MIT License.