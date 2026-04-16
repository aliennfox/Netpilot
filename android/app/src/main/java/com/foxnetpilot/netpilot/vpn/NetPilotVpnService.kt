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

/**
 * Android VpnService 实现：
 *  1. 通过 VpnService.Builder 申请 TUN 接口（IPv4/IPv6 + DNS + 路由）
 *  2. 把 fd detach 后传给 Go core ([NetPilotCore.startTun])
 *  3. 作为前台服务运行，避免被系统回收
 *
 * 真正的数据面（sing-box libbox）由 Go 侧 StartTun 内部启动；本类只负责 fd 与生命周期。
 */
class NetPilotVpnService : VpnService() {

    private var pfd: ParcelFileDescriptor? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> {
                stopTun()
                stopSelf()
                return START_NOT_STICKY
            }
            else -> startTun()
        }
        return START_STICKY
    }

    private fun startTun() {
        startForeground(NOTIF_ID, buildNotification())
        try {
            val builder = Builder()
                .setSession("NetPilot")
                .setMtu(1500)
                .addAddress("172.19.0.1", 30)
                .addAddress("fdfe:dcba:9876::1", 126)
                .addRoute("0.0.0.0", 0)
                .addRoute("::", 0)
                .addDnsServer("1.1.1.1")
                .addDnsServer("8.8.8.8")
                .setBlocking(false)
            // 排除 NetPilot 自己的流量，避免回环
            try { builder.addDisallowedApplication(packageName) } catch (_: Throwable) {}

            val tun = builder.establish() ?: run {
                Log.e(TAG, "VpnService.Builder.establish() returned null")
                stopSelf(); return
            }
            pfd = tun
            val fd = tun.detachFd()
            // configJSON 留空：让 Go 侧使用 overlay 合并后的当前配置
            NetPilotCore.startTun(fd, "")
            Log.i(TAG, "TUN started fd=$fd")
        } catch (t: Throwable) {
            Log.e(TAG, "startTun failed", t)
            stopSelf()
        }
    }

    private fun stopTun() {
        runCatching { NetPilotCore.stopTun() }
        runCatching { pfd?.close() }
        pfd = null
    }

    override fun onDestroy() {
        stopTun()
        super.onDestroy()
    }

    override fun onRevoke() {
        stopTun()
        super.onRevoke()
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
            // android.R.drawable.stat_sys_vpn_ic 是 @hide API,改用公开的锁图标占位;
            // 正式发版前应在 res/drawable/ 自备 vector 图标(见 C1)。
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
