package com.pilotty.app.data

import com.pilotty.app.PilottyCore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.serializer

/**
 * Kotlin 侧的统一入口：
 *  - 切到 IO 线程调 [PilottyCore]（gomobile 阻塞调用）
 *  - 解析 JSON envelope
 *  - 失败抛 [PilottyException]，调用方用 try/catch 或 Result.runCatching 处理
 */
object PilottyRepository {
    private val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
    }

    class PilottyException(message: String) : RuntimeException(message)

    private suspend inline fun <reified T> call(crossinline block: () -> String): T = withContext(Dispatchers.IO) {
        val raw = block()
        val env = json.decodeFromString(Envelope.serializer(), raw)
        if (!env.success) throw PilottyException(env.error ?: "unknown error")
        val data = env.data ?: throw PilottyException("empty data")
        json.decodeFromJsonElement(serializer<T>(), data)
    }

    private suspend inline fun callRaw(crossinline block: () -> String) = withContext(Dispatchers.IO) {
        val env = json.decodeFromString(Envelope.serializer(), block())
        if (!env.success) throw PilottyException(env.error ?: "unknown error")
    }

    suspend fun status(): StatusDto = call { PilottyCore.status() }
    suspend fun nodes(): List<NodeDto> = call { PilottyCore.nodes() }
    suspend fun switchNode(group: String, node: String): MessageDto = call { PilottyCore.switchNode(group, node) }
    suspend fun setMode(mode: String): MessageDto = call { PilottyCore.setMode(mode) }
    suspend fun testLatency(node: String): String = withContext(Dispatchers.IO) { PilottyCore.testLatency(node) }
    suspend fun testLatencyAll(): String = withContext(Dispatchers.IO) { PilottyCore.testLatencyAll() }
    suspend fun chat(message: String): ChatDto = call { PilottyCore.chat(message) }

    /**
     * Phase 7.3 流式 Chat。 Flow 涌出多个事件, 最后以 [ChatStreamEvent.Done] 或
     * [ChatStreamEvent.Error] 终结。 collect 在 IO 线程, viewmodel 切回 Main 更新 state。
     */
    fun chatStream(message: String): Flow<ChatStreamEvent> =
        PilottyCore.chatStream(message).flowOn(Dispatchers.IO)
    suspend fun subscriptions(): List<SubscriptionDto> = call { PilottyCore.subscriptions() }
    suspend fun addSubscription(name: String, url: String): MessageDto = call { PilottyCore.addSubscription(name, url) }
    suspend fun removeSubscription(id: String): MessageDto = call { PilottyCore.removeSubscription(id) }
    suspend fun updateAllSubscriptions(): MessageDto = call { PilottyCore.updateAllSubscriptions() }
    suspend fun importNodeURI(uri: String): MessageDto = call { PilottyCore.importNodeURI(uri) }
    suspend fun importSubscriptionFromData(name: String, data: ByteArray): MessageDto = call {
        val b64 = android.util.Base64.encodeToString(data, android.util.Base64.NO_WRAP)
        PilottyCore.importSubscriptionFromData(name, b64)
    }
    suspend fun applyTemplate(id: String): MessageDto = call { PilottyCore.applyTemplate(id) }
    suspend fun rules(): List<RouteRuleDto> = call { PilottyCore.rules() }
    suspend fun addRule(ruleJSON: String): MessageDto = call { PilottyCore.addRule(ruleJSON) }
    suspend fun removeRule(tag: String): MessageDto = call { PilottyCore.removeRule(tag) }
    suspend fun templates(): List<TemplateDto> = call { PilottyCore.templates() }

    // Phase 10-A2 rule-set (geoip / geosite / custom remote-or-local)
    suspend fun ruleSets(): List<RuleSetConfigDto> = call { PilottyCore.ruleSets() }
    suspend fun addRuleSet(setJSON: String): MessageDto = call { PilottyCore.addRuleSet(setJSON) }
    suspend fun removeRuleSet(tag: String): MessageDto = call { PilottyCore.removeRuleSet(tag) }
    suspend fun enableBuiltinRuleSet(alias: String): MessageDto = call { PilottyCore.enableBuiltinRuleSet(alias) }
    suspend fun listBuiltinRuleSets(): List<RuleSetConfigDto> = call { PilottyCore.listBuiltinRuleSets() }

    // Phase 9B 观测: TrafficHistory / Connections / RecentLogs 不走 envelope, 直接是 JSON 数组
    // (Go 侧 mobile.TrafficHistory 等返回的就是 []{...}, 不包一层 Envelope)
    suspend fun trafficHistory(n: Int = 0): List<TrafficSampleDto> = withContext(Dispatchers.IO) {
        json.decodeFromString(serializer(), PilottyCore.trafficHistory(n))
    }
    suspend fun connections(): List<ConnectionDto> = withContext(Dispatchers.IO) {
        // Connections() 走的是 okJSON(envelope), 要剥壳
        val env = json.decodeFromString(Envelope.serializer(), PilottyCore.connections())
        if (!env.success) throw PilottyException(env.error ?: "connections fail")
        val data = env.data ?: return@withContext emptyList()
        json.decodeFromJsonElement(serializer(), data)
    }
    suspend fun recentLogs(n: Int = 0): List<LogEntryDto> = withContext(Dispatchers.IO) {
        json.decodeFromString(serializer(), PilottyCore.recentLogs(n))
    }
    fun clearTrafficHistory() = PilottyCore.clearTrafficHistory()

    suspend fun snapshots(): List<SnapshotDto> = call { PilottyCore.snapshots() }
    suspend fun rollback(id: String = ""): MessageDto = call { PilottyCore.rollback(id) }

    fun clearHistory() = PilottyCore.clearHistory()
    fun agentReady(): Boolean = PilottyCore.agentReady()
    fun version(): String = PilottyCore.version()

    // Phase 10-E-D: 配置备份 / 恢复
    // exportBackup 返回 JSON 原文(非 Envelope data 里面),直接落文件即可
    suspend fun exportBackupRaw(): String = withContext(Dispatchers.IO) {
        val raw = PilottyCore.exportBackup()
        val env = json.decodeFromString(Envelope.serializer(), raw)
        if (!env.success) throw PilottyException(env.error ?: "export 失败")
        // data 是嵌套的 JsonElement, 这里序列化成可读 JSON 字符串
        json.encodeToString(
            kotlinx.serialization.json.JsonElement.serializer(),
            env.data ?: throw PilottyException("empty backup"),
        )
    }

    suspend fun importBackup(data: String): MessageDto = call { PilottyCore.importBackup(data) }

    // Phase 10-E-E WebDAV 同步
    suspend fun webDAVTest(url: String, user: String, pass: String): MessageDto = call {
        PilottyCore.webDAVTest(url, user, pass)
    }
    suspend fun webDAVPush(url: String, user: String, pass: String, path: String): MessageDto = call {
        PilottyCore.webDAVPush(url, user, pass, path)
    }
    suspend fun webDAVPull(url: String, user: String, pass: String, path: String): MessageDto = call {
        PilottyCore.webDAVPull(url, user, pass, path)
    }

    // Phase 10-F-1 Failover (后端 Phase 2.5 就做完, UI 零入口, 本轮补齐)
    suspend fun startFailover(configJSON: String = ""): FailoverStatusDto = call {
        PilottyCore.startFailover(configJSON)
    }
    suspend fun stopFailover(): FailoverStatusDto = call { PilottyCore.stopFailover() }
    suspend fun failoverStatus(): FailoverStatusDto = call { PilottyCore.failoverStatus() }
}
