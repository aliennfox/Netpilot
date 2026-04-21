package com.foxnetpilot.netpilot

import android.content.Context
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import mobile.Client
import mobile.Mobile

/**
 * Go core 单例。封装 gomobile 生成的 [mobile.Client]，
 * 上层只看到 Kotlin 友好的接口。所有方法都是同步阻塞，调用方负责切到 IO 线程。
 */
object NetPilotCore {
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

    private fun require(): Client = client ?: error("NetPilotCore.init() not called")

    // ---- thin proxies (JSON 字符串原样返回，调用方解析) ----
    fun status(): String                          = require().status()
    fun nodes(): String                           = require().nodes()
    fun switchNode(group: String, node: String): String = require().switchNode(group, node)
    fun setMode(mode: String): String             = require().setMode(mode)
    fun testLatency(node: String): String         = require().testLatency(node)
    fun testLatencyAll(): String                  = require().testLatencyAll()
    fun chat(message: String): String             = require().chat(message)
    fun clearHistory()                            = require().clearHistory()
    fun subscriptions(): String                   = require().subscriptions()
    fun addSubscription(name: String, url: String): String = require().addSubscription(name, url)
    fun removeSubscription(id: String): String    = require().removeSubscription(id)
    fun updateAllSubscriptions(): String          = require().updateAllSubscriptions()
    fun rules(): String                           = require().rules()
    fun templates(): String                       = require().templates()
    fun applyTemplate(id: String): String         = require().applyTemplate(id)
    fun agentReady(): Boolean                     = require().agentReady()
    fun version(): String                         = require().version()

    // VPN 数据面 (3B-3 改为 Kotlin 直接调 libbox.CommandServer, 详见 NetPilotVpnService.kt)。
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
}
