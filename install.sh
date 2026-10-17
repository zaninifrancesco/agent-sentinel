#!/bin/sh
# install.sh — Agent Sentinel Universal Installer & Setup Script
#
# Usage:
#   ./install.sh [--agent claude|cursor|all] [--dir PATH] [--user] [--port N]
#
# Or via curl:
#   curl -fsSL https://raw.githubusercontent.com/zaninifrancesco/agent-sentinel/main/install.sh | bash

set -e

REPO="zaninifrancesco/agent-sentinel"
TARGET_DIR="."
AGENT="auto"
PORT=8848
USER_MODE=""
START_SERVE=1
INSTALL_SKILL=1

# --- Parse arguments ---
while [ $# -gt 0 ]; do
    case "$1" in
        --agent)
            AGENT="$2"
            shift 2
            ;;
        --dir)
            TARGET_DIR="$2"
            shift 2
            ;;
        --user)
            USER_MODE="--user"
            shift
            ;;
        --port)
            PORT="$2"
            shift 2
            ;;
        --no-serve)
            START_SERVE=0
            shift
            ;;
        --no-skill)
            INSTALL_SKILL=0
            shift
            ;;
        --help|-h)
            echo "Agent Sentinel Installer & Setup"
            echo ""
            echo "Options:"
            echo "  --agent <claude|cursor|all>  Target AI agent to hook (default: auto-detect)"
            echo "  --dir <path>                 Target project directory (default: current dir)"
            echo "  --user                       Install hooks globally for all projects"
            echo "  --port <port>                Port for collector and cockpit (default: 8848)"
            echo "  --no-serve                   Do not start background collector daemon"
            echo "  --no-skill                   Do not copy skill to ~/.cursor and ~/.claude"
            echo "  --help                       Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            echo "Run with --help for usage." >&2
            exit 1
            ;;
    esac
done

SENTINEL_DIR="$HOME/.sentinel"
BIN_DIR="$SENTINEL_DIR/bin"
SENTINEL_BIN="$BIN_DIR/sentinel"

mkdir -p "$BIN_DIR"

echo "=== Agent Sentinel Installer ==="

# --- Step 1: Detect Platform ---
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    darwin|linux) ;;
    *)
        echo "Error: Unsupported operating system '$OS'. Agent Sentinel supports macOS (darwin) and Linux." >&2
        exit 1
        ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)
        echo "Error: Unsupported architecture '$ARCH'. Supported: amd64, arm64." >&2
        exit 1
        ;;
esac

echo "Platform: ${OS}/${ARCH}"

# --- Step 2: Binary Installation / Verification ---
HAVE_VALID_BIN=0
if [ -x "$SENTINEL_BIN" ]; then
    if "$SENTINEL_BIN" help 2>&1 | grep -q "serve"; then
        HAVE_VALID_BIN=1
        echo "Found existing Sentinel binary: $("$SENTINEL_BIN" version)"
    fi
fi

if [ "$HAVE_VALID_BIN" -eq 0 ]; then
    # Try local pre-built binary if inside repo
    SCRIPT_DIR="$(cd "$(dirname "$0")" 2>/dev/null && pwd || true)"
    if [ -n "$SCRIPT_DIR" ] && [ -x "$SCRIPT_DIR/bin/sentinel" ]; then
        echo "Using local build from $SCRIPT_DIR/bin/sentinel..."
        install -m 0755 "$SCRIPT_DIR/bin/sentinel" "$SENTINEL_BIN"
    elif [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/Makefile" ] && command -v go >/dev/null 2>&1 && command -v node >/dev/null 2>&1; then
        echo "Compiling from source in $SCRIPT_DIR..."
        (cd "$SCRIPT_DIR" && make install)
    else
        echo "Downloading latest prebuilt binary from GitHub ($REPO)..."
        LATEST_URL="https://api.github.com/repos/${REPO}/releases/latest"
        TAG=$(curl -sf "$LATEST_URL" 2>/dev/null | grep '"tag_name":' | head -n 1 | cut -d '"' -f 4 || true)
        if [ -z "$TAG" ]; then
            TAG="v0.1.0-alpha.2"
        fi
        VERSION="${TAG#v}"
        ARCHIVE_NAME="sentinel_${VERSION}_${OS}_${ARCH}.tar.gz"
        DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE_NAME}"
        
        TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'sentinel')"
        echo "Fetching $DOWNLOAD_URL..."
        if curl -fSL "$DOWNLOAD_URL" -o "$TMP_DIR/$ARCHIVE_NAME" 2>/dev/null; then
            tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR"
            install -m 0755 "$TMP_DIR/sentinel_${VERSION}_${OS}_${ARCH}/sentinel" "$SENTINEL_BIN"
            rm -rf "$TMP_DIR"
            echo "Installed $SENTINEL_BIN ($TAG)"
        else
            rm -rf "$TMP_DIR"
            echo "Could not download prebuilt release archive from $DOWNLOAD_URL." >&2
            if command -v go >/dev/null 2>&1; then
                echo "Attempting build via Go..."
                go install "github.com/${REPO}/cmd/sentinel@latest"
                if [ -x "$HOME/go/bin/sentinel" ]; then
                    install -m 0755 "$HOME/go/bin/sentinel" "$SENTINEL_BIN"
                fi
            else
                echo "Please install Go 1.27+ and Node.js to build from source, or check release availability." >&2
                exit 1
            fi
        fi
    fi
