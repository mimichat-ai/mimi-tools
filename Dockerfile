# Build stage for Go
# Usage for users in China:
#   docker build \
#     --build-arg GO_BUILDER_IMAGE=docker.m.daocloud.io/library/golang:1.26.5-trixie \
#     --build-arg BASE_IMAGE=docker.m.daocloud.io/library/golang:1.26.5-trixie \
#     --build-arg GOPROXY=https://goproxy.cn,direct \
#     -t mimi-tools .

# ─── Global build arguments (must be before first FROM) ────────────────────
ARG GO_BUILDER_IMAGE=golang:1.26.5-trixie
ARG BASE_IMAGE=golang:1.26.5-trixie

FROM ${GO_BUILDER_IMAGE} AS go-builder

ARG GOPROXY=https://proxy.golang.org,direct
RUN go env -w GOPROXY=${GOPROXY}

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
ARG COMMIT=none
ARG BUILDTIME=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILDTIME}" -o mimi-tools ./cmd/mimi-tools

# Final stage - development sandbox image with full toolchain
#
# 🔧 Why not a slim runtime image?
#   mimi-tools exposes an `exec` tool that lets LLMs run arbitrary shell commands,
#   including compiling code, installing packages, debugging, etc. A full Go/Node/Python
#   development environment inside the container enables LLM agents to:
#     - Build and test code before writing it to the host
#     - Run linters, formatters, and static analysis tools
#     - Debug with delve (dlv) and inspect running processes
#     - Execute scripts in multiple languages
#
#   For production deployments where exec is not needed, you can create a slim variant
#   by changing the base image to `debian:stable-slim` and removing the dev tool layers.

FROM ${BASE_IMAGE}

# Allow overriding apt mirror (e.g., for China: --build-arg APT_MIRROR=mirrors.ustc.edu.cn)
ARG APT_MIRROR=deb.debian.org
ARG APT_SECURITY_MIRROR=security.debian.org
# Handle both deb822 (trixie+) and traditional (bookworm) apt source formats
RUN if [ -f /etc/apt/sources.list.d/debian.sources ]; then \
        sed -i "s|deb.debian.org|${APT_MIRROR}|g" /etc/apt/sources.list.d/debian.sources \
        && sed -i "s|security.debian.org|${APT_SECURITY_MIRROR}|g" /etc/apt/sources.list.d/debian.sources; \
    elif [ -f /etc/apt/sources.list ]; then \
        sed -i "s|deb.debian.org|${APT_MIRROR}|g" /etc/apt/sources.list \
        && sed -i "s|security.debian.org|${APT_SECURITY_MIRROR}|g" /etc/apt/sources.list; \
    fi

# Allow overriding Node.js download mirror
ARG NODE_MIRROR=https://nodejs.org/dist
ARG NPM_REGISTRY=https://registry.npmjs.org/

# Install base tools
# Go environment is already included in the golang image
# Install Python
# Install Node.js
# All in one RUN to share apt cache
RUN apt-get update && apt-get install -y \
    # Core utilities (includes cat, cp, mv, rm, mkdir, cut, sort, uniq, etc.)
    coreutils \
    # Text processing
    sed gawk grep \
    # System tools
    procps findutils \
    # Network tools
    iputils-ping net-tools \
    # Compression
    tar gzip zip unzip \
    # Other utilities
    which less man \
    # Hex dump (frequently used by LLMs to inspect binary files)
    xxd \
    # Development basics
    git curl wget jq ca-certificates \
    # Build tools (some pip packages require compilation)
    build-essential \
    # Python
    python3 python3-pip python3-venv \
    # Clean up
    && rm -rf /var/lib/apt/lists/*

# Install Node.js
RUN NODE_VERSION=24.18.0 \
    && ARCH=$(uname -m | sed 's/x86_64/x64/' | sed 's/aarch64/arm64/') \
    && curl -fsSL ${NODE_MIRROR}/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${ARCH}.tar.xz \
    | tar -xJf - -C /usr/local --strip-components=1 \
    && npm config set registry ${NPM_REGISTRY} \
    && npm install -g npm@latest
# Allow overriding user UID (default 1000, change if your host user UID differs)
ARG USER_UID=1000
ARG USER_GID=1000

# Create non-root user
# -U creates a group with the same name as the user
RUN useradd -u ${USER_UID} -U -m -s /bin/bash mimi-tools \
    && mkdir -p /opt/go \
    && chown mimi-tools:mimi-tools /opt/go

# Add PATH for future tools (Rust removed to save space)
# Go is already in PATH from the golang image (/usr/local/go/bin)
ENV GOPATH="/opt/go"
ENV GOMODCACHE="/opt/go/pkg"
# Go module proxy
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
ENV PATH="/usr/local/go/bin:/opt/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

# Copy compiled binary (Go runtime is now available in the final image)
COPY --from=go-builder --chown=mimi-tools:mimi-tools /build/mimi-tools /usr/local/bin/mimi-tools

# Install Go tools (as root, before switching to non-root user)
RUN GOBIN=/usr/local/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest \
    && GOBIN=/usr/local/bin go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest \
    && GOBIN=/usr/local/bin go install github.com/josharian/impl@latest \
    && GOBIN=/usr/local/bin go install github.com/haya14busa/goplay/cmd/goplay@latest \
    && GOBIN=/usr/local/bin go install github.com/go-delve/delve/cmd/dlv@latest \
    && GOBIN=/usr/local/bin go install golang.org/x/tools/gopls@latest \
    && chown -R mimi-tools:mimi-tools /usr/local/bin/golangci-lint /usr/local/bin/sqlc /usr/local/bin/impl /usr/local/bin/goplay /usr/local/bin/dlv /usr/local/bin/gopls \
    && chown -R mimi-tools:mimi-tools /opt/go

# Set working directory
WORKDIR /projects

# Switch to non-root user
USER mimi-tools

# Expose port for HTTP/SSE stream
EXPOSE 2333

# Environment variables
# MIMI_TRANSPORT: transport type (http, sse, stdio), default: http
# MIMI_ADDR: listen address, default: 0.0.0.0:2333
ENV MIMI_TRANSPORT="http"
ENV MIMI_ADDR="0.0.0.0:2333"

# Default command
ENTRYPOINT ["mimi-tools"]