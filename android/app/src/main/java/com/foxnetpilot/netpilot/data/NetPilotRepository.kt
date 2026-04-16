package com.foxnetpilot.netpilot.data

import com.foxnetpilot.netpilot.NetPilotCore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.serializer

/**
 * Kotlin 侧的统一入口：
 *  - 切到 IO 线程调 [NetPilotCore]（gomobile 阻塞调用）
 *  - 解析 JSON envelope
 *  - 失败抛 [NetPilotException]，调用方用 try/catch 或 Result.runCatching 处理
 */
object NetPilotRepository {
    private val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
    }

    class NetPilotException(message: String) : RuntimeException(message)

    private suspend inline fun <reified T> call(crossinline block: () -> String): T = withContext(Dispatchers.IO) {
        val raw = block()
        val env = json.decodeFromString(Envelope.serializer(), raw)
        if (!env.success) throw NetPilotException(env.error ?: "unknown error")
        val data = env.data ?: throw NetPilotException("empty data")
        json.decodeFromJsonElement(serializer<T>(), data)
    }

    private suspend inline fun callRaw(crossinline block: () -> String) = withContext(Dispatchers.IO) {
        val env = json.decodeFromString(Envelope.serializer(), block())
        if (!env.success) throw NetPilotException(env.error ?: "unknown error")
    }

    suspend fun status(): StatusDto = call { NetPilotCore.status() }
    suspend fun nodes(): List<NodeDto> = call { NetPilotCore.nodes() }
    suspend fun switchNode(group: String, node: String): MessageDto = call { NetPilotCore.switchNode(group, node) }
    suspend fun setMode(mode: String): MessageDto = call { NetPilotCore.setMode(mode) }
    suspend fun testLatency(node: String): String = withContext(Dispatchers.IO) { NetPilotCore.testLatency(node) }
    suspend fun testLatencyAll(): String = withContext(Dispatchers.IO) { NetPilotCore.testLatencyAll() }
    suspend fun chat(message: String): ChatDto = call { NetPilotCore.chat(message) }
    suspend fun subscriptions(): List<SubscriptionDto> = call { NetPilotCore.subscriptions() }
    suspend fun addSubscription(name: String, url: String): MessageDto = call { NetPilotCore.addSubscription(name, url) }
    suspend fun removeSubscription(id: String): MessageDto = call { NetPilotCore.removeSubscription(id) }
    suspend fun updateAllSubscriptions(): MessageDto = call { NetPilotCore.updateAllSubscriptions() }
    suspend fun applyTemplate(id: String): MessageDto = call { NetPilotCore.applyTemplate(id) }
    suspend fun rules(): List<RouteRuleDto> = call { NetPilotCore.rules() }
    suspend fun templates(): List<TemplateDto> = call { NetPilotCore.templates() }

    fun clearHistory() = NetPilotCore.clearHistory()
    fun agentReady(): Boolean = NetPilotCore.agentReady()
    fun version(): String = NetPilotCore.version()
}
