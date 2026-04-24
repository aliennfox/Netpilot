package com.pilotty.app.vpn

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.ProxyInfo
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.util.Log
import androidx.core.app.NotificationCompat
import com.pilotty.app.MainActivity
import com.pilotty.app.PilottyApp
import com.pilotty.app.PilottyCore
import com.pilotty.app.perapp.PerAppVpnPrefs
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
 *   VpnService(framework) + PilottyPlatformInterface(libbox.PlatformInterface 默认实现)
 *   -> 同时满足 Android 框架 (TUN 建立权限) + sing-box libbox 回调契约 (openTun/protect)
 *
 * 启动流程 (抄 sing-box-for-android BoxService.kt):
 *   1. startForeground(前台通知)
 *   2. 加载并校验 sing-box config JSON
 *   3. Libbox.newCommandServer(this, this) —— handler=this, platformInterface=this
 *   4. commandServer.start()
 *   5. commandServer.startOrReloadService(configJSON, OverrideOptions())
 *      此时 sing-box 内部会反向调 openTun() 拿 fd
 *
 * VPN 授权由 MainActivity/VpnController 在启动 Service 前通过
 * VpnService.prepare() + ActivityResultLauncher 处理, Service 本身不弹框。
 */
class PilottyVpnService : VpnService(), PilottyPlatformInterface, CommandServerHandler {

    private var pfd: ParcelFileDescriptor? = null
    private var commandServer: CommandServer? = null

    // ──────────────── PilottyPlatformInterface (libbox 反向回调) ─────────────────

