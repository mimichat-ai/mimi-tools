#!/bin/bash
# deploy-android.sh
# Deploy mimi-tools MCP server (Android variant) in a secure Podman container
# Supports both online (build) and offline (load from tar) modes
#
# This variant is optimised for Android development:
#   - JDK 21 + Android SDK (API 35, build-tools 35.0.1, platform-tools)
#   - No Go, Node.js, or Python toolchains
#   - Persistent ~/.gradle cache and ~/.vscode-server volumes
#   - Port 2334 (to avoid conflict with the standard mimi-tools on 2333)

set -e

# Configuration
CONTAINER_NAME="mimi-tools-android"
IMAGE_NAME="mimi-tools-android:latest"
IMAGE_TAR="mimi-tools-android.tar"
HOST_PORT=2334
CONTAINER_PORT=2333
PROJECTS_DIR="/projects"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DNS_PRIMARY="${DNS_SERVER:-1.1.1.1}"
DNS_SECONDARY="${DNS_SERVER2:-1.0.0.1}"

# Host directories for persistent data
GIT_CONFIG_DIR="${MIMI_GIT_CONFIG_DIR:-$HOME/.config/mimi-tools/git}"
GRADLE_CACHE_DIR="${MIMI_GRADLE_CACHE_DIR:-$HOME/.config/mimi-tools/gradle}"
VSCODE_SERVER_DIR="${MIMI_VSCODE_SERVER_DIR:-$HOME/.config/mimi-tools/vscode-server}"

# Git config overrides (set these before running deploy-android.sh)
MIMI_GIT_USER_NAME="${MIMI_GIT_USER_NAME:-mimi-tools}"
MIMI_GIT_USER_EMAIL="${MIMI_GIT_USER_EMAIL:-mimi-tools@localhost}"
MIMI_GIT_CONFIG_EXTRA="${MIMI_GIT_CONFIG_EXTRA:-}"

# Build arguments for container image (set these if you need mirrors, e.g. in China)
#   Examples:
#     GOPROXY=https://goproxy.cn,direct
#     APT_MIRROR=mirrors.ustc.edu.cn
#     APT_SECURITY_MIRROR=mirrors.ustc.edu.cn/debian-security
#     ANDROID_SDK_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/android/repository
#     BASE_IMAGE=docker.m.daocloud.io/library/debian:stable-slim
#     GO_BUILDER_IMAGE=docker.m.daocloud.io/library/golang:1.26.5-trixie
BUILD_GOPROXY="${GOPROXY:-}"
BUILD_APT_MIRROR="${APT_MIRROR:-}"
BUILD_APT_SECURITY_MIRROR="${APT_SECURITY_MIRROR:-}"
BUILD_ANDROID_SDK_MIRROR="${ANDROID_SDK_MIRROR:-}"
BUILD_BASE_IMAGE="${BASE_IMAGE:-}"
BUILD_GO_BUILDER_IMAGE="${GO_BUILDER_IMAGE:-}"
BUILD_USER_UID="${USER_UID:-}"
BUILD_USER_GID="${USER_GID:-}"

# Version info (auto-detected from git, overridable via env)
BUILD_VERSION="${MIMI_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
BUILD_COMMIT="${MIMI_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "none")}"
BUILD_BUILDTIME="${MIMI_BUILDTIME:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"

# Transport configuration (http, sse, stdio)
# Note: stdio mode is not suitable for container deployment
MIMI_TRANSPORT="${MIMI_TRANSPORT:-http}"

# Parse arguments
OFFLINE_MODE=false
if [[ "$1" == "--offline" ]] || [[ "$1" == "-o" ]]; then
    OFFLINE_MODE=true
    echo "=== Offline Mode: Loading image from $IMAGE_TAR ==="
fi

echo "=== Deploying mimi-tools MCP Server (Android Variant) ==="
echo "Script directory: $SCRIPT_DIR"

# Step 1: Create config directories
echo "[1/6] Creating config directories..."

mkdir -p "$GIT_CONFIG_DIR"
cat > "$GIT_CONFIG_DIR/.gitconfig" << EOF
[user]
    name = $MIMI_GIT_USER_NAME
    email = $MIMI_GIT_USER_EMAIL
EOF
chmod 644 "$GIT_CONFIG_DIR/.gitconfig"

# Append extra git config if provided
if [ -n "$MIMI_GIT_CONFIG_EXTRA" ]; then
    echo "" >> "$GIT_CONFIG_DIR/.gitconfig"
    echo "$MIMI_GIT_CONFIG_EXTRA" >> "$GIT_CONFIG_DIR/.gitconfig"
fi

# Gradle cache (persistent — avoids re-downloading Gradle distribution and dependencies)
mkdir -p "$GRADLE_CACHE_DIR"

# VS Code Server (persistent — avoids re-downloading on container restart)
mkdir -p "$VSCODE_SERVER_DIR"

# Step 2: Verify projects directory permissions
echo "[2/6] Verifying projects directory permissions..."
if [ ! -d "$PROJECTS_DIR" ]; then
    echo "Error: $PROJECTS_DIR does not exist. Please create it first."
    exit 1
fi

PROJECTS_OWNER=$(stat -c '%U' "$PROJECTS_DIR" 2>/dev/null || echo "unknown")
CURRENT_USER=$(whoami)
if [ "$PROJECTS_OWNER" != "$CURRENT_USER" ]; then
    echo "⚠️  Warning: $PROJECTS_DIR is owned by $PROJECTS_OWNER, not $CURRENT_USER"
    echo "   The container mounts this directory and may have permission issues."
    echo "   To fix, run: sudo chown $CURRENT_USER:$CURRENT_USER $PROJECTS_DIR"
    echo ""
    read -r -p "Continue anyway? [y/N] " response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
