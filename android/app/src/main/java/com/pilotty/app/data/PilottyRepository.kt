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

    suspend fun snapshots(): List<SnapshotDto> = call { PilottyCore.snapshots() }
    suspend fun rollback(id: String = ""): MessageDto = call { PilottyCore.rollback(id) }

    fun clearHistory() = PilottyCore.clearHistory()
    fun agentReady(): Boolean = PilottyCore.agentReady()
    fun version(): String = PilottyCore.version()
}
