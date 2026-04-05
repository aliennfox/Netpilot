#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
CONFIG="$PROJECT_DIR/configs/minimal.json"
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
            echo "sing-box not running (stale PID file). Cleaning up."
            rm -f "$PID_FILE"
        fi
    else
        echo "No PID file found. sing-box may not be running."
    fi
}

start_singbox() {
    if ! command -v sing-box &>/dev/null; then
        echo "Error: sing-box not found. Install it first:"
        echo "  brew install sing-box"
        exit 1
    fi

    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            echo "sing-box already running (PID $PID). Use '$0 stop' first."
            exit 1
        fi
        rm -f "$PID_FILE"
    fi

    echo "Validating config..."
    sing-box check -c "$CONFIG"

    echo "Starting sing-box..."
    sing-box run -c "$CONFIG" &
    echo $! > "$PID_FILE"
    sleep 1

    if kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
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
