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
 * LLM 三件套配置 (BaseURL + Model + ApiKey) 持久化。
 *
 * 取代 [ApiKeyPrefs] 的单字段方案。 仍然明文 SP, 同 ApiKeyPrefs 安全债, v1.x 升 EncryptedSharedPreferences。
 *
 * 生命周期:
 *  - PilottyApp.onCreate 读三个值, 一次性传给 PilottyCore.init / setLLMConfig
 *  - 用户在 Settings UI 改任一字段 → [set] 写 SP + 广播 StateFlow + 调
 *    PilottyCore.setLLMConfig 触发 Go orchestrator 热重载
 *  - 兼容旧 ApiKeyPrefs: 读迁移仅做一次, 见 [migrateFromApiKeyPrefs]
 */
class LLMConfigPrefs private constructor(private val sp: SharedPreferences) {

    data class Config(val baseURL: String, val model: String, val apiKey: String) {
        val isConfigured: Boolean get() = apiKey.isNotBlank()
    }

    private val _state = MutableStateFlow(loadFromSP())
    val state: StateFlow<Config> = _state.asStateFlow()

    val baseURL: String get() = _state.value.baseURL
    val model: String get() = _state.value.model
    val apiKey: String get() = _state.value.apiKey
    val isConfigured: Boolean get() = _state.value.isConfigured

    fun set(baseURL: String, model: String, apiKey: String) {
        val b = baseURL.trim()
        val m = model.trim()
        val k = apiKey.trim()
        sp.edit()
            .putString(KEY_BASE_URL, b)
            .putString(KEY_MODEL, m)
            .putString(KEY_API_KEY, k)
            .apply()
        _state.value = Config(b, m, k)
        // 热重载 Go orchestrator
        CoroutineScope(Dispatchers.IO).launch {
            runCatching { PilottyCore.setLLMConfig(b, m, k) }
                .onSuccess { Log.i(TAG, "setLLMConfig reloaded: agent_ready=${k.isNotBlank()}") }
                .onFailure { Log.w(TAG, "setLLMConfig failed", it) }
        }
    }

    fun clear() = set("", "", "")

    /** "sk-abc...xy12" → "sk-***xy12" 供 UI 展示。 空字符串返回空。 */
    fun maskedKey(): String {
        val k = _state.value.apiKey
        if (k.isBlank()) return ""
        if (k.length < 8) return "***"
        return k.take(3) + "***" + k.takeLast(4)
    }

    private fun loadFromSP(): Config {
        val b = sp.getString(KEY_BASE_URL, "").orEmpty()
        val m = sp.getString(KEY_MODEL, "").orEmpty()
        val k = sp.getString(KEY_API_KEY, "").orEmpty()
        return Config(b, m, k)
    }

    /**
     * 从旧 agent_apikey SharedPreferences 迁移单字段 ApiKey 到新三件套 (一次性)。
     * 用户升级安装后, 老 key 不丢, baseURL/model 留空走 Go default。
     */
    private fun migrateFromApiKeyPrefs(ctx: Context) {
        if (sp.contains(KEY_API_KEY)) return  // 已迁移过 (任意三件套字段写过即视为迁移完毕)
        val old = ctx.applicationContext
            .getSharedPreferences("agent_apikey", Context.MODE_PRIVATE)
            .getString("llm_api_key", "")
            .orEmpty()
        if (old.isNotBlank()) {
            sp.edit().putString(KEY_API_KEY, old).apply()
            _state.value = _state.value.copy(apiKey = old)
            Log.i(TAG, "migrated legacy apikey: ${old.take(3)}***${old.takeLast(4)}")
        }
    }

    companion object {
        private const val TAG = "LLMConfigPrefs"
        private const val FILE = "llm_config"
        private const val KEY_BASE_URL = "base_url"
        private const val KEY_MODEL = "model"
        private const val KEY_API_KEY = "api_key"

        @Volatile private var instance: LLMConfigPrefs? = null

        fun get(ctx: Context): LLMConfigPrefs {
            instance?.let { return it }
            return synchronized(this) {
                instance ?: LLMConfigPrefs(
                    ctx.applicationContext.getSharedPreferences(FILE, Context.MODE_PRIVATE),
                ).also {
                    it.migrateFromApiKeyPrefs(ctx)
                    instance = it
                }
            }
        }

        // 预设:三家主流 OpenAI 兼容 provider + 自定义占位
        data class Preset(val label: String, val baseURL: String, val defaultModel: String)

        val PRESETS = listOf(
            Preset("硅基流动 SiliconFlow", "https://api.siliconflow.cn/v1", "Qwen/Qwen3.6-35B-A3B"),
            Preset("阿里云 DashScope", "https://dashscope.aliyuncs.com/compatible-mode/v1", "qwen3.6-max-preview"),
            Preset("OpenRouter", "https://openrouter.ai/api/v1", "qwen/qwen3.6-plus"),
            Preset("自定义", "", ""),
        )
    }
}