fi

chmod +x "$SENTINEL_BIN"
echo "Verified binary: $("$SENTINEL_BIN" version)"

# --- Step 3: Start / Verify Background Collector ---
if [ "$START_SERVE" -eq 1 ]; then
    CONFIG_JSON=$(curl -sf "http://127.0.0.1:${PORT}/api/config" 2>/dev/null || true)
    if [ -n "$CONFIG_JSON" ] && echo "$CONFIG_JSON" | grep -q '"mode":"observe"'; then
        echo "Sentinel collector already running on port ${PORT} in observation mode."
    else
        echo "Starting Sentinel collector in background on port ${PORT}..."
        nohup "$SENTINEL_BIN" serve --mode observe --port "$PORT" > "$SENTINEL_DIR/serve.log" 2>&1 &
        
        # Wait up to 5 seconds for collector to respond
        ATTEMPTS=0
        while [ $ATTEMPTS -lt 25 ]; do
            CONFIG_JSON=$(curl -sf "http://127.0.0.1:${PORT}/api/config" 2>/dev/null || true)
            if [ -n "$CONFIG_JSON" ] && echo "$CONFIG_JSON" | grep -q '"mode":"observe"'; then
                break
            fi
            sleep 0.2
            ATTEMPTS=$((ATTEMPTS + 1))
        done
        
        if [ $ATTEMPTS -ge 25 ]; then
            echo "Warning: Collector did not answer within 5s. Check $SENTINEL_DIR/serve.log" >&2
        else
            echo "Sentinel collector active at http://127.0.0.1:${PORT}"
        fi
    fi
fi

# --- Step 4: Auto-detect & Install Hooks ---
ABS_TARGET="$(cd "$TARGET_DIR" 2>/dev/null && pwd || echo "$TARGET_DIR")"

INSTALL_CURSOR=0
INSTALL_CLAUDE=0

if [ "$AGENT" = "all" ]; then
    INSTALL_CURSOR=1
    INSTALL_CLAUDE=1
elif [ "$AGENT" = "cursor" ]; then
    INSTALL_CURSOR=1
elif [ "$AGENT" = "claude" ]; then
    INSTALL_CLAUDE=1
else
    # Auto-detect
    if [ -d "$ABS_TARGET/.cursor" ] || [ -d "$HOME/.cursor" ]; then
        INSTALL_CURSOR=1
    fi
    if [ -d "$ABS_TARGET/.claude" ] || [ -d "$HOME/.claude" ]; then
        INSTALL_CLAUDE=1
    fi
    if [ "$INSTALL_CURSOR" -eq 0 ] && [ "$INSTALL_CLAUDE" -eq 0 ]; then
        # Default to both if neither directory exists yet
        INSTALL_CURSOR=1
        INSTALL_CLAUDE=1
    fi
fi

if [ "$INSTALL_CURSOR" -eq 1 ]; then
    echo "Installing Cursor hooks in $ABS_TARGET..."
    # shellcheck disable=SC2086
    "$SENTINEL_BIN" hook install --dir "$ABS_TARGET" $USER_MODE
fi

if [ "$INSTALL_CLAUDE" -eq 1 ]; then
    echo "Installing Claude Code hooks in $ABS_TARGET..."
    # shellcheck disable=SC2086
    "$SENTINEL_BIN" hook install --agent claude --dir "$ABS_TARGET" $USER_MODE
