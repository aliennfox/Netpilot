#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BASE_CONFIG="$PROJECT_DIR/configs/minimal.json"
MERGED_CONFIG="$PROJECT_DIR/data/merged.json"
REFRESH_BIN="$PROJECT_DIR/build/refresh-sub"
REFRESH_SRC="$PROJECT_DIR/cmd/refresh-sub/main.go"
PID_FILE="$PROJECT_DIR/.singbox.pid"

stop_singbox() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            echo "Stopping sing-box (PID $PID)..."
            kill "$PID"
            rm -f "$PID_FILE"
            echo "Stopped."
        else
            rm -f "$PID_FILE"
        fi
    fi
    # orphaned sing-box (e.g. started by refresh-sub) — pkill as safety net
    pkill -x sing-box 2>/dev/null || true
}

ensure_refresh_bin() {
    # Rebuild if binary missing, or source newer than binary
    if [ ! -x "$REFRESH_BIN" ] || [ "$REFRESH_SRC" -nt "$REFRESH_BIN" ]; then
        echo "Building refresh-sub..."
        (cd "$PROJECT_DIR" && go build -o "$REFRESH_BIN" ./cmd/refresh-sub)
    fi
}

refresh_and_start() {
    # refresh-sub fetches subscription, rewrites merged.json, and restarts sing-box
    # via SingBoxAdapter.Reload() (pkill + sing-box run -c merged.json).
    cd "$PROJECT_DIR"
    if "$REFRESH_BIN"; then
        sleep 2
        PID=$(pgrep -x sing-box | head -1)
        if [ -n "$PID" ]; then
            echo "$PID" > "$PID_FILE"
            return 0
        fi
    fi
    return 1
}

plain_start() {
    local cfg="${1:-$MERGED_CONFIG}"
    [ -f "$cfg" ] || cfg="$BASE_CONFIG"
    echo "Validating $cfg..."
    sing-box check -c "$cfg"
    echo "Starting sing-box with $cfg..."
    sing-box run -c "$cfg" &
    echo $! > "$PID_FILE"
    sleep 1
    kill -0 "$(cat "$PID_FILE")" 2>/dev/null
}

start_singbox() {
    if ! command -v sing-box &>/dev/null; then
        echo "Error: sing-box not found. Install it first:"
        echo "  brew install sing-box"
        exit 1
    fi

    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
        echo "sing-box already running (PID $(cat "$PID_FILE")). Use '$0 restart' to refresh+restart."
        exit 1
    fi
    rm -f "$PID_FILE"
    pkill -x sing-box 2>/dev/null || true
    sleep 0.3

    # Try: refresh subscription -> restart sing-box with new merged.json
    if [ -f "$PROJECT_DIR/data/subscriptions.json" ]; then
        ensure_refresh_bin
        if refresh_and_start; then
            echo "sing-box started via refresh (PID $(cat "$PID_FILE"))"
            echo "  Clash API: http://127.0.0.1:9090"
            echo "  Mixed proxy: 127.0.0.1:1080"
            return 0
        fi
        echo "Refresh failed — falling back to plain start."
    fi

    if plain_start "$MERGED_CONFIG"; then
        echo "sing-box started (PID $(cat "$PID_FILE"))"
        echo "  Clash API: http://127.0.0.1:9090"
        echo "  Mixed proxy: 127.0.0.1:1080"
    else
        echo "Error: sing-box failed to start. Check logs."
        rm -f "$PID_FILE"
        exit 1
    fi
}

case "${1:-start}" in
    start)
        start_singbox
        ;;
    stop)
        stop_singbox
        ;;
    restart)
        stop_singbox
        sleep 1
        start_singbox
        ;;
    *)
        echo "Usage: $0 {start|stop|restart}"
        exit 1
        ;;
esac
