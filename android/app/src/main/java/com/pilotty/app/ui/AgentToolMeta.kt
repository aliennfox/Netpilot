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
    "set_dns_config" to R.string.tool_name_set_dns_config,
    // 原子 macro tool (Phase 2 + B 方向 #1/#2/#3): 一步完成多个底层 tool 的语义级原子操作
    "setup_app_chain" to R.string.tool_name_setup_app_chain,
    "switch_to_fastest_node" to R.string.tool_name_switch_to_fastest_node,
    "import_and_activate_subscription" to R.string.tool_name_import_and_activate_subscription,
    "diagnose_connectivity" to R.string.tool_name_diagnose_connectivity,
)

// atomicToolNames 标记哪些 tool 是 macro/原子 tool. UI 给它们额外视觉标识 (徽章 + 更大的 output preview),
// 因为它们的 Message 通常包含 [N/M] 步骤 trace + 综合结论, 比单步底层 tool 信息密度高.
private val atomicToolNames = setOf(
    "setup_app_chain",
    "switch_to_fastest_node",
    "import_and_activate_subscription",
    "diagnose_connectivity",
)

@Composable
fun friendlyToolName(rawName: String): String {
    val resId = toolNameResMap[rawName] ?: return rawName
    return stringResource(resId)
}

/** 这是不是 macro/原子 tool — UI 用它判断要不要加徽章 + 用更大 output preview 长度. */
fun isAtomicTool(rawName: String): Boolean = rawName in atomicToolNames
