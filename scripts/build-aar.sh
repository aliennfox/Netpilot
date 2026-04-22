#!/usr/bin/env bash
# 构建 android/app/libs/netpilot.aar —— gomobile bind 封装 sing-box libbox。
# 该 aar 被 .gitignore, 每次 clone 后首次构建 Android 都要先跑一次本脚本。
# 每次改 mobile/ 或 libcore/ 后也要重跑, 否则 Kotlin 侧会报 Unresolved reference (见 CLAUDE.md #M8)。
#
# 2026-04-17 Phase 3B-1 重写: 绑定包从 ./mobile 扩到 ./libcore ./mobile,
# 硬编码 Go 1.25 + JDK 17 + checklinkname=0 等一整套工具链约束 (详见 docs/phase-3b-plan.md §11)。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$REPO_ROOT/android/app/libs/netpilot.aar"
ANDROID_API=21  # 对齐 android/app/build.gradle.kts 的 minSdk

# ── 工具链 ─────────────────────────────────────────────────────────────────
# Go: 1.25.x 是 sing-box v1.13.8 + gomobile latest 的唯一 sweet spot。
#   - 1.23/1.24 过不了传递依赖 certmagic 和 mobile 的 go 最低版本门槛
#   - 1.26+ os.checkPidfdOnce linkname 被收紧失败 (见 CLAUDE.md #M11)
if [ -x "/opt/homebrew/opt/go@1.25/libexec/bin/go" ]; then
    export GOROOT="/opt/homebrew/opt/go@1.25/libexec"
elif [ -x "/usr/local/opt/go@1.25/libexec/bin/go" ]; then
    export GOROOT="/usr/local/opt/go@1.25/libexec"
else
    echo "错误: 未找到 Go 1.25。装法: brew install go@1.25" >&2
    exit 1
fi
export GOTOOLCHAIN=local  # 禁止 Go 自动升级到 1.26 (与 sing-box linkname 不兼容)
export PATH="$GOROOT/bin:$PATH:$(go env GOPATH)/bin"

# JDK 17: 对齐 android/app/build.gradle.kts 的 sourceCompatibility
if [ -d "/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home" ]; then
    export JAVA_HOME="/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home"
elif [ -d "/opt/homebrew/opt/openjdk@17" ]; then
    export JAVA_HOME="/opt/homebrew/opt/openjdk@17"
else
    echo "错误: 未找到 JDK 17。装法: brew install openjdk@17" >&2
    exit 1
fi
export PATH="$JAVA_HOME/bin:$PATH"

# Android NDK
if [ -z "${ANDROID_NDK_HOME:-}" ]; then
    if [ -d "/opt/homebrew/share/android-ndk" ]; then
        export ANDROID_NDK_HOME="/opt/homebrew/share/android-ndk"
    elif [ -d "$HOME/Library/Android/sdk/ndk/25.0.8775105" ]; then
        export ANDROID_NDK_HOME="$HOME/Library/Android/sdk/ndk/25.0.8775105"
    else
        echo "错误: 未找到 Android NDK。装法: brew install --cask android-ndk 或装 Android Studio SDK Manager" >&2
        exit 1
    fi
fi

# GOPROXY: 本机跑 Pilotty/sing-box 作 TUN 时, 默认 proxy.golang.org 会被 TUN 阻塞卡死。
# 见 CLAUDE.md #M9。
export GOPROXY="${GOPROXY:-https://goproxy.cn,https://goproxy.io,direct}"
export GOSUMDB="${GOSUMDB:-off}"

# 工具存在性检查
if ! command -v gomobile >/dev/null 2>&1; then
    cat >&2 <<'EOF'
错误: 未找到 gomobile。安装步骤:
  go install golang.org/x/mobile/cmd/gomobile@latest
  go install golang.org/x/mobile/cmd/gobind@latest
  gomobile init
EOF
    exit 1
fi
if ! command -v javac >/dev/null 2>&1; then
    echo "错误: javac 未在 PATH; 检查 JAVA_HOME=$JAVA_HOME" >&2
    exit 1
fi

mkdir -p "$(dirname "$OUT")"
cd "$REPO_ROOT"

echo "▶ 构建 $OUT"
echo "  Go:         $(go version)"
echo "  JDK:        $(javac -version 2>&1)"
echo "  NDK:        $ANDROID_NDK_HOME"
echo "  GOPROXY:    $GOPROXY"
echo "  源包:       ./libcore ./mobile + sing-box/experimental/libbox (直接暴露给 Kotlin)"
echo "  minSdk:     $ANDROID_API"

gomobile bind \
    -target=android \
    -androidapi "$ANDROID_API" \
    -trimpath \
    -ldflags='-s -w -checklinkname=0' \
    -tags='with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api,with_naive_outbound' \
    -o "$OUT" \
    ./libcore ./mobile \
    github.com/sagernet/sing-box/experimental/libbox

SIZE=$(du -h "$OUT" | awk '{print $1}')
echo "✓ 已生成: $OUT ($SIZE)"
