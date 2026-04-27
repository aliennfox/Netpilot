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
 * 多会话气泡存储, 替代单会话的 [ChatHistoryPrefs] (Phase 11)。
 *
 * - 会话上限 20, 每会话消息上限 80; 超额从最旧裁掉
 * - 切换/删除/新建是 UI 侧操作; Go 侧 ConversationHistory (LLM 上下文摘要)
 *   仍是单一 active 实例, 切换会话时调 `PilottyRepository.clearHistory()` 让
 *   Agent 重新开始 — 接受"旧会话查看时 Agent 不记得"这个 trade-off,
 *   把"上下文连续性"和"会话可回看"解耦
 * - 标题取首条 user msg 截前 20 字, 空则显示 "(空会话)"
 */
class ChatSessionStore private constructor(private val sp: SharedPreferences) {

    @Serializable
    data class Entry(
        val role: String,
        val text: String,
        val source: String = "",
        val events: List<ToolEventDto> = emptyList(),
    )

    @Serializable
    data class Session(
        val id: String,
        val title: String = "",
        val createdAt: Long = System.currentTimeMillis(),
        val lastUpdatedAt: Long = System.currentTimeMillis(),
        val messages: List<Entry> = emptyList(),
    )

    private val json = Json { ignoreUnknownKeys = true; encodeDefaults = true }
    private val listSerializer = ListSerializer(Session.serializer())

    private val _sessions = MutableStateFlow(loadAll())
    val sessions: StateFlow<List<Session>> = _sessions.asStateFlow()

    private val _activeId = MutableStateFlow(initActiveId())
    val activeId: StateFlow<String> = _activeId.asStateFlow()

    private fun loadAll(): List<Session> = runCatching {
        val raw = sp.getString(KEY_SESSIONS, null) ?: return@runCatching emptyList<Session>()
        json.decodeFromString(listSerializer, raw)
    }.onFailure { Log.w(TAG, "load failed", it) }.getOrDefault(emptyList())

    private fun initActiveId(): String {
        val saved = sp.getString(KEY_ACTIVE_ID, null)
        val current = _sessions.value
        if (saved != null && current.any { it.id == saved }) return saved
        if (current.isEmpty()) {
            val s = Session(id = newId())
            persistAll(listOf(s))
            sp.edit().putString(KEY_ACTIVE_ID, s.id).apply()
            return s.id
        }
        val first = current.first().id
        sp.edit().putString(KEY_ACTIVE_ID, first).apply()
        return first
    }

    private fun persistAll(list: List<Session>) {
        runCatching {
            sp.edit().putString(KEY_SESSIONS, json.encodeToString(listSerializer, list)).apply()
            _sessions.value = list
        }.onFailure { Log.w(TAG, "save failed", it) }
    }

    /** 当前 active 会话的消息; ChatViewModel 启动时调一次 + 切换会话时再调 */
    fun activeMessages(): List<Entry> =
        _sessions.value.firstOrNull { it.id == _activeId.value }?.messages ?: emptyList()

    /** 写当前 active 会话; 自动 trim + 自动从首条 user msg 派生标题 */
    fun saveActive(messages: List<Entry>) {
        val trimmed = if (messages.size > MAX_MSGS) messages.takeLast(MAX_MSGS) else messages
        val derivedTitle = trimmed.firstOrNull { it.role == "user" }?.text
            ?.take(20)?.replace('\n', ' ')?.trim() ?: ""
        val list = _sessions.value.map {
            if (it.id == _activeId.value) it.copy(
                messages = trimmed,
                title = if (it.title.isEmpty() && derivedTitle.isNotEmpty()) derivedTitle else it.title,
                lastUpdatedAt = System.currentTimeMillis(),
            ) else it
        }
        persistAll(list)
    }

    /** 新建一个会话并切到它 */
    fun newSession() {
        val s = Session(id = newId())
        val combined = listOf(s) + _sessions.value
        val capped = if (combined.size > MAX_SESSIONS) combined.take(MAX_SESSIONS) else combined
        persistAll(capped)
        switchTo(s.id)
    }

    fun switchTo(id: String) {
        if (_sessions.value.none { it.id == id }) return
        sp.edit().putString(KEY_ACTIVE_ID, id).apply()
        _activeId.value = id
    }

    fun delete(id: String) {
        val remaining = _sessions.value.filterNot { it.id == id }
        if (remaining.isEmpty()) {
            // 删的是最后一个, 自动建一个新空会话
            val s = Session(id = newId())
            persistAll(listOf(s))
            sp.edit().putString(KEY_ACTIVE_ID, s.id).apply()
            _activeId.value = s.id
            return
        }
        persistAll(remaining)
        if (_activeId.value == id) {
            val firstId = remaining.first().id
            sp.edit().putString(KEY_ACTIVE_ID, firstId).apply()
            _activeId.value = firstId
        }
    }

    private fun newId(): String = "s${System.currentTimeMillis()}-${(0..9999).random()}"

    companion object {
        private const val TAG = "ChatSessionStore"
        private const val FILE = "chat_sessions"
        private const val KEY_SESSIONS = "sessions"
        private const val KEY_ACTIVE_ID = "active_id"
        private const val MAX_MSGS = 80
        private const val MAX_SESSIONS = 20

        @Volatile private var INSTANCE: ChatSessionStore? = null

        fun get(ctx: Context): ChatSessionStore = INSTANCE ?: synchronized(this) {
            INSTANCE ?: ChatSessionStore(
                ctx.applicationContext.getSharedPreferences(FILE, Context.MODE_PRIVATE),
            ).also { INSTANCE = it }
        }
    }
}
