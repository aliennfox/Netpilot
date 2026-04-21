#!/bin/bash
# 真实节点端到端测试脚本
# 前置条件: sing-box 已启动 (clash API on 127.0.0.1:9090)
#
# 用法:
#   export SILICONFLOW_API_KEY=sk-xxx
#   ./scripts/test-real-nodes.sh

set -e

cd "$(dirname "$0")/.."

echo "=== Pilotty 真实节点测试 ==="
echo ""

# 编译
echo "编译..."
go build -o netpilot ./cmd/netpilot/
echo "编译成功。"
echo ""

SUB1="https://a.xn--gwt061a.com/pingmin?token=7c9d41a9ffc3abecea8ae7bd5ca5a791"
SUB2="http://47.242.55.240/link/vWjArsq38Vc7Ofwc?list=shadowrocket"

# 自动化测试序列
cat <<EOF | ./netpilot
import ${SUB1}
nodes
测速
status
quit
EOF

echo ""
echo "=== 测试完成 ==="
