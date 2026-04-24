#!/usr/bin/env bash
# M13 订阅 × 协议回归矩阵
#
# 主测试: Go 表驱动 parser→converter→marshal 全链
# 可选: 用 sing-box 二进制 `check` 再验一次产出的 outbound(需 binary 版本
#       与 go.mod 里的 sing-box 对齐, 否则新协议会被旧二进制误判)
#
# 用法:
#   scripts/subscription-matrix-test.sh          # 只跑 Go 测
#   scripts/subscription-matrix-test.sh --deep   # 同时跑 sing-box binary 验证
#   scripts/subscription-matrix-test.sh -v       # Go 测 verbose

set -euo pipefail

cd "$(dirname "$0")/.."

# GOPROXY hack 避开 #M9 TUN 网关卡死
export GOPROXY="${GOPROXY:-https://goproxy.cn,https://goproxy.io,direct}"
export GOSUMDB="${GOSUMDB:-off}"

VERBOSE=""
DEEP=0
for arg in "$@"; do
  case "$arg" in
    -v|--verbose) VERBOSE="-v" ;;
    --deep)       DEEP=1 ;;
  esac
done

echo "==> [Go] 跑 parser → converter → marshal 全链单测"
go test ./internal/subscription/... $VERBOSE -run 'TestSubscriptionFixtures|TestParse|TestConvert|TestIsClashYAML'
echo ""

if (( DEEP == 0 )); then
  echo "✅ Go 测通过。 要同时跑 sing-box binary 深度验证, 加 --deep"
  exit 0
fi

if ! command -v sing-box >/dev/null 2>&1; then
  echo "ℹ️  --deep 模式但未装 sing-box 二进制, 跳过。brew install sing-box"
  exit 0
fi

SINGBOX_VERSION="$(sing-box version 2>&1 | head -1)"
GO_MOD_VERSION="$(grep 'github.com/sagernet/sing-box v' go.mod | awk '{print $2}')"
echo "==> [bin] 用 ${SINGBOX_VERSION} 验证 fixture 产出"
echo "     (go.mod: ${GO_MOD_VERSION} —— 二进制和 module 版本不一致时可能产生误报)"

# 临时目录,留个 main.go 让 `go run` 能走当前 module 解析依赖
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

DUMP="cmd/dev-subdump"
mkdir -p "$DUMP"
cat > "$DUMP/main.go" <<'GOEOF'
// Build-only dev tool: 把 fixture 过一遍 ParseSubscription + ConvertToSingboxOutbound,
// 输出最小 sing-box config JSON。 scripts/subscription-matrix-test.sh 用。
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/foxnetpilot/netpilot/internal/subscription"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dev-subdump <fixture>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	nodes, err := subscription.ParseSubscription(string(data))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	outs := []map[string]interface{}{{"type": "direct", "tag": "direct"}}
	var eps []map[string]interface{}
	for _, n := range nodes {
		if n.IsInfoEntry {
			continue
		}
		ob, err := subscription.ConvertToSingboxOutbound(n)
		if err != nil || ob == nil {
			continue
		}
		// #M25: wireguard 是 endpoint 型, sing-box 1.13.8+ 不在 outbounds
		if t, _ := ob["type"].(string); t == "wireguard" {
			eps = append(eps, ob)
			continue
		}
		outs = append(outs, ob)
	}
	cfg := map[string]interface{}{
		"log":       map[string]interface{}{"level": "error"},
		"outbounds": outs,
	}
	if len(eps) > 0 {
		cfg["endpoints"] = eps
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	fmt.Print(string(b))
}
GOEOF

trap 'rm -rf "$TMP" "$DUMP"' EXIT

FAIL=0
for fixture in internal/subscription/testdata/fixtures/*.yaml internal/subscription/testdata/fixtures/*.json internal/subscription/testdata/fixtures/*.txt; do
  [[ -e "$fixture" ]] || continue
  name=$(basename "$fixture")
  if ! go run "./$DUMP" "$fixture" > "$TMP/out.json" 2>"$TMP/dumperr"; then
    echo "  ⚠️  $name: dump 失败"
    cat "$TMP/dumperr"
    FAIL=$((FAIL+1))
    continue
  fi
  if sing-box check -c "$TMP/out.json" 2>"$TMP/err"; then
    echo "  ✅ $name: sing-box check OK"
  else
    echo "  ❌ $name: sing-box check 失败"
    cat "$TMP/err"
    FAIL=$((FAIL+1))
  fi
done

if (( FAIL > 0 )); then
  echo ""
  echo "❌ 有 $FAIL 个 fixture 失败 sing-box binary validate"
  echo "   (若 binary 版本和 go.mod 不一致,新协议可能被 binary 识别为未知,这时"
  echo "    升级 binary 到 ${GO_MOD_VERSION} 再重试,或先忽略该告警)"
  exit 1
fi

echo ""
echo "✅ 订阅矩阵深度回归(含 sing-box binary validate)全部通过"
