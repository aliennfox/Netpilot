package com.pilotty.app.vpn

import android.content.Context
import android.content.SharedPreferences
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/**
 * M9 · VPN 异常退出 / 系统回收 的用户反馈。
 *
 * VpnService 每次 stop 都写一条 reason (user 主动 / revoke / crash / config_fail),
 * UI 层 (HomeScreen) 启动时若发现最后一次 stop 不是 user, 提示"VPN 意外中断"
 * 并引导用户去"实时日志"看详情 (M7 已做导出)。
 *
 * 写入 SharedPreferences, 重启 / 杀进程都留得住。 用户点"我知道了"后 ack, 提示消失。
 */
class VpnStopReasonPrefs private constructor(private val sp: SharedPreferences) {

    enum class Reason { NONE, USER, REVOKE, CRASH, CONFIG_FAIL }

    data class Snapshot(
        val reason: Reason = Reason.NONE,
        val ts: Long = 0L,
        /** 用户 ack 过 = 不用再弹。 每次写 reason 会重置为 false。 */
        val acked: Boolean = true,
    )

    private val _state = MutableStateFlow(load())
    val state: StateFlow<Snapshot> = _state.asStateFlow()

    private fun load(): Snapshot = Snapshot(
        reason = runCatching { Reason.valueOf(sp.getString(KEY_REASON, Reason.NONE.name)!!) }
            .getOrDefault(Reason.NONE),
        ts = sp.getLong(KEY_TS, 0L),
        acked = sp.getBoolean(KEY_ACKED, true),
    )

    /** VpnService 在 stopService() 时调; user 停用则 reason=USER, 其余为异常。 */
    fun record(reason: Reason) {
        sp.edit()
            .putString(KEY_REASON, reason.name)
            .putLong(KEY_TS, System.currentTimeMillis())
            // user 主动停的不用弹提示; 其它情况弹
            .putBoolean(KEY_ACKED, reason == Reason.USER)
            .apply()
        _state.value = Snapshot(reason, System.currentTimeMillis(), reason == Reason.USER)
    }

    /** 用户点"我知道了"后调, 下次启动 Home 不再弹。 */
    fun ack() {
        if (_state.value.acked) return
        sp.edit().putBoolean(KEY_ACKED, true).apply()
        _state.value = _state.value.copy(acked = true)
    }

    companion object {
        private const val KEY_REASON = "reason"
        private const val KEY_TS = "ts"
        private const val KEY_ACKED = "acked"

        @Volatile private var inst: VpnStopReasonPrefs? = null
        fun get(ctx: Context): VpnStopReasonPrefs = inst ?: synchronized(this) {
            inst ?: VpnStopReasonPrefs(
                ctx.applicationContext.getSharedPreferences("vpn_stop_reason", Context.MODE_PRIVATE)
            ).also { inst = it }
        }
    }
}
