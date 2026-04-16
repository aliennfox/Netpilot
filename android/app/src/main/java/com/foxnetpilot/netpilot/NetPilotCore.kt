package com.foxnetpilot.netpilot

import android.content.Context
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

    // VPN data plane (TUN fd handover)
    fun startTun(fd: Int, configJSON: String)     = require().startTun(fd, configJSON)
    fun stopTun()                                 = require().stopTun()
    fun tunRunning(): Boolean                     = require().tunRunning()

    // 故障自动切换
    fun startFailover(configJSON: String = ""): String = require().startFailover(configJSON)
    fun stopFailover(): String                         = require().stopFailover()
    fun failoverStatus(): String                       = require().failoverStatus()

    // 配置备份/导入
    fun exportBackup(): String                         = require().exportBackup()
    fun importBackup(data: String): String             = require().importBackup(data)
}
