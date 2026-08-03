# Deployment (Android Variant)

Detailed guide for deploying the Android development variant of mimi-tools in containers.

This variant is optimised for building native Android apps (Java/Kotlin + Gradle). It does **not** include Go, Node.js, or Python toolchains.

## What's Included

| Component | Version | Purpose |
|-----------|---------|---------|
| OpenJDK | 21 (headless) | Java/Kotlin compilation |
| Android SDK cmdline-tools | latest (15859902) | `sdkmanager`, `avdmanager` |
| Android SDK platform-tools | latest | `adb`, `fastboot` (network debugging) |
| Android SDK platform | android-35 (API 35) | Compilation target |
| Android SDK build-tools | 35.0.1 | `aapt2`, `d8`, `zipalign`, etc. |
| Linux utilities | — | git, curl, wget, jq, grep, sed, awk, etc. |

## What's NOT Included

- Go toolchain (use the standard `Dockerfile` for Go development)
- Node.js / npm (use the standard `Dockerfile` for JavaScript/TypeScript)
- Python (use the standard `Dockerfile` for Python)
- Android emulator / system images (this variant is for compilation only)
- NDK / CMake (no native C/C++ code support)

## Differences from Standard Deployment

| Aspect | Standard (`Dockerfile`) | Android (`Dockerfile.android`) |
|--------|------------------------|-------------------------------|
| Base image | `golang:1.26.5-trixie` | `debian:stable-slim` |
| JDK | — | OpenJDK 21 (headless) |
| Android SDK | — | API 35, build-tools 35.0.1 |
| Go / Node / Python | ✅ | ❌ |
| Container name | `mimi-tools` | `mimi-tools-android` |
| Port | 2333 | 2334 |
| `~/.gradle` volume | — | ✅ (persistent) |
| `~/.vscode-server` volume | tmpfs (volatile) | ✅ (persistent) |
| `~/.npm` tmpfs | ✅ | ❌ (removed) |
| `/opt/go` tmpfs | ✅ | ❌ (removed) |
| `~/.android` tmpfs | — | ✅ (for adb keys) |

## Quick Start

```bash
# Build and deploy
./deploy-android.sh

# Or build manually
podman build -f Dockerfile.android -t mimi-tools-android:latest .
podman run -d --name mimi-tools-android -p 2334:2333 -v /projects:/projects mimi-tools-android:latest
```

## Transport Modes

Same as the standard deployment. See [DEPLOYMENT.md](DEPLOYMENT.md) for details.

| Mode | `MIMI_TRANSPORT` | Description |
|------|-------------------|-------------|
| HTTP | `http` | Streamable HTTP (default) |
| SSE | `sse` | Server-Sent Events |
| Stdio | `stdio` | Standard Input/Output |

## Environment Variables

### Server Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MIMI_TRANSPORT` | `http` | Transport mode: `http`, `sse`, or `stdio` |
| `MIMI_ADDR` | `0.0.0.0:2333` | Listen address (only for http/sse modes) |

### Android SDK Configuration (set in Dockerfile)

| Variable | Value | Description |
|----------|-------|-------------|
| `ANDROID_SDK_ROOT` | `/opt/android-sdk` | Android SDK root directory |
| `ANDROID_HOME` | `/opt/android-sdk` | Alias for `ANDROID_SDK_ROOT` (deprecated but some tools still use it) |
| `JAVA_HOME` | `/usr/lib/jvm/java-21-current` | JDK 21 home directory |

