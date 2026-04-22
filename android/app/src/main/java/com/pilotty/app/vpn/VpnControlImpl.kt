package com.pilotty.app.vpn

import android.util.Log
import com.pilotty.app.PilottyCore
import mobile.VpnControlCallback

/**
 * Phase 8: Agent 的 start_vpn / stop_vpn / vpn_status tools 通过 gomobile reverse-binding
 * 反向调用 Kotlin 侧这个单例。 代替 Phase 7 的 ChatViewModel 关键词拦截 —— 让 Agent 真正
 * 掌控 VPN 生命周期, 而不是在 App 层打补丁。
 *
 * 路径:
 *   LLM tool_call("start_vpn")
 *     → Go tool.toolStartVpn
 *     → clientVpnAdapter.RequestStart
 *     → VpnControlCallback.requestStart()  (JNI)
 *     → VpnControlImpl.requestStart
 *     → StartVpnBus.request()
 *     → MainActivity.LaunchedEffect { StartVpnBus.requests.collect { onStartVpn() } }
 *     → VpnController.start()
 *
 * 注入位置: PilottyApp.onCreate, PilottyCore.init 之后。
 */
object VpnControlImpl : VpnControlCallback {
    private const val TAG = "VpnControlImpl"

    override fun requestStart() {
        Log.i(TAG, "requestStart → StartVpnBus")
        StartVpnBus.request()
    }

    override fun requestStop() {
        Log.i(TAG, "requestStop → StopVpnBus")
        StopVpnBus.request()
    }

    override fun isRunning(): Boolean = PilottyCore.tunRunning.value
}
