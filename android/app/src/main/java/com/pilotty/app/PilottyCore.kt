package com.pilotty.app

import android.content.Context
import android.util.Log
import com.pilotty.app.data.ChatStreamEvent
import com.pilotty.app.data.ToolEventDto
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import mobile.ChatStreamCallback
import mobile.Client
import mobile.Mobile
import mobile.VpnControlCallback

/**
 * Go core 单例。封装 gomobile 生成的 [mobile.Client]，
 * 上层只看到 Kotlin 友好的接口。所有方法都是同步阻塞，调用方负责切到 IO 线程。
 */
object PilottyCore {
    @Volatile private var client: Client? = null

    /** 在 Application.onCreate 中调用一次。 */
    fun init(ctx: Context, clashAPIAddr: String = "127.0.0.1:9090", apiKey: String = "") {
        if (client != null) return
        synchronized(this) {
            if (client != null) return
            client = Mobile.newClient(ctx.filesDir.absolutePath, clashAPIAddr, apiKey)
        }
    }

    fun shutdown() {
        client?.shutdown()
        client = null
    }

    private fun require(): Client = client ?: error("PilottyCore.init() not called")

    // ---- thin proxies (JSON 字符串原样返回，调用方解析) ----
    fun status(): String                          = require().status()
    fun nodes(): String                           = require().nodes()
    fun switchNode(group: String, node: String): String = require().switchNode(group, node)
    fun setMode(mode: String): String             = require().setMode(mode)
    fun testLatency(node: String): String         = require().testLatency(node)
    fun testLatencyAll(): String                  = require().testLatencyAll()
    fun chat(message: String): String             = require().chat(message)

    /**
     * Phase 7.3 流式 Chat。 每个 event 通过 Flow 涌出, 终态 [ChatStreamEvent.Done] 或
     * [ChatStreamEvent.Error]。 collect 时 Flow 会持续到 Done/Error 之一到达, 然后自动 close。
     *
     * 内部用 [callbackFlow] 包装 gomobile [ChatStreamCallback]: Go 侧在 goroutine 里跑
     * orchestrator.RunStream, OnEvent/OnDone/OnError 三路 callback 直接 trySend 到 channel。
     * awaitClose 时没有显式 cancel 路径 (Go 侧 RunStream 已发完事件就结束), 若 Flow 被上游
     * 取消, 仍旧让 goroutine 跑完但丢弃后续事件。 stop-mid-stream 留给 v1.x。
     */
    fun chatStream(message: String): Flow<ChatStreamEvent> = callbackFlow {
        val client = require()
        val cb = object : ChatStreamCallback {
            override fun onEvent(eventJSON: String) {
                runCatching { streamJson.decodeFromString(ChatStreamEvent.serializer(), eventJSON) }
                    .onSuccess { trySend(it) }
                    .onFailure { Log.w("PilottyCore", "chatStream: bad event json: $eventJSON", it) }
            }
            override fun onDone(finalJSON: String) {
                runCatching { streamJson.decodeFromString(ChatStreamDone.serializer(), finalJSON) }
                    .onSuccess {
                        trySend(ChatStreamEvent.Done(it.reply, it.source, it.events))
                        close()
                    }
                    .onFailure {
                        Log.w("PilottyCore", "chatStream: bad done json: $finalJSON", it)
                        trySend(ChatStreamEvent.Error("内部错误: final JSON 解析失败"))
                        close()
                    }
            }
            override fun onError(message: String) {
                trySend(ChatStreamEvent.Error(message))
                close()
            }
        }
        client.chatStream(message, cb)
        awaitClose { /* Go goroutine 自行结束;不主动 cancel */ }
    }

    private val streamJson = Json {
        ignoreUnknownKeys = true
        classDiscriminator = "type"
        coerceInputValues = true // Go 侧 Events 为空时 marshal 成 null, 这里 coerce 回 emptyList()
    }

    /** 仅用于解码 OnDone 的终态 payload。 形状与非流式 Chat 的 data 一致。 */
    @Serializable
    private data class ChatStreamDone(
        val reply: String = "",
        val source: String = "",
        val events: List<ToolEventDto> = emptyList(),
    )
    /** M8 热重载: 用户在 Settings 改 apiKey 后立即调这个, 无需重启 App。 空字符串 = 关 Agent。 */
    fun setApiKey(key: String): String            = require().setAPIKey(key)
    /** Phase 8: Kotlin 在 PilottyApp.onCreate 注入 VpnControlImpl, 让 Agent tool 能控制 VPN。 */
    fun setVpnControl(cb: VpnControlCallback)     = require().setVpnControl(cb)
    fun clearHistory()                            = require().clearHistory()
    fun subscriptions(): String                   = require().subscriptions()
    fun addSubscription(name: String, url: String): String = require().addSubscription(name, url)
    fun removeSubscription(id: String): String    = require().removeSubscription(id)
    fun updateAllSubscriptions(): String          = require().updateAllSubscriptions()
    fun importNodeURI(uri: String): String        = require().importNodeURI(uri)
    fun rules(): String                           = require().rules()
    fun addRule(ruleJSON: String): String         = require().addRule(ruleJSON)
    fun removeRule(tag: String): String           = require().removeRule(tag)
    fun templates(): String                       = require().templates()
    fun applyTemplate(id: String): String         = require().applyTemplate(id)
    fun ruleSets(): String                        = require().ruleSets()
    fun addRuleSet(setJSON: String): String       = require().addRuleSet(setJSON)
    fun removeRuleSet(tag: String): String        = require().removeRuleSet(tag)
    fun enableBuiltinRuleSet(alias: String): String = require().enableBuiltinRuleSet(alias)
    fun listBuiltinRuleSets(): String             = require().listBuiltinRuleSets()

    // Phase 9B 观测: ring buffer 已在 Go 侧就绪, Kotlin 侧按需轮询
    fun trafficHistory(n: Int): String            = require().trafficHistory(n.toLong())
    fun connections(): String                     = require().connections()
    fun recentLogs(n: Int): String                = require().recentLogs(n.toLong())
    fun clearTrafficHistory()                     = require().clearTrafficHistory()
    fun agentReady(): Boolean                     = require().agentReady()
    fun version(): String                         = require().version()

    // VPN 数据面 (3B-3 改为 Kotlin 直接调 libbox.CommandServer, 详见 PilottyVpnService.kt)。
    // Go 侧 libcore.BoxInstance 当前保留作为占位, 待 3B-5 精简时移除。
    // mobile/netpilot.go 的 StartTun / StopTun / SetPlatformInterface 保留签名但 Android 不再调用。
    //
    // #M13 修复: tunRunning 对外暴露为 StateFlow, 让 Compose ViewModel 可订阅;
    // 原来是 @Volatile Boolean, UI 只在 init/刷新时读一次 -> VpnService markTunRunning 后 UI 不刷
    private val _tunRunning = MutableStateFlow(false)
    val tunRunning: StateFlow<Boolean> = _tunRunning.asStateFlow()
    fun markTunRunning(running: Boolean) { _tunRunning.value = running }

    // 故障自动切换
    fun startFailover(configJSON: String = ""): String = require().startFailover(configJSON)
    fun stopFailover(): String                         = require().stopFailover()
    fun failoverStatus(): String                       = require().failoverStatus()

    // 配置备份/导入
    fun exportBackup(): String                         = require().exportBackup()
    fun importBackup(data: String): String             = require().importBackup(data)

    // 快照 / 手动回滚 (D2 Safety card)
    fun snapshots(): String                            = require().snapshots()
    fun rollback(id: String = ""): String              = require().rollback(id)
}
