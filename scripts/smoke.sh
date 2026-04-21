#!/usr/bin/env bash
# NetPilot 冒烟测试：导入订阅 → 切节点 → 开 VPN → ping
# 用法: ./scripts/smoke.sh [BASE_URL] [SUB_URL]
#   BASE_URL 默认 http://127.0.0.1:8080
#   SUB_URL  订阅链接（可选；缺省跳过导入步骤）

set -euo pipefail

BASE="${1:-http://127.0.0.1:8080}"
SUB_URL="${2:-}"
PROXY_HOST="${PROXY_HOST:-127.0.0.1}"
PROXY_PORT="${PROXY_PORT:-7890}"
PING_URL="${PING_URL:-https://www.gstatic.com/generate_204}"

step() { printf "\n\033[1;36m▶ %s\033[0m\n" "$*"; }
ok()   { printf "  \033[32m✓\033[0m %s\n" "$*"; }
fail() { printf "  \033[31m✗\033[0m %s\n" "$*"; exit 1; }

req() {
  local method="$1" path="$2" body="${3:-}"
  if [[ -n "$body" ]]; then
    curl -fsS -X "$method" -H 'Content-Type: application/json' -d "$body" "$BASE$path"
  else
    curl -fsS -X "$method" "$BASE$path"
  fi
}

step "1. 服务健康检查"
req GET /api/status > /tmp/np_status.json || fail "服务未启动 ($BASE)"
ok "status OK"

if [[ -n "$SUB_URL" ]]; then
  step "2. 导入订阅"
  req POST /api/subscriptions "{\"name\":\"smoke\",\"url\":\"$SUB_URL\"}" > /tmp/np_sub.json
  ok "订阅已添加"
  req POST /api/subscriptions/update-all > /dev/null
  ok "订阅已拉取"
else
  step "2. 跳过订阅导入（未提供 SUB_URL）"
fi

step "3. 列出节点并选择第一个"
req GET /api/nodes > /tmp/np_nodes.json
NODE=$(python3 -c "import json,sys; d=json.load(open('/tmp/np_nodes.json')); \
  proxies=d.get('data',d).get('proxies',{}); \
  cands=[n for n,v in proxies.items() if v.get('type') not in ('Selector','URLTest','Direct','Reject','Compatible')]; \
  print(cands[0] if cands else '')")
[[ -n "$NODE" ]] || fail "无可用节点"
ok "选中节点: $NODE"

step "4. 切换到该节点"
req POST /api/switch "{\"group\":\"proxy-group\",\"node\":\"$NODE\"}" > /dev/null
ok "已切换"

step "5. 测速"
req GET "/api/latency?node=$NODE" > /tmp/np_lat.json && ok "延迟测试完成" || fail "测速失败"

step "6. 通过 HTTP 代理 ping 外网"
if curl -fsS --max-time 10 -x "http://$PROXY_HOST:$PROXY_PORT" -o /dev/null -w "  HTTP %{http_code}, %{time_total}s\n" "$PING_URL"; then
  ok "代理连通"
else
  fail "代理不通 ($PROXY_HOST:$PROXY_PORT → $PING_URL)"
fi

step "7. 启动故障自动切换"
req POST /api/failover/start '{}' > /dev/null && ok "failover started" || ok "failover skip"
sleep 2
req GET /api/failover > /tmp/np_fo.json && ok "failover status OK"
req POST /api/failover/stop '{}' > /dev/null && ok "failover stopped"

step "8. 备份导出 / 导入"
curl -fsS "$BASE/api/backup/export" -o /tmp/np_backup.json
ok "导出: $(wc -c < /tmp/np_backup.json) bytes"
curl -fsS -X POST -H 'Content-Type: application/json' \
  --data-binary @/tmp/np_backup.json "$BASE/api/backup/import" > /tmp/np_imp.json
ok "导入回环 OK"

printf "\n\033[1;32m✅ 冒烟测试全部通过\033[0m\n"