### deploy-android.sh Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MIMI_TRANSPORT` | `http` | Container transport mode (`http`, `sse`) |
| `DNS_SERVER` | `1.1.1.1` | Primary DNS server for the container |
| `DNS_SERVER2` | `1.0.0.1` | Secondary DNS server for the container |
| `MIMI_GIT_CONFIG_DIR` | `~/.config/mimi-tools/git` | Host path for `.gitconfig` file |
| `MIMI_GIT_USER_NAME` | `mimi-tools` | Git user name in container's `.gitconfig` |
| `MIMI_GIT_USER_EMAIL` | `mimi-tools@localhost` | Git user email in container's `.gitconfig` |
| `MIMI_GIT_CONFIG_EXTRA` | *(empty)* | Additional git config lines appended to `.gitconfig` |
| `MIMI_GRADLE_CACHE_DIR` | `~/.config/mimi-tools/gradle` | Host path for persistent Gradle cache |
| `MIMI_VSCODE_SERVER_DIR` | `~/.config/mimi-tools/vscode-server` | Host path for persistent VS Code Server |
| `GOPROXY` | *(empty)* | Go module proxy, passed as `--build-arg` (only used in builder stage) |
| `APT_MIRROR` | *(empty)* | APT mirror, passed as `--build-arg` (e.g. `mirrors.ustc.edu.cn`) |
| `APT_SECURITY_MIRROR` | *(empty)* | APT security mirror, passed as `--build-arg` |
| `ANDROID_SDK_MIRROR` | *(empty)* | Android SDK download mirror, passed as `--build-arg` |
| `BASE_IMAGE` | *(empty)* | Final stage base image, passed as `--build-arg` (e.g. `docker.m.daocloud.io/library/debian:stable-slim`) |
| `GO_BUILDER_IMAGE` | *(empty)* | Go builder stage base image, passed as `--build-arg` (e.g. `docker.m.daocloud.io/library/golang:1.26.5-trixie`) |
| `USER_UID` | *(empty)* | Container user UID, passed as `--build-arg` |
| `USER_GID` | *(empty)* | Container user GID, passed as `--build-arg` |
| `MIMI_VERSION` | *(auto)* | Version string for `--build-arg VERSION` (default: `git describe`) |
| `MIMI_COMMIT` | *(auto)* | Git commit for `--build-arg COMMIT` (default: `git rev-parse --short HEAD`) |
| `MIMI_BUILDTIME` | *(auto)* | Build timestamp for `--build-arg BUILDTIME` (default: `date -u`) |

## Docker / Podman

```bash
# Build image (version auto-detected from git)
podman build -f Dockerfile.android -t mimi-tools-android:latest .

# Or specify version explicitly
podman build \
  -f Dockerfile.android \
  --build-arg VERSION=$(git describe --tags --always --dirty) \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILDTIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  -t mimi-tools-android:latest .

# HTTP mode (default)
podman run -p 2334:2333 -v /projects:/projects mimi-tools-android:latest

# SSE mode
podman run -p 2334:2333 -v /projects:/projects -e MIMI_TRANSPORT=sse mimi-tools-android:latest
```

> Podman users: replace `docker` with `podman` in the commands above.

<details>
<summary>🐉 For users in China — use domestic mirrors for faster builds</summary>

> **Important:** Docker Hub (`registry-1.docker.io`) is often blocked or very slow in China. You must override the base images via `BASE_IMAGE` and `GO_BUILDER_IMAGE` build-args, otherwise the build will fail at the image pull step.

**Option A: Direct `podman build`**

```bash
podman build \
  -f Dockerfile.android \
  --build-arg GO_BUILDER_IMAGE=docker.m.daocloud.io/library/golang:1.26.5-trixie \
  --build-arg BASE_IMAGE=docker.m.daocloud.io/library/debian:stable-slim \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg APT_MIRROR=mirrors.ustc.edu.cn \
  --build-arg APT_SECURITY_MIRROR=mirrors.ustc.edu.cn/debian-security \
  --build-arg ANDROID_SDK_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/android/repository \
  -t mimi-tools-android:latest .
```

**Option B: Via `deploy-android.sh` (recommended)**

