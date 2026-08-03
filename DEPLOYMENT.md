# Deployment

Detailed guide for deploying mimi-tools in containers.

## Transport Modes

mimi-tools supports three transport modes, controlled by the `MIMI_TRANSPORT` environment variable:

| Mode | `MIMI_TRANSPORT` | Description |
|------|-------------------|-------------|
| HTTP | `http` | Streamable HTTP (default) |
| SSE | `sse` | Server-Sent Events |
| Stdio | `stdio` | Standard Input/Output |

### HTTP Mode (Default)

```bash
MIMI_TRANSPORT=http mimi-tools
# or
mimi-tools  # defaults to http
```

Listens on `MIMI_ADDR` (default: `127.0.0.1:2333`). Clients send POST requests to the server.

### SSE Mode

```bash
MIMI_TRANSPORT=sse mimi-tools
```

Listens on `MIMI_ADDR` (default: `127.0.0.1:2333`). Clients connect via GET to `/sse` for Server-Sent Events.

### Stdio Mode

```bash
MIMI_TRANSPORT=stdio mimi-tools
```

Reads from stdin and writes to stdout. Suitable for local MCP client integration.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MIMI_TRANSPORT` | `http` | Transport mode: `http`, `sse`, or `stdio` |
| `MIMI_ADDR` | `127.0.0.1:2333` | Listen address (only for http/sse modes) |

## Docker / Podman

```bash
# Build image (version auto-detected from git)
docker build -t mimi-tools .

# Or specify version explicitly
docker build \
  --build-arg VERSION=$(git describe --tags --always --dirty) \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILDTIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  -t mimi-tools .

# HTTP mode (default)
docker run -p 2333:2333 -v /projects:/projects mimi-tools:latest

# SSE mode
docker run -p 2333:2333 -v /projects:/projects -e MIMI_TRANSPORT=sse mimi-tools:latest
```

> Podman users: replace `docker` with `podman` in the commands above.

<details>
<summary>🐉 For users in China — use domestic mirrors for faster builds</summary>

**Option A: Direct `docker build` / `podman build`**

```bash
docker build \
  --build-arg GO_BUILDER_IMAGE=docker.m.daocloud.io/library/golang:1.26.5-trixie \
  --build-arg BASE_IMAGE=docker.m.daocloud.io/library/golang:1.26.5-trixie \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg APT_MIRROR=mirrors.ustc.edu.cn \
  --build-arg APT_SECURITY_MIRROR=mirrors.ustc.edu.cn/debian-security \
  --build-arg NODE_MIRROR=https://npmmirror.com/mirrors/node \
  --build-arg NPM_REGISTRY=https://registry.npmmirror.com \
  -t mimi-tools .
```

**Option B: Via `deploy.sh` (recommended)**

Set the same variables as environment variables before running `deploy.sh`:

```bash
GO_BUILDER_IMAGE=docker.m.daocloud.io/library/golang:1.26.5-trixie \
BASE_IMAGE=docker.m.daocloud.io/library/golang:1.26.5-trixie \
GOPROXY=https://goproxy.cn,direct \
APT_MIRROR=mirrors.ustc.edu.cn \
APT_SECURITY_MIRROR=mirrors.ustc.edu.cn/debian-security \
NODE_MIRROR=https://npmmirror.com/mirrors/node \
NPM_REGISTRY=https://registry.npmmirror.com \
DNS_SERVER=223.5.5.5 DNS_SERVER2=223.6.6.6 \
./deploy.sh
```

</details>

## Deployment with deploy.sh

Use `deploy.sh` to deploy in a Podman container with systemd integration:

```bash
# HTTP mode (default)
./deploy.sh

# SSE mode
MIMI_TRANSPORT=sse ./deploy.sh

# Override git user identity
MIMI_GIT_USER_NAME="Zhang San" MIMI_GIT_USER_EMAIL="zhangsan@example.com" ./deploy.sh

