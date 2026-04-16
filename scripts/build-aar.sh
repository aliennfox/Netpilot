#!/usr/bin/env bash
# 构建 android/app/libs/netpilot.aar —— gomobile bind 封装
# 该 aar 被 .gitignore,因此每次 clone 后首次构建 Android 都要先跑一次本脚本
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$REPO_ROOT/android/app/libs/netpilot.aar"
PKG="./mobile"
ANDROID_API=21  # 对齐 android/app/build.gradle.kts 的 minSdk

if ! command -v gomobile >/dev/null 2>&1; then
    cat >&2 <<'EOF'
错误: 未找到 gomobile。安装步骤:
  go install golang.org/x/mobile/cmd/gomobile@latest
  go install golang.org/x/mobile/cmd/gobind@latest
  gomobile init     # 下载 Android NDK stubs,首次较慢

确保 $(go env GOPATH)/bin 在 PATH 里。
EOF
    exit 1
fi

if ! command -v javac >/dev/null 2>&1; then
    echo "错误: gomobile 需要 JDK(javac);请先装 JDK 17 并 export JAVA_HOME。" >&2
    exit 1
fi

mkdir -p "$(dirname "$OUT")"
cd "$REPO_ROOT"

echo "▶ 构建 $OUT"
echo "  源包:   $PKG"
echo "  minSdk: $ANDROID_API"
gomobile bind -target=android -androidapi "$ANDROID_API" -o "$OUT" "$PKG"

SIZE=$(du -h "$OUT" | awk '{print $1}')
echo "✓ 已生成: $OUT ($SIZE)"
