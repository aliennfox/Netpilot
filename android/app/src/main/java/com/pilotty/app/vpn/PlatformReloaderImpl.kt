package com.pilotty.app.vpn

import android.util.Log
import com.pilotty.app.PilottyApp
import mobile.PlatformReloader

/**
 * D3 reload 通道. Agent 写完 overlay (Per-App / 路由规则等) 后, Go 侧通过本 callback
 * 通知 Kotlin 触发 [PilottyVpnService.requestReload], 让运行中的 sing-box 实例热加载
 * merged.json. VPN 未运行时 PilottyVpnService.reloadIfRunning 是 no-op, 不报错.
 *
 * 路径:
 *   LLM tool_call("set_per_app_vpn") → Go toolSetPerAppVpn → ov.SetPerAppVpn + ov.Apply
 *     → reloader.RequestReload() (Go reloaderAdapter)
 *     → PlatformReloader.requestReload() JNI
 *     → 本对象 → PilottyVpnService.requestReload(appContext)
 *     → onStartCommand ACTION_RELOAD → reloadIfRunning()
 *     → libbox CommandServer.startOrReloadService(merged.json, OverrideOptions())
 *
 * 注入位置: PilottyApp.onCreate, 紧跟 setVpnControl 之后.
 */
object PlatformReloaderImpl : PlatformReloader {
    private const val TAG = "PlatformReloader"

    override fun requestReload() {
        val ctx = PilottyApp.appContext ?: run {
            Log.w(TAG, "requestReload: appContext null, skipping")
            return
        }
        Log.i(TAG, "requestReload → PilottyVpnService.ACTION_RELOAD")
        runCatching { PilottyVpnService.requestReload(ctx) }
            .onFailure { Log.e(TAG, "requestReload failed", it) }
    }
}