    /** libbox 启动 TUN inbound 时反向调用;此时 builder.establish() 并返回 fd。 */
    override fun openTun(options: TunOptions): Int {
        val builder = Builder()
            .setSession("Pilotty")
            .setMtu(options.mtu)
            .setConfigureIntent(buildConfigureIntent())

        // 地址
        iterateRoute(options.inet4Address) { addr, prefix -> builder.addAddress(addr, prefix) }
        iterateRoute(options.inet6Address) { addr, prefix -> builder.addAddress(addr, prefix) }

        // 路由: 优先使用 sing-box 计算出的显式前缀列表; 若为空且 autoRoute=true, 回退 0.0.0.0/0 + ::/0
        val v4Added = iterateRoute(options.inet4RouteAddress) { addr, prefix -> builder.addRoute(addr, prefix) }
        val v6Added = iterateRoute(options.inet6RouteAddress) { addr, prefix -> builder.addRoute(addr, prefix) }
        if (options.autoRoute && v4Added == 0 && v6Added == 0) {
            builder.addRoute("0.0.0.0", 0)
            builder.addRoute("::", 0)
        }

        // 排除路由 (Android Q+, sing-box 用其避开特定 CIDR 比如 DNS 冲突)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            iterateRoute(options.inet4RouteExcludeAddress) { addr, prefix ->
                runCatching { builder.excludeRoute(android.net.IpPrefix(java.net.InetAddress.getByName(addr), prefix)) }
                    .onFailure { Log.w(TAG, "excludeRoute v4 $addr/$prefix", it) }
            }
            iterateRoute(options.inet6RouteExcludeAddress) { addr, prefix ->
                runCatching { builder.excludeRoute(android.net.IpPrefix(java.net.InetAddress.getByName(addr), prefix)) }
                    .onFailure { Log.w(TAG, "excludeRoute v6 $addr/$prefix", it) }
            }
        }

        // DNS (StringBox.getValue 有可能 throw)
        if (options.autoRoute) {
            runCatching { builder.addDnsServer(options.dnsServerAddress.value) }
                .onFailure {
                    Log.w(TAG, "dnsServerAddress fallback to 1.1.1.1", it)
                    builder.addDnsServer("1.1.1.1")
                }
        }

        // Per-app VPN
        val includeCount = iterateStrings(options.includePackage) { pkg ->
            runCatching { builder.addAllowedApplication(pkg) }
                .onFailure { Log.w(TAG, "addAllowedApplication $pkg failed", it) }
        }
        if (includeCount == 0) {
            // 没有白名单时走黑名单; Pilotty 自身必须排除, 否则 TUN 流量回环
            runCatching { builder.addDisallowedApplication(packageName) }
            iterateStrings(options.excludePackage) { pkg ->
                runCatching { builder.addDisallowedApplication(pkg) }
                    .onFailure { Log.w(TAG, "addDisallowedApplication $pkg failed", it) }
            }
        }

        // HTTP Proxy (Q+)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q && options.isHTTPProxyEnabled) {
            runCatching {
                val proxy = ProxyInfo.buildDirectProxy(
                    options.httpProxyServer,
                    options.httpProxyServerPort,
                )
                builder.setHttpProxy(proxy)
            }.onFailure { Log.w(TAG, "setHttpProxy failed", it) }
        }

        val tun = builder.establish() ?: error("VpnService.Builder.establish() returned null")
        pfd = tun
        val fd = tun.detachFd()
        Log.i(TAG, "openTun established fd=$fd mtu=${options.mtu} v4Routes=$v4Added v6Routes=$v6Added")
        return fd
    }

    /** 对 outbound socket fd 调 VpnService.protect 防止回环;VpnService 独有能力。 */
    override fun autoDetectInterfaceControl(fd: Int) {
        protect(fd)
    }

    // ──────────────── CommandServerHandler ─────────────────

    override fun serviceReload() {
        Log.i(TAG, "CommandServer: serviceReload requested")
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
        Log.i(TAG, "onStartCommand action=${intent?.action} flags=$flags startId=$startId")
        when (intent?.action) {
            ACTION_STOP -> {
                // M9: 用户主动停 — 不视为异常, 不弹 "意外中断" 提示
                VpnStopReasonPrefs.get(this).record(VpnStopReasonPrefs.Reason.USER)
                stopService()
                stopSelf()
                return START_NOT_STICKY
            }
            ACTION_RELOAD -> {
                reloadIfRunning()
                return START_STICKY
            }
            else -> startService()
        }
        return START_STICKY
    }

    /** 订阅/overlay 变更后触发: 若 libbox 已在跑, 让它吃新的 merged.json。 */
    private fun reloadIfRunning() {
        val server = commandServer ?: run {
            Log.i(TAG, "reload: service not running, ignoring")
            return
        }
        try {
            val configJson = loadConfigJson()
            runCatching { Libbox.checkConfig(configJson) }
                .onFailure { throw IllegalStateException("reload config invalid: ${it.message}", it) }
            server.startOrReloadService(configJson, OverrideOptions())
            Log.i(TAG, "libbox config reloaded (${configJson.length} bytes)")
        } catch (t: Throwable) {
            Log.e(TAG, "reload failed", t)
        }
    }

    private fun startService() {
        // 幂等保护: 第二次 onStartCommand 进来时 (用户双击 / Android 重复投递 intent),
        // 不要再开一个 libbox instance —— 否则新旧 libbox 都会抢 9090, 导致失败路径 stopSelf()
        // 把第一个正常运行的 instance 也一起拖死。
        if (commandServer != null) {
            Log.i(TAG, "startService re-entered, already running; ignoring")
            startForeground(NOTIF_ID, buildNotification())
            return
        }
        startForeground(NOTIF_ID, buildNotification())
        try {
            val configJson = loadConfigJson()
            Log.i(TAG, "config loaded ${configJson.length} bytes")

            // 预校验, 早失败
            runCatching { Libbox.checkConfig(configJson) }
                .onFailure {
                    throw IllegalStateException("sing-box config invalid: ${it.message}", it)
                }

            val server = Libbox.newCommandServer(this, this)
            server.start()
            commandServer = server
            Log.i(TAG, "CommandServer started")

            server.startOrReloadService(configJson, OverrideOptions())
            Log.i(TAG, "sing-box service started via libbox")
            PilottyCore.markTunRunning(true)
        } catch (t: Throwable) {
            Log.e(TAG, "startService failed", t)
            // M9: 启动过程中的异常 (config_invalid / libbox 起不来) 记 CONFIG_FAIL
            VpnStopReasonPrefs.get(this).record(VpnStopReasonPrefs.Reason.CONFIG_FAIL)
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
        runCatching { DefaultNetworkMonitor.unregister() }
        // Android 12+: 没有 stopForeground 前台服务会 linger, dumpsys 里 ServiceRecord 仍可见,
        // VPN key 状态栏图标不消失。 必须显式调这个才真正释放 foreground + 通知。
        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.N) {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } else {
            @Suppress("DEPRECATION")
            stopForeground(true)
        }
        PilottyCore.markTunRunning(false)
    }

    override fun onDestroy() {
        // M9: onDestroy 被调时若 commandServer 仍持有 → 系统 OOM 回收, 视为 CRASH;
        // 若 commandServer 已经 null → 正常退出路径 (ACTION_STOP / onRevoke 已 record 过)
        if (commandServer != null) {
            VpnStopReasonPrefs.get(this).record(VpnStopReasonPrefs.Reason.CRASH)
        }
        stopService()
        super.onDestroy()
    }

    override fun onRevoke() {
        // M9: 用户在系统设置里撤销了 VPN 授权, 或另一个 VPN 抢了 (Android 限制只能一个 active)
        VpnStopReasonPrefs.get(this).record(VpnStopReasonPrefs.Reason.REVOKE)
        stopService()
        super.onRevoke()
    }

    /**
     * 配置来源优先级:
     *  1) filesDir/merged.json —— Go overlay.Apply() 生成 (mobile.Client dataDir=filesDir);
     *     经 ConfigMerger 二次注入 tun inbound / route 系统规则 (详见 ConfigMerger 文档 + Known Issue #H4)
     *  2) filesDir/configs/android_tun_base.json —— PilottyApp.onCreate() 从 assets 拷出
     */
    private fun loadConfigJson(): String {
        val tunBaseFile = File(filesDir, "configs/${PilottyApp.TUN_BASE_NAME}")
        val mergedFile = File(filesDir, "merged.json")

        val perAppSnap = PerAppVpnPrefs.get(this).state.value
        val sniffSnap = SniffingIpv6Prefs.get(this).state.value
        val localProxySnap = LocalProxyPrefs.get(this).state.value

        if (mergedFile.exists() && mergedFile.length() > 0) {
            Log.i(TAG, "using config ${mergedFile.path}")
            val merged = mergedFile.readText()
            val withTun = if (tunBaseFile.exists() && tunBaseFile.length() > 0) {
                ConfigMerger.ensureTunInbound(merged, tunBaseFile.readText())
            } else {
                Log.w(TAG, "tun base asset missing, merged.json used as-is (may lack tun inbound!)")
                merged
            }
            val withPerApp = ConfigMerger.injectPerAppRules(withTun, perAppSnap)
            val withSniff = ConfigMerger.injectSniffingIpv6(withPerApp, sniffSnap)
            return ConfigMerger.injectLocalProxy(withSniff, localProxySnap)
        }
        if (tunBaseFile.exists() && tunBaseFile.length() > 0) {
            Log.i(TAG, "using config ${tunBaseFile.path} (no merged.json)")
            val withPerApp = ConfigMerger.injectPerAppRules(tunBaseFile.readText(), perAppSnap)
            val withSniff = ConfigMerger.injectSniffingIpv6(withPerApp, sniffSnap)
            return ConfigMerger.injectLocalProxy(withSniff, localProxySnap)
        }
        error("no sing-box config found; expected ${mergedFile.path} or ${tunBaseFile.path}")
    }

    private fun buildConfigureIntent(): PendingIntent = PendingIntent.getActivity(
        this, 0, Intent(this, MainActivity::class.java),
        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
    )

    private fun buildNotification(): Notification {
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val ch = NotificationChannel(
                CHANNEL_ID,
                getString(com.pilotty.app.R.string.notif_channel_name),
                NotificationManager.IMPORTANCE_LOW,
            )
            nm.createNotificationChannel(ch)
        }
        // Phase 10-E-B: 加停止按钮, 锁屏下拉可直接关 VPN 不用先 unlock 进 App
        val stopIntent = Intent(this, PilottyVpnService::class.java).apply {
            action = ACTION_STOP
        }
        val stopPi = PendingIntent.getService(
            this,
            1,
            stopIntent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(getString(com.pilotty.app.R.string.app_name))
            .setContentText(getString(com.pilotty.app.R.string.notif_text_connected))
            // M3: 小图标用品牌 tile (盾 + 对勾, 单色, 适配通知栏 tint),
            // 取代 Android 内置 ic_lock_lock (和 Pilotty 品牌无关)
            .setSmallIcon(com.pilotty.app.R.drawable.ic_tile_pilotty)
            .setContentIntent(buildConfigureIntent())
            .addAction(
                android.R.drawable.ic_menu_close_clear_cancel,
                getString(com.pilotty.app.R.string.notif_action_stop),
                stopPi,
            )
            .setOngoing(true)
            .build()
    }

    // ──────────────── 工具方法 ─────────────────

    /** 迭代 RoutePrefixIterator; 返回实际处理的条目数。 */
    private inline fun iterateRoute(
        iter: libbox.RoutePrefixIterator?,
        action: (address: String, prefix: Int) -> Unit,
    ): Int {
        if (iter == null) return 0
        var n = 0
        runCatching {
            while (iter.hasNext()) {
                val p = iter.next()
                action(p.address(), p.prefix())
                n++
            }
        }.onFailure { Log.w(TAG, "route iterate error after $n items", it) }
        return n
    }

    /** 迭代 StringIterator; 返回实际处理的条目数。 */
    private inline fun iterateStrings(
        iter: libbox.StringIterator?,
        action: (value: String) -> Unit,
    ): Int {
        if (iter == null) return 0
        var n = 0
        runCatching {
            while (iter.hasNext()) {
                action(iter.next())
                n++
            }
        }.onFailure { Log.w(TAG, "strings iterate error after $n items", it) }
        return n
    }

    companion object {
        private const val TAG = "PilottyVpnService"
        private const val CHANNEL_ID = "pilotty_vpn"
        private const val NOTIF_ID = 1001
        const val ACTION_STOP = "com.pilotty.app.STOP_VPN"
        const val ACTION_RELOAD = "com.pilotty.app.RELOAD_VPN"

        /** 订阅/规则变更后请求 VpnService 重载 libbox 配置;未运行时为 no-op。 */
        fun requestReload(ctx: Context) {
            val intent = Intent(ctx, PilottyVpnService::class.java).apply {
                action = ACTION_RELOAD
            }
            runCatching { ctx.startService(intent) }
        }
    }
}
