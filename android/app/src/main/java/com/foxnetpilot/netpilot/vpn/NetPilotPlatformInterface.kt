package com.foxnetpilot.netpilot.vpn

import android.net.ConnectivityManager
import android.os.Build
import android.util.Base64
import androidx.annotation.RequiresApi
import com.foxnetpilot.netpilot.NetPilotApp
import libbox.ConnectionOwner
import libbox.InterfaceUpdateListener
import libbox.LocalDNSTransport
import libbox.NetworkInterface as LibboxNetworkInterface
import libbox.NetworkInterfaceIterator
import libbox.Notification as LibboxNotification
import libbox.PlatformInterface
import libbox.StringIterator
import libbox.TunOptions
import libbox.WIFIState
import java.net.InetSocketAddress
import java.security.KeyStore

/**
 * libbox.PlatformInterface 的 Kotlin interface-with-defaults 实现。
 * 抄作业来源: ~/References/ 及 sing-box-for-android 的 PlatformInterfaceWrapper.kt (204 行).
 *
 * 设计要点:
 *  - 所有 15 个 libbox 要求的方法都给"合理默认实现", 子类 (NetPilotVpnService) 只需 override
 *    openTun / autoDetectInterfaceControl 两个 VpnService 相关方法
 *  - 多继承模拟: NetPilotVpnService : VpnService(), NetPilotPlatformInterface 同时满足
 *    framework 要求 + Go 侧回调契约
 *  - 3B-3 初版: 若 sing-box 调到未实现的方法(例如 getInterfaces / findConnectionOwner), 返回
 *    最简 stub, 不崩溃为前提; 3B-5 稳定性阶段按需补充真实现
 */
interface NetPilotPlatformInterface : PlatformInterface {

    // VpnService 子类必须实现的两个关键方法 (openTun 和 protect)
    override fun openTun(options: TunOptions): Int { error("openTun must be implemented by VpnService subclass") }
    override fun autoDetectInterfaceControl(fd: Int) {} // 默认 no-op, VpnService 重写为 protect(fd)

    // 平台元信息
    override fun usePlatformAutoDetectInterfaceControl(): Boolean = true
    override fun useProcFS(): Boolean = Build.VERSION.SDK_INT < Build.VERSION_CODES.Q
    override fun underNetworkExtension(): Boolean = false
    override fun includeAllNetworks(): Boolean = false

    // DNS / 缓存
    override fun clearDNSCache() {}
    override fun localDNSTransport(): LocalDNSTransport? = null

    // 默认网络监听 (3B-5 接真 ConnectivityManager.NetworkCallback; 当前无 op 不影响 TUN 建立)
    override fun startDefaultInterfaceMonitor(listener: InterfaceUpdateListener?) {}
    override fun closeDefaultInterfaceMonitor(listener: InterfaceUpdateListener?) {}

    // 接口列表 (sing-box 启动时调用;空迭代器让内核走默认路径)
    override fun getInterfaces(): NetworkInterfaceIterator = emptyInterfaceIterator()

    // WIFI 状态 (3B-5 接 WifiManager.connectionInfo)
    override fun readWIFIState(): WIFIState? = null

    // 系统根证书
    override fun systemCertificates(): StringIterator {
        val certificates = mutableListOf<String>()
        try {
            val keyStore = KeyStore.getInstance("AndroidCAStore")
            keyStore.load(null, null)
            val aliases = keyStore.aliases()
            while (aliases.hasMoreElements()) {
                val cert = keyStore.getCertificate(aliases.nextElement())
                certificates.add(
                    "-----BEGIN CERTIFICATE-----\n" +
                        Base64.encodeToString(cert.encoded, Base64.DEFAULT).trim() +
                        "\n-----END CERTIFICATE-----"
                )
            }
        } catch (_: Throwable) {
            // AndroidCAStore 不可用时返回空迭代器, sing-box 会走其自带 CA 包
        }
        return StringArray(certificates)
    }

    // 连接归属查询 (per-app route 归因)。Android Q+ 用系统 API。
    @RequiresApi(Build.VERSION_CODES.Q)
    override fun findConnectionOwner(
        ipProtocol: Int,
        sourceAddress: String,
        sourcePort: Int,
        destinationAddress: String,
        destinationPort: Int,
    ): ConnectionOwner {
        val cm = NetPilotApp.connectivity ?: error("connectivity service not ready")
        val uid = cm.getConnectionOwnerUid(
            ipProtocol,
            InetSocketAddress(sourceAddress, sourcePort),
            InetSocketAddress(destinationAddress, destinationPort),
        )
        val owner = ConnectionOwner()
        owner.userId = uid
        val pm = NetPilotApp.packageManager ?: return owner
        val packages = pm.getPackagesForUid(uid)
        owner.userName = packages?.firstOrNull() ?: ""
        owner.setAndroidPackageNames(StringArray(packages?.toList() ?: emptyList()))
        return owner
    }

    // 通知回调 (sing-box 发诊断通知时调用;当前 no-op, UI 后续接)
    override fun sendNotification(notification: LibboxNotification?) {}

    /** StringIterator 最小实现,供 systemCertificates / package names 使用 */
    class StringArray(private val list: List<String>) : StringIterator {
        private val iter = list.iterator()
        override fun hasNext(): Boolean = iter.hasNext()
        override fun next(): String = iter.next()
        override fun len(): Int = list.size.toLong().toInt()
    }

    companion object {
        /** 空 NetworkInterface 迭代器,让 sing-box fallback 到默认策略 */
        private fun emptyInterfaceIterator() = object : NetworkInterfaceIterator {
            override fun hasNext(): Boolean = false
            override fun next(): LibboxNetworkInterface =
                throw NoSuchElementException("empty iterator")
        }
    }
}
