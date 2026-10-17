#!/bin/sh
# scripts/claude-hook.sh
# Forwards Claude Code hook events to Agent Sentinel observation collector.
# If sentinel is available, delegates via stdin to `sentinel hook --mode observe`.
# If sentinel is not available or collector is offline, returns {} without blocking.

set -e

SENTINEL_BIN="${SENTINEL_BIN:-$HOME/.sentinel/bin/sentinel}"

if [ ! -x "$SENTINEL_BIN" ]; then
    if command -v sentinel >/dev/null 2>&1; then
        SENTINEL_BIN="$(command -v sentinel)"
    fi
fi

if [ -x "$SENTINEL_BIN" ]; then
    exec "$SENTINEL_BIN" hook --mode observe
else
    # Neutral acknowledgement for Claude Code when sentinel is not installed
    echo "{}"
fi