```bash
GO_BUILDER_IMAGE=docker.m.daocloud.io/library/golang:1.26.5-trixie \
BASE_IMAGE=docker.m.daocloud.io/library/debian:stable-slim \
GOPROXY=https://goproxy.cn,direct \
APT_MIRROR=mirrors.ustc.edu.cn \
APT_SECURITY_MIRROR=mirrors.ustc.edu.cn/debian-security \
ANDROID_SDK_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/android/repository \
DNS_SERVER=223.5.5.5 DNS_SERVER2=223.6.6.6 \
./deploy-android.sh
```

> **Note:** The `ANDROID_SDK_MIRROR` only affects the cmdline-tools download. The `sdkmanager` downloads SDK packages (platform-tools, platforms, build-tools) from `dl.google.com` internally. This is usually accessible in China, but may be slower. The cmdline-tools download is the largest single file (~150 MB), so mirroring it provides the most benefit.

> **Docker Hub mirrors:** Common options include `docker.m.daocloud.io`, `docker.1panel.live`, etc. Availability may change over time — use whichever mirror is currently accessible.

</details>

## Deployment with deploy-android.sh

Use `deploy-android.sh` to deploy in a Podman container with systemd integration:

```bash
# HTTP mode (default)
./deploy-android.sh

# SSE mode
MIMI_TRANSPORT=sse ./deploy-android.sh

# Override git user identity
MIMI_GIT_USER_NAME="Zhang San" MIMI_GIT_USER_EMAIL="zhangsan@example.com" ./deploy-android.sh

# With extra git config
MIMI_GIT_USER_NAME="Zhang San" \
MIMI_GIT_USER_EMAIL="zhangsan@example.com" \
MIMI_GIT_CONFIG_EXTRA="[core]
    autocrlf = input
[init]
    defaultBranch = main" \
./deploy-android.sh
```

## Container Security

The `deploy-android.sh` script deploys mimi-tools with the following security measures:

- **Non-root user**: The container runs as the `mimi-tools` user (UID configurable via `USER_UID`)
- **Read-only filesystem**: `--read-only` prevents writes outside of mounted volumes and tmpfs
- **All capabilities dropped**: `--cap-drop=ALL` removes all Linux capabilities
- **User namespace isolation**: `--userns=keep-id` maps the container user to the host user
- **Writable directories via tmpfs**: `/tmp`, `/home/mimi-tools/.cache`, `/home/mimi-tools/.local`, `/home/mimi-tools/.android` are writable tmpfs mounts
- **Persistent volumes**: `~/.gradle` and `~/.vscode-server` are mounted from host directories (survive container restarts)
- **DNS configuration**: Custom DNS servers prevent DNS-based attacks
- **Port mapping**: Only port 2334 is exposed to the host

## Usage Examples

### Building an Android App

Once the container is running, the LLM can use the `exec` tool to build Android projects:

```bash
# Navigate to project directory
cd /projects/my-android-app

# Build debug APK
./gradlew assembleDebug

# Build release AAB
./gradlew bundleRelease

# Run unit tests
./gradlew test

# Check for lint issues
./gradlew lint
```

### Connecting to a Physical Device via adb

```bash
# Connect to a device on the network (device must have adb over TCP enabled)
adb connect 192.168.1.100:5555

# Install APK
adb install app/build/outputs/apk/debug/app-debug.apk

# View logs
adb logcat
```

### VS Code Remote Development

The container supports VS Code Remote development. The `~/.vscode-server` directory is persisted on the host, so VS Code Server only needs to be downloaded once.

1. Install the [Remote - SSH](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-ssh) extension in VS Code
2. Connect to the host machine running the container
3. Use VS Code's "Remote-Containers: Attach to Running Container" feature to connect to the `mimi-tools-android` container
4. VS Code Server will be downloaded to the persisted `~/.vscode-server` directory

## Running Both Variants Simultaneously

The standard mimi-tools (port 2333) and the Android variant (port 2334) can run simultaneously:

```bash
# Deploy standard variant
./deploy.sh

# Deploy Android variant
./deploy-android.sh

# Both are now running:
# - Standard: http://localhost:2333
# - Android:  http://localhost:2334
```
