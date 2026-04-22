package com.pilotty.app.chat

import android.content.Context
import android.content.SharedPreferences
import android.util.Log
import com.pilotty.app.data.ToolEventDto
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.serialization.Serializable
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.json.Json

/**
 * Chat 消息历史持久化 (Phase 7.1)。
 *
 * UI 气泡存这里, Go 侧 ConversationHistory (LLM system prompt 摘要) 独立存在
 * `filesDir/chat_history.json`, 两边分别管自己的可见性但内容对齐 —— Kotlin 负责
 * 展示历史, Go 负责多轮上下文连续性。
 *
 * 明文 SharedPreferences, 与 ApiKeyPrefs 一致的安全债;对话内容一般不敏感, 但
 * 若用户真贴了密码/token 进 Chat, 会落盘。 v1.x 升 Encrypted 时一并处理。
 */
class ChatHistoryPrefs private constructor(private val sp: SharedPreferences) {

    @Serializable
    data class Entry(
        val role: String,
        val text: String,
        val source: String = "",
        val events: List<ToolEventDto> = emptyList(),
    )

    private val json = Json { ignoreUnknownKeys = true; encodeDefaults = true }
    private val listSerializer = ListSerializer(Entry.serializer())

    private val _state = MutableStateFlow(loadInitial())
    val state: StateFlow<List<Entry>> = _state.asStateFlow()

    private fun loadInitial(): List<Entry> = runCatching {
        val raw = sp.getString(KEY_MESSAGES, null) ?: return emptyList()
        json.decodeFromString(listSerializer, raw)
    }.onFailure { Log.w(TAG, "load failed, start fresh", it) }.getOrDefault(emptyList())

    fun save(entries: List<Entry>) {
        val trimmed = if (entries.size > MAX) entries.takeLast(MAX) else entries
        runCatching {
            val raw = json.encodeToString(listSerializer, trimmed)
            sp.edit().putString(KEY_MESSAGES, raw).apply()
            _state.value = trimmed
        }.onFailure { Log.w(TAG, "save failed", it) }
    }

    fun clear() {
        sp.edit().remove(KEY_MESSAGES).apply()
        _state.value = emptyList()
    }

    companion object {
        private const val TAG = "ChatHistoryPrefs"
        private const val FILE = "chat_history"
        private const val KEY_MESSAGES = "messages"

        /** 40 轮 Q&A = 80 条 entry, 对齐 Go ConversationHistory.maxEntries。 */
        private const val MAX = 80

        @Volatile private var INSTANCE: ChatHistoryPrefs? = null

        fun get(ctx: Context): ChatHistoryPrefs = INSTANCE ?: synchronized(this) {
            INSTANCE ?: ChatHistoryPrefs(
                ctx.applicationContext.getSharedPreferences(FILE, Context.MODE_PRIVATE),
            ).also { INSTANCE = it }
        }
    }
}
