package com.pilotty.app.vpn

import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.util.Log
import com.pilotty.app.PilottyApp
import libbox.InterfaceUpdateListener
import java.net.NetworkInterface

/**
 * Android 默认物理网络监听 —— libbox.PlatformInterface.startDefaultInterfaceMonitor 的真实现。
 *
 * 关键动机 (Known Issue #H1): 没有这个, sing-box 不知道默认物理出口, 自己发起的上游连接会回环进 TUN
 * 无限循环, 表现为 "UI 已连接但 ping 超时"。
 *
 * Android P (API 28) 起 `registerDefaultNetworkCallback` 会把 VPN 自己也当"默认网络"返回;
 * 结果是 TUN 一启动, 回调就把 tun0 当成 underlying, sing-box 出口指向自己 => 死循环。
 * 修复: 用 `registerBestMatchingNetworkCallback` (API 31+) 或 `requestNetwork` (API 28+) 带
 * NetworkRequest filter (默认不含 VPN), 才能拿到真实的 wlan0 / cellular。
 *
 * 抄作业: NekoBoxForAndroid/.../DefaultNetworkListener.kt + hiddify-app DefaultNetworkMonitor.kt。
 */
object DefaultNetworkMonitor {
    private const val TAG = "DefaultNetworkMonitor"

    @Volatile private var listener: InterfaceUpdateListener? = null
    @Volatile private var defaultNetwork: Network? = null
    private var registered = false
    private val mainHandler = Handler(Looper.getMainLooper())

    private val request: NetworkRequest = NetworkRequest.Builder().apply {
        addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
        addCapability(NetworkCapabilities.NET_CAPABILITY_NOT_RESTRICTED)
        // Builder 默认包含 NET_CAPABILITY_NOT_VPN, 正是我们要的 —— VPN 自身不会匹配
    }.build()

    private val callback = object : ConnectivityManager.NetworkCallback() {
        override fun onAvailable(network: Network) {
            defaultNetwork = network
            fireUpdate(network)
        }

        override fun onLost(network: Network) {
            if (defaultNetwork == network) {
                defaultNetwork = null
                fireUpdate(null)
            }
        }

        override fun onCapabilitiesChanged(network: Network, caps: NetworkCapabilities) {
            defaultNetwork = network
            fireUpdate(network)
        }
    }

    fun setListener(l: InterfaceUpdateListener?) {
        listener = l
        if (l != null) {
            ensureRegistered()
            fireUpdate(defaultNetwork)
        }
    }

    private fun ensureRegistered() {
        if (registered) return
        val cm = PilottyApp.connectivity ?: run {
            Log.w(TAG, "connectivity service unavailable, cannot register")
            return
        }
        runCatching {
            when {
                Build.VERSION.SDK_INT >= Build.VERSION_CODES.S -> {
                    // API 31+: 带 filter 的 best-matching, 不会把 VPN 当默认
                    cm.registerBestMatchingNetworkCallback(request, callback, mainHandler)
                }
                Build.VERSION.SDK_INT >= Build.VERSION_CODES.P -> {
                    // API 28-30: 用 requestNetwork 避开 registerDefaultNetworkCallback 把 VPN 当默认的 bug
                    cm.requestNetwork(request, callback, mainHandler)
                }
                else -> {
                    cm.registerDefaultNetworkCallback(callback, mainHandler)
                }
            }
            registered = true
            Log.i(TAG, "default network callback registered (api=${Build.VERSION.SDK_INT})")
        }.onFailure { Log.w(TAG, "register callback failed", it) }
    }

    fun unregister() {
        val cm = PilottyApp.connectivity ?: return
        if (!registered) return
        runCatching { cm.unregisterNetworkCallback(callback) }
            .onFailure { Log.w(TAG, "unregisterNetworkCallback", it) }
        registered = false
        defaultNetwork = null
        listener = null
    }

    private fun fireUpdate(network: Network?) {
        val l = listener ?: return
        if (network == null) {
            runCatching { l.updateDefaultInterface("", -1, false, false) }
                .onFailure { Log.w(TAG, "updateDefaultInterface(empty)", it) }
            return
        }
        val cm = PilottyApp.connectivity ?: return
        val name = cm.getLinkProperties(network)?.interfaceName ?: return
        // NetworkInterface.getByName 在 callback 触发瞬间可能返回 null (内核慢一拍), 轮询 10 次
        var idx = -1
        for (i in 0 until 10) {
            val ni = runCatching { NetworkInterface.getByName(name) }.getOrNull()
            if (ni != null) { idx = ni.index; break }
            try { Thread.sleep(50) } catch (_: InterruptedException) { break }
        }
        if (idx < 0) {
            Log.w(TAG, "interface index for $name not resolved, skip")
            return
        }
        val caps = cm.getNetworkCapabilities(network)
        val expensive = caps?.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_METERED)?.not() ?: false
        val constrained = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            caps?.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_BANDWIDTH_CONSTRAINED)?.not() ?: false
        } else false
        Log.i(TAG, "updateDefaultInterface name=$name idx=$idx expensive=$expensive")
        runCatching { l.updateDefaultInterface(name, idx, expensive, constrained) }
            .onFailure { Log.w(TAG, "updateDefaultInterface($name)", it) }
    }
}
