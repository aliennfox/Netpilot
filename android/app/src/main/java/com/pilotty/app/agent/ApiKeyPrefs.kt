package com.pilotty.app.agent

import android.content.Context
import android.content.SharedPreferences
import android.util.Log
import com.pilotty.app.PilottyCore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/**
 * LLM apiKey 存储 (M8 最短路径)。
 *
 * **重要**: 当前使用明文 SharedPreferences。 自用场景可接受, v1.x 必须升级到
 * EncryptedSharedPreferences (androidx.security-crypto)。 已在 [CLAUDE.md] 和
 * `docs/android-mvp-gap.md` 标注为安全债。
 *
 * 生命周期:
 *  - [PilottyApp.onCreate] 调 [get].key 读一次, 传给 PilottyCore.init 做首次初始化
 *  - 用户在 Settings UI 改 key → [set] 更新 SP + 广播 StateFlow + 调 PilottyCore.setApiKey
 *    触发 Go 侧 orchestrator 热重载, 无需重启 App (阶段 2 2026-04-22 完成)
 */
class ApiKeyPrefs private constructor(private val sp: SharedPreferences) {

    private val _state = MutableStateFlow(sp.getString(KEY, "").orEmpty())
    val state: StateFlow<String> = _state.asStateFlow()

    val key: String get() = _state.value
    val isConfigured: Boolean get() = _state.value.isNotBlank()

    fun set(value: String) {
        val trimmed = value.trim()
        sp.edit().putString(KEY, trimmed).apply()
        _state.value = trimmed
        // 热重载 Go orchestrator。 gomobile 方法是阻塞 JNI, 丢到 IO 线程避免主线程 ANR
        CoroutineScope(Dispatchers.IO).launch {
            runCatching { PilottyCore.setApiKey(trimmed) }
                .onSuccess { Log.i(TAG, "setApiKey reloaded: agent_ready=${trimmed.isNotBlank()}") }
                .onFailure { Log.w(TAG, "setApiKey reload failed", it) }
        }
    }

    fun clear() = set("")

    /** "sk-abc...xy12" → "sk-***xy12" 供 UI 展示, 绝不暴露完整值。 */
    fun masked(): String {
        val k = _state.value
        if (k.length < 8) return if (k.isBlank()) "" else "***"
        return k.take(3) + "***" + k.takeLast(4)
    }

    companion object {
        private const val TAG = "ApiKeyPrefs"
        private const val FILE = "agent_apikey"
        private const val KEY = "llm_api_key"

        @Volatile private var instance: ApiKeyPrefs? = null

        fun get(ctx: Context): ApiKeyPrefs {
            instance?.let { return it }
            return synchronized(this) {
                instance ?: ApiKeyPrefs(
                    ctx.applicationContext.getSharedPreferences(FILE, Context.MODE_PRIVATE),
                ).also { instance = it }
            }
        }
    }
}
