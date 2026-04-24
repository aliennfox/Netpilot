package com.pilotty.app.vpn

import android.content.Context
import android.content.SharedPreferences
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/**
 * A4 本地入站代理偏好。 VPN 运行时额外开一个 mixed (SOCKS5 + HTTP) inbound 绑到
 * 0.0.0.0:port, 让 LAN 里其他设备 (PC / 另一台手机) 把 Android 当出口代理。
 *
 * 安全提示: 默认 listen=0.0.0.0 无鉴权。 **所以必须让用户手动开关 + 显式提示风险** —
 * 用户家 WiFi 是可信网络时才开; 咖啡厅 WiFi 绝对不能开。 UI 文案要能吓退误用。
 *
 * 端口范围 1025-65535, 默认 7890 (Clash 习惯)。 port conflict 由 sing-box 在 Start 时报错,
 * UI 层捕获后提示用户换端口。
 */
class LocalProxyPrefs private constructor(private val sp: SharedPreferences) {
    data class Snapshot(
        val enabled: Boolean = false,
        val port: Int = DEFAULT_PORT,
    )

    private val _state = MutableStateFlow(load())
    val state: StateFlow<Snapshot> = _state.asStateFlow()

    private fun load(): Snapshot = Snapshot(
        enabled = sp.getBoolean(KEY_ENABLED, false),
        port = sp.getInt(KEY_PORT, DEFAULT_PORT).coerceIn(MIN_PORT, MAX_PORT),
    )

    fun setEnabled(v: Boolean) {
        sp.edit().putBoolean(KEY_ENABLED, v).apply()
        _state.value = _state.value.copy(enabled = v)
    }

    fun setPort(v: Int) {
        val clamped = v.coerceIn(MIN_PORT, MAX_PORT)
        sp.edit().putInt(KEY_PORT, clamped).apply()
        _state.value = _state.value.copy(port = clamped)
    }

    companion object {
        const val DEFAULT_PORT = 7890
        const val MIN_PORT = 1025
        const val MAX_PORT = 65535
        private const val KEY_ENABLED = "enabled"
        private const val KEY_PORT = "port"

        @Volatile private var inst: LocalProxyPrefs? = null
        fun get(ctx: Context): LocalProxyPrefs = inst ?: synchronized(this) {
            inst ?: LocalProxyPrefs(
                ctx.applicationContext.getSharedPreferences("local_proxy", Context.MODE_PRIVATE)
            ).also { inst = it }
        }
    }
}
