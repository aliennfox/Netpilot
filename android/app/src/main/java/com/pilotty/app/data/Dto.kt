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

// Phase 9B 观测 DTO
@Serializable
data class TrafficSampleDto(
    val t: Long = 0,        // Unix epoch seconds
    val up: Long = 0,       // 累计上传
    val down: Long = 0,     // 累计下载
    @SerialName("up_rate") val upRate: Long = 0,     // 当秒增量
    @SerialName("down_rate") val downRate: Long = 0, // 当秒增量
)

@Serializable
data class ConnectionDto(
    val id: String = "",
    val destination: String = "",
    val protocol: String = "",
    @SerialName("process_name") val processName: String = "",
    val upload: Long = 0,
    val download: Long = 0,
    @SerialName("start_time") val startTime: String = "",
    @SerialName("duration_ms") val durationMs: Long = 0,
    val chain: List<String> = emptyList(),
    val rule: String = "",
)

@Serializable
data class LogEntryDto(
    val ts: Long = 0,            // Unix epoch milliseconds
    val level: String = "",      // "I" / "W" / "E" / "D"
    val msg: String = "",
)

// Phase P1-C 自定义 DNS / DoH / DoQ
@Serializable
data class DNSServerDto(
    val type: String = "", // "udp" | "tls" | "https" | "quic" | "h3" | "local"
    val tag: String = "",
    val server: String = "",
    @SerialName("server_port") val serverPort: Int = 0,
    val detour: String = "",
)

@Serializable
data class DNSRuleDto(
    val action: String = "", // "route" | "reject"
    val server: String = "",
    val outbound: String = "",
    val domain: List<String> = emptyList(),
    @SerialName("domain_suffix") val domainSuffix: List<String> = emptyList(),
)

@Serializable
data class DNSConfigDto(
    val servers: List<DNSServerDto> = emptyList(),
    val rules: List<DNSRuleDto> = emptyList(),
    val final: String = "",
    val strategy: String = "",
)

// Phase P1-B Net Check 自检
@Serializable
data class NetCheckLineDto(
    val section: String = "",
    val level: String = "", // "ok" | "warn" | "fail"
    val msg: String = "",
    val detail: String = "",
)

@Serializable
data class NetCheckReportDto(
    val ts: Long = 0,
    val lines: List<NetCheckLineDto> = emptyList(),
    @SerialName("elapsed_ms") val elapsedMs: Long = 0,
)

// Phase 10-F-1 Failover
@Serializable
data class FailoverStatusDto(
    val running: Boolean = false,
    val group: String = "",
    @SerialName("current_node") val currentNode: String = "",
    @SerialName("consecutive_fails") val consecutiveFails: Int = 0,
    @SerialName("last_switch_at") val lastSwitchAt: String = "",
    @SerialName("last_switch_to") val lastSwitchTo: String = "",
    @SerialName("last_error") val lastError: String = "",
)

@Serializable
data class SnapshotDto(
    val id: String = "",
    val timestamp: String = "",
    @SerialName("active_proxies") val activeProxies: Map<String, String> = emptyMap(),
)
