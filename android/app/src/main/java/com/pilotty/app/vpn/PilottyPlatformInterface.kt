package com.pilotty.app.vpn

import android.net.NetworkCapabilities
import android.os.Build
import android.system.OsConstants
import android.util.Base64
import androidx.annotation.RequiresApi
import com.pilotty.app.PilottyApp
import libbox.ConnectionOwner
import libbox.InterfaceUpdateListener
import libbox.Libbox
import libbox.LocalDNSTransport
import libbox.NetworkInterface as LibboxNetworkInterface
import libbox.NetworkInterfaceIterator
import libbox.Notification as LibboxNotification
import libbox.PlatformInterface
import libbox.StringIterator
import libbox.TunOptions
import libbox.WIFIState
import java.net.Inet6Address
import java.net.InetSocketAddress
import java.net.NetworkInterface

/**
 * libbox.PlatformInterface 的 Kotlin interface-with-defaults 实现。
 * 抄作业来源: hiddify-app/.../PlatformInterfaceWrapper.kt + sing-box-for-android。
 *
 * 设计要点:
 *  - 所有 libbox 要求的方法都给"合理默认实现", 子类 (PilottyVpnService) 只需 override
 *    openTun / autoDetectInterfaceControl 两个 VpnService 相关方法
 *  - 多继承模拟: PilottyVpnService : VpnService(), PilottyPlatformInterface 同时满足
 *    framework 要求 + Go 侧回调契约
 *  - getInterfaces / startDefaultInterfaceMonitor 真实实现 ——
 *    没有真数据 sing-box 的上游连接会回环进自己的 TUN, 导致 "UI 已连接但 ping 超时"
 */
interface PilottyPlatformInterface : PlatformInterface {

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

    // 默认网络监听: 真实现, 否则 sing-box 上游连接会无限回环进 TUN (Known Issue #H1)
    override fun startDefaultInterfaceMonitor(listener: InterfaceUpdateListener?) {
        DefaultNetworkMonitor.setListener(listener)
    }
    override fun closeDefaultInterfaceMonitor(listener: InterfaceUpdateListener?) {
        DefaultNetworkMonitor.setListener(null)
    }

    /**
     * 枚举所有活动的物理网络接口, 供 sing-box 选择上游出口。
     * 对 VpnService 尤其关键: 没有真数据 sing-box 无法识别 wlan0/cellular, 会把上游连接
     * 错误地又发回 TUN, 造成无限回环。
     */
    override fun getInterfaces(): NetworkInterfaceIterator {
        val cm = PilottyApp.connectivity
            ?: return InterfaceArray(emptyList())
        val jInterfaces = runCatching { NetworkInterface.getNetworkInterfaces()?.toList() }
            .getOrNull() ?: emptyList()
        val result = mutableListOf<LibboxNetworkInterface>()
        for (network in cm.allNetworks) {
            val lp = cm.getLinkProperties(network) ?: continue
            val caps = cm.getNetworkCapabilities(network) ?: continue
            val name = lp.interfaceName ?: continue
            val jIf = jInterfaces.find { it.name == name } ?: continue

            val iface = LibboxNetworkInterface()
            iface.name = name
            iface.index = jIf.index
            runCatching { iface.mtu = jIf.mtu }

            iface.type = when {
                caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) -> Libbox.InterfaceTypeWIFI
                caps.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) -> Libbox.InterfaceTypeCellular
                caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) -> Libbox.InterfaceTypeEthernet
                else -> Libbox.InterfaceTypeOther
            }

            iface.addresses = StringArray(
                jIf.interfaceAddresses.map { ia ->
                    val a = ia.address
                    val host = if (a is Inet6Address) {
                        Inet6Address.getByAddress(a.address).hostAddress
                    } else {
                        a.hostAddress
                    }
                    "$host/${ia.networkPrefixLength}"
                }
            )
            iface.dnsServer = StringArray(
                lp.dnsServers.mapNotNull { it.hostAddress }
            )

            var flags = 0
            if (caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)) {
                flags = OsConstants.IFF_UP or OsConstants.IFF_RUNNING
            }
            if (jIf.isLoopback) flags = flags or OsConstants.IFF_LOOPBACK
            if (jIf.isPointToPoint) flags = flags or OsConstants.IFF_POINTOPOINT
            if (jIf.supportsMulticast()) flags = flags or OsConstants.IFF_MULTICAST
            iface.flags = flags

            iface.metered = !caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_METERED)
            result.add(iface)
        }
        return InterfaceArray(result)
    }

    // WIFI 状态 (3B-5 接 WifiManager.connectionInfo, 当前 null 不影响 TUN)
    override fun readWIFIState(): WIFIState? = null

    // 系统根证书
    override fun systemCertificates(): StringIterator {
        val certificates = mutableListOf<String>()
        try {
            val keyStore = java.security.KeyStore.getInstance("AndroidCAStore")
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
        val cm = PilottyApp.connectivity ?: error("connectivity service not ready")
        val uid = cm.getConnectionOwnerUid(
            ipProtocol,
            InetSocketAddress(sourceAddress, sourcePort),
            InetSocketAddress(destinationAddress, destinationPort),
        )
        val owner = ConnectionOwner()
        owner.userId = uid
        val pm = PilottyApp.packageManager ?: return owner
        val packages = pm.getPackagesForUid(uid)
        owner.userName = packages?.firstOrNull() ?: ""
        owner.setAndroidPackageNames(StringArray(packages?.toList() ?: emptyList()))
        return owner
    }

    // 通知回调 (sing-box 发诊断通知时调用;当前 no-op, UI 后续接)
    override fun sendNotification(notification: LibboxNotification?) {}

    /** StringIterator 最小实现 */
    class StringArray(private val list: List<String>) : StringIterator {
        private val iter = list.iterator()
        override fun hasNext(): Boolean = iter.hasNext()
        override fun next(): String = iter.next()
        override fun len(): Int = list.size
    }

    /** NetworkInterfaceIterator 最小实现 */
    class InterfaceArray(private val list: List<LibboxNetworkInterface>) : NetworkInterfaceIterator {
        private val iter = list.iterator()
        override fun hasNext(): Boolean = iter.hasNext()
        override fun next(): LibboxNetworkInterface = iter.next()
    }

}
