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

@Serializable
data class RouteRuleDto(
    val tag: String = "",
    @SerialName("domain_suffix") val domainSuffix: List<String> = emptyList(),
    val domain: List<String> = emptyList(),
    @SerialName("ip_cidr") val ipCidr: List<String> = emptyList(),
    @SerialName("process_name") val processName: List<String> = emptyList(),
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

@Serializable
data class SnapshotDto(
    val id: String = "",
    val timestamp: String = "",
    @SerialName("active_proxies") val activeProxies: Map<String, String> = emptyMap(),
)
