package com.pilotty.app.ui

import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import com.pilotty.app.R

// D3 Action Trace UI: 把 Go 侧 raw tool 名映射到本地化友好显示名.
// 找不到映射回退原 raw name (可读但英文), 不抛异常.
private val toolNameResMap = mapOf(
    "set_per_app_vpn" to R.string.tool_name_set_per_app_vpn,
    "get_per_app_vpn" to R.string.tool_name_get_per_app_vpn,
    "start_vpn" to R.string.tool_name_start_vpn,
    "stop_vpn" to R.string.tool_name_stop_vpn,
    "vpn_status" to R.string.tool_name_vpn_status,
    "switch_node" to R.string.tool_name_switch_node,
    "set_mode" to R.string.tool_name_set_mode,
    "test_latency" to R.string.tool_name_test_latency,
    "test_latency_all" to R.string.tool_name_test_latency_all,
    "get_node_pool" to R.string.tool_name_get_node_pool,
    "get_connections" to R.string.tool_name_get_connections,
    "get_logs" to R.string.tool_name_get_logs,
    "list_route_rules" to R.string.tool_name_list_route_rules,
    "patch_route_rule" to R.string.tool_name_patch_route_rule,
    "remove_route_rule" to R.string.tool_name_remove_route_rule,
    "create_chain" to R.string.tool_name_create_chain,
    "import_subscription" to R.string.tool_name_import_subscription,
    "list_subscriptions" to R.string.tool_name_list_subscriptions,
    "update_subscription" to R.string.tool_name_update_subscription,
    "remove_subscription" to R.string.tool_name_remove_subscription,
    "get_dns_config" to R.string.tool_name_get_dns_config,
)

@Composable
fun friendlyToolName(rawName: String): String {
    val resId = toolNameResMap[rawName] ?: return rawName
    return stringResource(resId)
}
