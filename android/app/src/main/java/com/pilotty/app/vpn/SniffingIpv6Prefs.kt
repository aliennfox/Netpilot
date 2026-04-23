package com.pilotty.app.vpn

import android.content.Context
import android.content.SharedPreferences
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

/**
 * Sniffing 档位 + IPv6 模式偏好 (Phase P1-A)。
 *
 * Sniffing 是 sing-box 从 TCP/UDP 首包嗅探域名 / 协议的能力, 关掉后 ip-cidr / ip_is_private
 * 以外的规则 (domain_suffix / domain_keyword) 对于"直接走 IP"的应用会不生效。默认开。
 *
 * IPv6 模式对应 sing-box dns.strategy + tun inbound address:
 *  - PreferIpv4 (默认): dns.strategy=prefer_ipv4, tun 双栈
 *  - PreferIpv6       : dns.strategy=prefer_ipv6, tun 双栈
 *  - Ipv4Only         : dns.strategy=ipv4_only,  tun 只 IPv4
 *  - Ipv6Only         : dns.strategy=ipv6_only,  tun 只 IPv6 (小众但对称暴露)
 */
class SniffingIpv6Prefs(ctx: Context) {

    enum class Ipv6Mode(val raw: String, val label: String) {
        PreferIpv4("prefer_ipv4", "优先 IPv4"),
        PreferIpv6("prefer_ipv6", "优先 IPv6"),
        Ipv4Only("ipv4_only", "仅 IPv4"),
        Ipv6Only("ipv6_only", "仅 IPv6");

        companion object {
            fun fromRaw(s: String?): Ipv6Mode = values().firstOrNull { it.raw == s } ?: PreferIpv4
        }
    }

    private val sp: SharedPreferences =
        ctx.applicationContext.getSharedPreferences(FILE, Context.MODE_PRIVATE)

    data class Snapshot(val sniff: Boolean, val ipv6Mode: Ipv6Mode)

    private val _state = MutableStateFlow(currentSnapshot())
    val state: StateFlow<Snapshot> = _state

    fun sniff(): Boolean = sp.getBoolean(KEY_SNIFF, true)
    fun ipv6Mode(): Ipv6Mode = Ipv6Mode.fromRaw(sp.getString(KEY_IPV6, Ipv6Mode.PreferIpv4.raw))

    fun setSniff(on: Boolean) {
        sp.edit().putBoolean(KEY_SNIFF, on).apply()
        _state.value = currentSnapshot()
    }

    fun setIpv6Mode(mode: Ipv6Mode) {
        sp.edit().putString(KEY_IPV6, mode.raw).apply()
        _state.value = currentSnapshot()
    }

    private fun currentSnapshot() = Snapshot(sniff(), ipv6Mode())

    companion object {
        private const val FILE = "sniffing_ipv6"
        private const val KEY_SNIFF = "sniff"
        private const val KEY_IPV6 = "ipv6_mode"

        @Volatile private var INSTANCE: SniffingIpv6Prefs? = null

        fun get(ctx: Context): SniffingIpv6Prefs =
            INSTANCE ?: synchronized(this) {
                INSTANCE ?: SniffingIpv6Prefs(ctx).also { INSTANCE = it }
            }
    }
}