fi

# Step 3: Stop and remove existing container
echo "[3/6] Stopping existing container (if any)..."
podman stop "$CONTAINER_NAME" 2>/dev/null || true
podman rm "$CONTAINER_NAME" 2>/dev/null || true

# Step 4: Build or load the image
if [ "$OFFLINE_MODE" = true ]; then
    echo "[4/6] Loading container image from $IMAGE_TAR..."
    if [ ! -f "$SCRIPT_DIR/$IMAGE_TAR" ]; then
        echo "Error: $IMAGE_TAR not found in $SCRIPT_DIR"
        echo "Please copy the image tar file to the script directory."
        exit 1
    fi
    podman load -i "$SCRIPT_DIR/$IMAGE_TAR"
else
    echo "[4/6] Building container image..."

    # Build arguments (only add non-empty values)
    BUILD_ARGS=()
    [ -n "$BUILD_GOPROXY" ] && BUILD_ARGS+=(--build-arg "GOPROXY=$BUILD_GOPROXY")
    [ -n "$BUILD_APT_MIRROR" ] && BUILD_ARGS+=(--build-arg "APT_MIRROR=$BUILD_APT_MIRROR")
    [ -n "$BUILD_APT_SECURITY_MIRROR" ] && BUILD_ARGS+=(--build-arg "APT_SECURITY_MIRROR=$BUILD_APT_SECURITY_MIRROR")
    [ -n "$BUILD_ANDROID_SDK_MIRROR" ] && BUILD_ARGS+=(--build-arg "ANDROID_SDK_MIRROR=$BUILD_ANDROID_SDK_MIRROR")
    [ -n "$BUILD_BASE_IMAGE" ] && BUILD_ARGS+=(--build-arg "BASE_IMAGE=$BUILD_BASE_IMAGE")
    [ -n "$BUILD_GO_BUILDER_IMAGE" ] && BUILD_ARGS+=(--build-arg "GO_BUILDER_IMAGE=$BUILD_GO_BUILDER_IMAGE")
    [ -n "$BUILD_USER_UID" ] && BUILD_ARGS+=(--build-arg "USER_UID=$BUILD_USER_UID")
    [ -n "$BUILD_USER_GID" ] && BUILD_ARGS+=(--build-arg "USER_GID=$BUILD_USER_GID")
    BUILD_ARGS+=(--build-arg "VERSION=$BUILD_VERSION")
    BUILD_ARGS+=(--build-arg "COMMIT=$BUILD_COMMIT")
    BUILD_ARGS+=(--build-arg "BUILDTIME=$BUILD_BUILDTIME")

    if [ ${#BUILD_ARGS[@]} -gt 0 ]; then
        echo "   Build args: ${BUILD_ARGS[*]}"
    fi

    podman build -f Dockerfile.android "${BUILD_ARGS[@]}" -t "$IMAGE_NAME" "$SCRIPT_DIR"
fi

# Step 5: Run the container
echo "[5/6] Starting container..."
podman run -d \
  --name "$CONTAINER_NAME" \
  --userns=keep-id \
  --read-only \
  --cap-drop=ALL \
  --tmpfs /home/mimi-tools/.cache \
  --tmpfs /home/mimi-tools/.local \
  --tmpfs /home/mimi-tools/.android \
  --tmpfs /tmp \
  -v "$GRADLE_CACHE_DIR:/home/mimi-tools/.gradle" \
  -v "$VSCODE_SERVER_DIR:/home/mimi-tools/.vscode-server" \
  --dns "$DNS_PRIMARY" \
  --dns "$DNS_SECONDARY" \
  -p "$HOST_PORT:$CONTAINER_PORT" \
  -e "MIMI_TRANSPORT=$MIMI_TRANSPORT" \
  -v "$PROJECTS_DIR:/projects" \
  -v "$GIT_CONFIG_DIR/.gitconfig:/home/mimi-tools/.gitconfig" \
  -w /projects \
  "$IMAGE_NAME"

# Step 6: Generate and enable systemd service
echo "[6/6] Setting up systemd service for auto-start..."
mkdir -p ~/.config/systemd/user
podman generate systemd --new --name "$CONTAINER_NAME" > ~/.config/systemd/user/${CONTAINER_NAME}.service

systemctl --user daemon-reload
systemctl --user enable "$CONTAINER_NAME"
systemctl --user start "$CONTAINER_NAME"

# Enable linger to keep service running after logout
echo ""
echo "Enabling user linger for persistent service..."
sudo loginctl enable-linger "$(whoami)"

# Verification
echo ""
echo "=== Deployment Complete ==="
echo ""
echo "Variant: Android (JDK 21 + Android SDK API 35)"
echo "Transport mode: $MIMI_TRANSPORT"
echo "Container status:"
podman ps --filter "name=$CONTAINER_NAME"
echo ""
echo "Service status:"
systemctl --user status "$CONTAINER_NAME" --no-pager || true
echo ""
echo "MCP Server should be available at: http://localhost:$HOST_PORT"
echo ""
echo "To view logs: podman logs -f $CONTAINER_NAME"
echo "To stop: podman stop $CONTAINER_NAME"
echo "To restart: podman start $CONTAINER_NAME"