fi

# --- Step 5: Synthetic Self-Test Proof ---
echo "Verifying telemetry capture via synthetic self-test..."
if [ "$INSTALL_CURSOR" -eq 1 ]; then
    printf '%s\n' "{\"hook_event_name\":\"beforeShellExecution\",\"conversation_id\":\"setup\",\"command\":\"echo sentinel-setup-ok\",\"cwd\":\"$ABS_TARGET\"}" \
        | "$SENTINEL_BIN" hook --mode observe >/dev/null 2>&1 || true
    printf '%s\n' "{\"hook_event_name\":\"afterShellExecution\",\"conversation_id\":\"setup\",\"command\":\"echo sentinel-setup-ok\",\"output\":\"sentinel-setup-ok\\n\",\"duration\":1}" \
        | "$SENTINEL_BIN" hook --mode observe >/dev/null 2>&1 || true
fi

if [ "$INSTALL_CLAUDE" -eq 1 ]; then
    printf '%s\n' "{\"hook_event_name\":\"PreToolUse\",\"session_id\":\"setup\",\"cwd\":\"$ABS_TARGET\",\"tool_name\":\"Bash\",\"tool_input\":{\"command\":\"echo sentinel-setup-ok\"},\"tool_use_id\":\"setup-1\"}" \
        | "$SENTINEL_BIN" hook --mode observe >/dev/null 2>&1 || true
    printf '%s\n' "{\"hook_event_name\":\"PostToolUse\",\"session_id\":\"setup\",\"tool_name\":\"Bash\",\"tool_input\":{\"command\":\"echo sentinel-setup-ok\"},\"tool_response\":{\"stdout\":\"sentinel-setup-ok\",\"stderr\":\"\",\"interrupted\":false},\"tool_use_id\":\"setup-1\",\"duration_ms\":1}" \
        | "$SENTINEL_BIN" hook --mode observe >/dev/null 2>&1 || true
fi

EVENTS_JSON=$(curl -sf "http://127.0.0.1:${PORT}/api/events" 2>/dev/null || true)
if echo "$EVENTS_JSON" | grep -q "sentinel-setup-ok"; then
    echo "Self-test verified: synthetic request and response recorded."
else
    echo "Notice: Synthetic test sent. Collector events will appear once session connects."
fi

# --- Step 6: Install Skills to Agent Directories ---
if [ "$INSTALL_SKILL" -eq 1 ]; then
    SCRIPT_DIR="$(cd "$(dirname "$0")" 2>/dev/null && pwd || true)"
    SKILL_SRC=""
    if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/skills/agent-sentinel/SKILL.md" ]; then
        SKILL_SRC="$SCRIPT_DIR/skills/agent-sentinel"
    fi

    if [ -n "$SKILL_SRC" ]; then
        if [ -d "$HOME/.cursor" ]; then
            mkdir -p "$HOME/.cursor/skills/agent-sentinel"
            cp -r "$SKILL_SRC/"* "$HOME/.cursor/skills/agent-sentinel/"
            echo "Installed skill to $HOME/.cursor/skills/agent-sentinel"
        fi
        if [ -d "$HOME/.claude" ]; then
            mkdir -p "$HOME/.claude/skills/agent-sentinel"
            cp -r "$SKILL_SRC/"* "$HOME/.claude/skills/agent-sentinel/"
            echo "Installed skill to $HOME/.claude/skills/agent-sentinel"
        fi
        if [ -d "$HOME/.agents" ]; then
            mkdir -p "$HOME/.agents/skills/agent-sentinel"
            cp -r "$SKILL_SRC/"* "$HOME/.agents/skills/agent-sentinel/"
            echo "Installed skill to $HOME/.agents/skills/agent-sentinel"
        fi
    fi
fi

echo ""
echo "============================================================"
echo " Agent Sentinel is ready!"
echo " Cockpit Dashboard: http://127.0.0.1:${PORT}"
echo " Sessions Log:      $SENTINEL_DIR/sessions/"
echo " Mode:              Passive Observation (non-blocking)"
echo "============================================================"
if [ "$INSTALL_CLAUDE" -eq 1 ]; then
    echo "Note for Claude Code: Start a new session ('claude') to reload settings."
fi
echo ""
