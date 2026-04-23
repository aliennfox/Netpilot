package com.pilotty.app.ui.settings

import android.content.Context
import android.content.SharedPreferences
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/**
 * ThemeMode 持久化 (Phase 10-E)。
 *
 * 之前 MainActivity 用 rememberSaveable 只跨 config change 不跨 App 重启 ——
 * 用户选 Dark 后杀进程再开又回到 System。 这里走 SharedPreferences + StateFlow,
 * 启动时读取 / 改动时写回 + 广播给订阅者 (MainActivity 直接 collect)。
 */
class ThemePrefs private constructor(private val sp: SharedPreferences) {

    private val _mode = MutableStateFlow(readMode())
    val mode: StateFlow<ThemeMode> = _mode.asStateFlow()

    fun set(m: ThemeMode) {
        sp.edit().putString(KEY, m.name).apply()
        _mode.value = m
    }

    private fun readMode(): ThemeMode {
        val raw = sp.getString(KEY, null) ?: return ThemeMode.System
        return runCatching { ThemeMode.valueOf(raw) }.getOrDefault(ThemeMode.System)
    }

    companion object {
        private const val FILE = "theme"
        private const val KEY = "mode"

        @Volatile private var instance: ThemePrefs? = null

        fun get(ctx: Context): ThemePrefs {
            instance?.let { return it }
            return synchronized(this) {
                instance ?: ThemePrefs(
                    ctx.applicationContext.getSharedPreferences(FILE, Context.MODE_PRIVATE),
                ).also { instance = it }
            }
        }
    }
}
