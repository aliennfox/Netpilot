package com.pilotty.app.data

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement

/**
 * 与 Go 端 mobile.response 对应的统一响应包络。
 * Go 写入 data 时是任意 JSON，因此用 JsonElement 推迟解析。
 */
@Serializable
data class Envelope(
    val success: Boolean = false,
    val data: JsonElement? = null,
    val error: String? = null,
)

@Serializable
data class StatusDto(
    @SerialName("current_node") val currentNode: String = "",
    val mode: String = "",
    @SerialName("node_count") val nodeCount: Int = 0,
    val connections: Int = 0,
    val upload: Long = 0,
    val download: Long = 0,
    @SerialName("agent_ready") val agentReady: Boolean = false,
)

@Serializable
data class NodeDto(
    val tag: String = "",
    val type: String = "",
    val server: String = "",
    val port: Int = 0,
    val alive: Boolean = false,
    val latency: Int = 0,
    @SerialName("group_tag") val groupTag: String = "",
    val active: Boolean = false,
)

@Serializable
data class ChatDto(
    val reply: String = "",
    val source: String = "",
    val events: List<ToolEventDto> = emptyList(),
)

/**
 * D3 Chat Tool-Call Timeline。 对应 Go 侧 [agent.ToolEvent], 记录一次 tool 调用的可观测细节。
 * source="local" 时 events 为空 (IntentRouter 不经 orchestrator)。
 */
@Serializable
data class ToolEventDto(
    val name: String = "",
    @SerialName("args_summary") val argsSummary: String = "",
    @SerialName("duration_ms") val durationMs: Long = 0,
    @SerialName("output_preview") val outputPreview: String = "",
    val error: String = "",
    /** Diagnose / Configure / Verify (多角色流水线阶段), 单角色快速路径时也会填 */
    val role: String = "",
)

@Serializable
data class SubscriptionDto(
    val id: String = "",
    val name: String = "",
    val url: String = "",
    @SerialName("node_count") val nodeCount: Int = 0,
    val tags: List<String> = emptyList(),
    @SerialName("auto_update") val autoUpdate: Boolean = false,
    @SerialName("interval_minutes") val intervalMinutes: Int = 0,
    @SerialName("user_info") val userInfo: UserInfoDto? = null,
)

@Serializable
data class UserInfoDto(
    val upload: Long = 0,
    val download: Long = 0,
    val total: Long = 0,
    val expire: Long = 0,
)

@Serializable
data class MessageDto(val message: String = "")

/**
 * Phase 7.3 流式 Chat 事件。 前 5 种 (TextDelta/PhaseStart/PhaseEnd/ToolStart/ToolEnd) 从
 * Go mobile.ChatStreamCallback.OnEvent(jsonStr) 解析, 按 "type" 字段走多态反序列化。
 *
 * Done/Error 是 Kotlin bridge 合成的终态 —— Go 走 OnDone/OnError 两个独立 callback 方法,
 * bridge 层把它们包成同一 sealed family 方便 Flow collect { when (it) { ... } } 一锅端。
 *
 * UI 映射:
 *  - TextDelta → 当前 assistant 气泡 text 追加
 *  - PhaseStart/PhaseEnd → 顶部 "[诊断中]"/"[配置中]"/"[验证中]" 指示器
 *  - ToolStart/ToolEnd → ToolCallTimeline 实时新增, "▸ running" → "✓ done"
 *  - Done → 气泡 finalize, sending=false, 持久化 messages
 *  - Error → 红色气泡
 */
@Serializable
sealed class ChatStreamEvent {
    @Serializable
    @SerialName("text_delta")
    data class TextDelta(val role: String = "", val delta: String = "") : ChatStreamEvent()

    @Serializable
    @SerialName("phase_start")
    data class PhaseStart(val role: String = "") : ChatStreamEvent()

    @Serializable
    @SerialName("phase_end")
    data class PhaseEnd(val role: String = "", val summary: String = "") : ChatStreamEvent()

    @Serializable
    @SerialName("tool_start")
    data class ToolStart(
        val name: String = "",
        @SerialName("args_summary") val argsSummary: String = "",
        val role: String = "",
    ) : ChatStreamEvent()

    @Serializable
    @SerialName("tool_end")
    data class ToolEnd(
        val name: String = "",
        @SerialName("args_summary") val argsSummary: String = "",
        @SerialName("duration_ms") val durationMs: Long = 0,
        @SerialName("output_preview") val outputPreview: String = "",
        val error: String = "",
        val role: String = "",
    ) : ChatStreamEvent()

    /** 终态, 合成自 Go OnDone(finalJSON) */
    data class Done(val reply: String, val source: String, val events: List<ToolEventDto>) : ChatStreamEvent()

    /** 终态, 合成自 Go OnError(msg) */
    data class Error(val message: String) : ChatStreamEvent()
}

@Serializable
data class RouteRuleDto(
    val tag: String = "",
    @SerialName("domain_suffix") val domainSuffix: List<String> = emptyList(),
    val domain: List<String> = emptyList(),
    @SerialName("domain_keyword") val domainKeyword: List<String> = emptyList(),
    @SerialName("domain_regex") val domainRegex: List<String> = emptyList(),
    @SerialName("ip_cidr") val ipCidr: List<String> = emptyList(),
    @SerialName("process_name") val processName: List<String> = emptyList(),
    val port: List<Int> = emptyList(),
    @SerialName("port_range") val portRange: List<String> = emptyList(),
    val network: List<String> = emptyList(),
    val protocol: List<String> = emptyList(),
    @SerialName("rule_set") val ruleSet: List<String> = emptyList(),
    val geoip: List<String> = emptyList(),
    val geosite: List<String> = emptyList(),
    val outbound: String = "",
    val description: String = "",
    val source: String = "",
)

@Serializable
data class TemplateDto(
    val id: String = "",
    val name: String = "",
    val description: String = "",
    val keywords: List<String> = emptyList(),
)

// Phase 10-A2 rule-set 声明, 对应 Go overlay.RuleSetConfig
@Serializable
data class RuleSetConfigDto(
    val tag: String = "",
    val type: String = "", // "remote" | "local"
    val format: String = "",
    val url: String = "",
    val path: String = "",
    @SerialName("download_detour") val downloadDetour: String = "",
    @SerialName("update_interval") val updateInterval: String = "",
    val source: String = "",
)

@Serializable
data class SnapshotDto(
    val id: String = "",
    val timestamp: String = "",
    @SerialName("active_proxies") val activeProxies: Map<String, String> = emptyMap(),
)