# With extra git config (e.g., signing key, default branch)
MIMI_GIT_USER_NAME="Zhang San" \
MIMI_GIT_USER_EMAIL="zhangsan@example.com" \
MIMI_GIT_CONFIG_EXTRA="[core]
    autocrlf = input
[init]
    defaultBranch = main" \
./deploy.sh
```

### deploy.sh Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MIMI_TRANSPORT` | `http` | Container transport mode (`http`, `sse`) |
| `DNS_SERVER` | `1.1.1.1` | Primary DNS server for the container |
| `DNS_SERVER2` | `1.0.0.1` | Secondary DNS server for the container |
| `MIMI_GIT_CONFIG_DIR` | `~/.config/mimi-tools/git` | Host path for generated `.gitconfig` file |
| `MIMI_GIT_USER_NAME` | `mimi-tools` | Git user name in container's `.gitconfig` |
| `MIMI_GIT_USER_EMAIL` | `mimi-tools@localhost` | Git user email in container's `.gitconfig` |
| `MIMI_GIT_CONFIG_EXTRA` | *(empty)* | Additional git config lines appended to `.gitconfig` |
| `MIMI_GO_CONFIG_DIR` | `~/.config/mimi-tools/go` | Host path for Go environment config |
| `MIMI_VSCODE_SERVER_DIR` | `~/.config/mimi-tools/vscode-server` | Host path for persistent VS Code Server |
| `GOPROXY` | *(empty)* | Go module proxy, passed as `--build-arg` to `podman build` (e.g. `https://goproxy.cn,direct`) |
| `APT_MIRROR` | *(empty)* | APT mirror, passed as `--build-arg` (e.g. `mirrors.ustc.edu.cn`) |
| `APT_SECURITY_MIRROR` | *(empty)* | APT security mirror, passed as `--build-arg` |
| `NODE_MIRROR` | *(empty)* | Node.js download mirror, passed as `--build-arg` |
| `NPM_REGISTRY` | *(empty)* | NPM registry, passed as `--build-arg` |
| `BASE_IMAGE` | *(empty)* | Final stage base image, passed as `--build-arg` (e.g. `docker.m.daocloud.io/library/golang:1.26.5-trixie`) |
| `GO_BUILDER_IMAGE` | *(empty)* | Go builder stage base image, passed as `--build-arg` (e.g. `docker.m.daocloud.io/library/golang:1.26.5-trixie`) |
| `USER_UID` | *(empty)* | Container user UID, passed as `--build-arg` |
| `USER_GID` | *(empty)* | Container user GID, passed as `--build-arg` |
| `MIMI_VERSION` | *(auto)* | Version string for `--build-arg VERSION` (default: `git describe`) |
| `MIMI_COMMIT` | *(auto)* | Git commit for `--build-arg COMMIT` (default: `git rev-parse --short HEAD`) |
| `MIMI_BUILDTIME` | *(auto)* | Build timestamp for `--build-arg BUILDTIME` (default: `date -u`) |

## Container Security

The `deploy.sh` script deploys mimi-tools with the following security measures:

- **Non-root user**: The container runs as the `mimi-tools` user (UID configurable via `USER_UID`)
- **Read-only filesystem**: `--read-only` prevents writes outside of mounted volumes and tmpfs
- **All capabilities dropped**: `--cap-drop=ALL` removes all Linux capabilities
- **User namespace isolation**: `--userns=keep-id` maps the container user to the host user
- **Writable directories via tmpfs**: `/tmp`, `/home/mimi-tools/.cache`, `/opt/go`, etc. are writable tmpfs mounts
- **DNS configuration**: Custom DNS servers prevent DNS-based attacks
- **Port mapping**: Only port 2333 is exposed to the host

The Docker image defaults to `MIMI_ADDR=0.0.0.0:2333` (all interfaces) so the server is accessible from outside the container. This is safe because the container provides isolation. On bare metal, the default is `127.0.0.1:2333` (localhost only) — do not change this to `0.0.0.0` on shared or public servers without additional authentication.
