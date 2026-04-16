package com.foxnetpilot.netpilot.vpn

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.util.Log
import androidx.core.app.NotificationCompat
import com.foxnetpilot.netpilot.MainActivity
import com.foxnetpilot.netpilot.NetPilotCore
import libbox.CommandServer
import libbox.CommandServerHandler
import libbox.Libbox
import libbox.OverrideOptions
import libbox.TunOptions
import java.io.File

/**
 * Android VpnService 数据面实现。
 *
 * 架构:
 *   VpnService(framework) + NetPilotPlatformInterface(libbox.PlatformInterface 默认实现)
 *   -> 同时满足 Android 框架 (TUN 建立权限) + sing-box libbox 回调契约 (openTun/protect)
 *
 * 启动流程 (3B-3 初版, 抄 sing-box-for-android bg/BoxService.kt:96-151):
 *   1. startForeground(前台通知)
 *   2. commandServer = Libbox.newCommandServer(this, this) —— handler=this, platformInterface=this
 *   3. commandServer.start()
 *   4. configJSON = NetPilotCore 合并后的 overlay config
 *   5. commandServer.startOrReloadService(configJSON, OverrideOptions())
 *      此时 sing-box 内部会反向调 openTun() 拿 fd, 我们在 openTun() 里 builder.establish()
 *
 * 停止:
 *   commandServer.closeService() -> commandServer.close() -> pfd.close() -> stopForeground
 */
class NetPilotVpnService : VpnService(), NetPilotPlatformInterface, CommandServerHandler {

    private var pfd: ParcelFileDescriptor? = null
    private var commandServer: CommandServer? = null

    // ──────────────── VpnService / NetPilotPlatformInterface ─────────────────

    /** libbox 启动 TUN inbound 时反向调用;此时 builder.establish() 并返回 fd。 */
    override fun openTun(options: TunOptions): Int {
        if (prepare(this) != null) error("VPN permission not granted")

        val builder = Builder()
            .setSession("NetPilot")
            .setMtu(options.mtu)

        // IPv4 地址
        runCatching {
            val inet4 = options.inet4Address
            while (inet4.hasNext()) {
                val addr = inet4.next()
                builder.addAddress(addr.address(), addr.prefix())
            }
        }.onFailure { Log.w(TAG, "inet4Address iterate", it) }

        // IPv6 地址
        runCatching {
            val inet6 = options.inet6Address
            while (inet6.hasNext()) {
                val addr = inet6.next()
                builder.addAddress(addr.address(), addr.prefix())
            }
        }.onFailure { Log.w(TAG, "inet6Address iterate", it) }

        // DNS + 默认路由 (autoRoute)
        if (options.autoRoute) {
            runCatching { builder.addDnsServer(options.dnsServerAddress.value) }
                .onFailure { builder.addDnsServer("1.1.1.1") }
            builder.addRoute("0.0.0.0", 0)
            builder.addRoute("::", 0)
        }

        // 排除 NetPilot 自身防止回环
        runCatching { builder.addDisallowedApplication(packageName) }

        val tun = builder.establish() ?: error("VpnService.Builder.establish() returned null")
        pfd = tun
        val fd = tun.detachFd()
        Log.i(TAG, "openTun established fd=$fd mtu=${options.mtu}")
        return fd
    }

    /** 对 outbound socket fd 调 VpnService.protect 防止回环;VpnService 独有能力。 */
    override fun autoDetectInterfaceControl(fd: Int) {
        protect(fd)
    }

    // ──────────────── CommandServerHandler (sing-box 回调业务侧, 5 个方法) ─────────────────

    override fun serviceReload() {
        Log.i(TAG, "CommandServer: serviceReload requested")
        // 3B-5 接入真 reload 流程
    }

    override fun serviceStop() {
        Log.i(TAG, "CommandServer: serviceStop requested")
        stopService()
    }

    override fun getSystemProxyStatus(): libbox.SystemProxyStatus = libbox.SystemProxyStatus()

    override fun setSystemProxyEnabled(enabled: Boolean) {}

    override fun writeDebugMessage(msg: String?) {
        msg?.let { Log.d(TAG, "libbox: $it") }
    }

    // ──────────────── Service 生命周期 ─────────────────

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> {
                stopService()
                stopSelf()
                return START_NOT_STICKY
            }
            else -> startService()
        }
        return START_STICKY
    }

    private fun startService() {
        startForeground(NOTIF_ID, buildNotification())
        try {
            // 读 overlay 合并后的 sing-box config JSON
            val configJson = loadConfigJson()
            Log.i(TAG, "config loaded ${configJson.length} bytes")

            // 启 CommandServer
            val server = Libbox.newCommandServer(this, this)
            server.start()
            commandServer = server
            Log.i(TAG, "CommandServer started")

            // 加载配置, 触发 openTun 回调建 TUN
            server.startOrReloadService(configJson, OverrideOptions())
            Log.i(TAG, "sing-box service started via libbox")
            NetPilotCore.markTunRunning(true)
        } catch (t: Throwable) {
            Log.e(TAG, "startService failed", t)
            stopService()
            stopSelf()
        }
    }

    private fun stopService() {
        runCatching { commandServer?.closeService() }.onFailure { Log.w(TAG, "closeService", it) }
        runCatching { commandServer?.close() }.onFailure { Log.w(TAG, "commandServer.close", it) }
        commandServer = null
        runCatching { pfd?.close() }
        pfd = null
        NetPilotCore.markTunRunning(false)
    }

    override fun onDestroy() {
        stopService()
        super.onDestroy()
    }

    override fun onRevoke() {
        stopService()
        super.onRevoke()
    }

    /**
     * 从 NetPilot 的 overlay 合并配置文件加载 JSON。
     * filesDir/configs/merged.json 或其他约定路径。3B-3 当前用简单 fallback 到 minimal.json。
     */
    private fun loadConfigJson(): String {
        val candidates = listOf(
            File(filesDir, "configs/merged.json"),
            File(filesDir, "configs/minimal.json"),
        )
        val f = candidates.firstOrNull { it.exists() }
            ?: error("no sing-box config found; expected ${candidates.joinToString()}")
        return f.readText()
    }

    private fun buildNotification(): Notification {
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val ch = NotificationChannel(CHANNEL_ID, "NetPilot VPN", NotificationManager.IMPORTANCE_LOW)
            nm.createNotificationChannel(ch)
        }
        val tapIntent = PendingIntent.getActivity(
            this, 0, Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("NetPilot")
            .setContentText("VPN 已连接")
            .setSmallIcon(android.R.drawable.ic_lock_lock)
            .setContentIntent(tapIntent)
            .setOngoing(true)
            .build()
    }

    companion object {
        private const val TAG = "NetPilotVpnService"
        private const val CHANNEL_ID = "netpilot_vpn"
        private const val NOTIF_ID = 1001
        const val ACTION_STOP = "com.foxnetpilot.netpilot.STOP_VPN"
    }
}
