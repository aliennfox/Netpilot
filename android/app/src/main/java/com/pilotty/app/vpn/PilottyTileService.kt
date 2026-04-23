package com.pilotty.app.vpn

import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.service.quicksettings.Tile
import android.service.quicksettings.TileService
import androidx.annotation.RequiresApi
import com.pilotty.app.MainActivity
import com.pilotty.app.PilottyCore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch

/**
 * Quick Settings Tile: 下拉通知栏一键开关 VPN。
 *
 * 参考: ~/References/nekobox/app/src/main/java/io/nekohasekai/sagernet/bg/TileService.kt
 *
 * 设计决策:
 *  - Tile 状态镜像 [PilottyCore.tunRunning] StateFlow, 订阅期只在 onStartListening→onStopListening
 *    之间存活, 避免 Tile 不在视野时还 collect (Android Tile lifecycle 不保证调 onDestroy)
 *  - 启动路径: VpnService.prepare() 要 Activity 上下文才能弹系统授权框 → tile 无法自己做,
 *    必须 startActivityAndCollapse 到 [MainActivity] 带上 [EXTRA_START_VPN], 由 Activity 完成授权
 *  - 停止路径: 不需要 Activity, 直接 startService(ACTION_STOP) 即可
 *  - API 24+ (Nougat) 才有 QS Tile, Pilotty minSdk=21 时该类在 21-23 上不会被系统加载, 天然安全
 */
@RequiresApi(Build.VERSION_CODES.N)
class PilottyTileService : TileService() {

    private var watchScope: CoroutineScope? = null
    private var watchJob: Job? = null

    override fun onStartListening() {
        super.onStartListening()
        // tile 被用户下拉可见时才启动 collect, 节能
        val scope = CoroutineScope(Dispatchers.Main).also { watchScope = it }
        watchJob = scope.launch {
            PilottyCore.tunRunning.collectLatest { running ->
                syncTile(running)
            }
        }
    }

    override fun onStopListening() {
        watchJob?.cancel()
        watchScope?.cancel()
        watchScope = null
        super.onStopListening()
    }

    override fun onClick() {
        super.onClick()
        val running = PilottyCore.tunRunning.value
        if (running) {
            stopVpn()
        } else {
            launchStartFlow()
        }
    }

    private fun syncTile(running: Boolean) {
        val tile = qsTile ?: return
        tile.state = if (running) Tile.STATE_ACTIVE else Tile.STATE_INACTIVE
        tile.label = "Pilotty"
        // subtitle API 29+
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            tile.subtitle = if (running) "已连接" else "未连接"
        }
        tile.updateTile()
    }

    private fun stopVpn() {
        val intent = Intent(this, PilottyVpnService::class.java).apply {
            action = PilottyVpnService.ACTION_STOP
        }
        startService(intent)
    }

    /**
     * 启动 VPN:如果系统已给过 VpnService 授权, 直接 startForegroundService;
     * 否则必须通过 Activity 完成 [VpnService.prepare] 流程。
     *
     * Android 14+ startActivityAndCollapse 签名改成 PendingIntent, 老 API 仍用 Intent, 兼容分叉。
     */
    private fun launchStartFlow() {
        val needsPrepare = VpnService.prepare(this) != null
        if (!needsPrepare) {
            val intent = Intent(this, PilottyVpnService::class.java)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                startForegroundService(intent)
            } else {
                startService(intent)
            }
            // 乐观更新, StateFlow 会在 libbox 起来后刷
            syncTile(true)
            return
        }

        val activityIntent = Intent(this, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP
            action = ACTION_START_VPN_FROM_TILE
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            // API 34+: 必须用 PendingIntent
            val pi = PendingIntent.getActivity(
                this,
                0,
                activityIntent,
                PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
            )
            @Suppress("NewApi")
            startActivityAndCollapse(pi)
        } else {
            @Suppress("DEPRECATION")
            startActivityAndCollapse(activityIntent)
        }
    }

    companion object {
        /** MainActivity 收到这个 action 时,调用 VpnController.start() 完成 prepare() + 启动服务 */
        const val ACTION_START_VPN_FROM_TILE = "com.pilotty.app.START_VPN_FROM_TILE"
    }
}
