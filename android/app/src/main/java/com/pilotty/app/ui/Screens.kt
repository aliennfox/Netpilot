package com.pilotty.app.ui

import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.compose.viewModel
import com.pilotty.app.chat.ChatSessionStore
import com.pilotty.app.data.*
import com.pilotty.app.ui.agent.AgentQueryBus
import com.pilotty.app.ui.components.*
import com.pilotty.app.ui.theme.LocalPilottyColors
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/* ============================= Chat ============================= */

data class ChatMessage(
    val role: String,
    val text: String,
    val source: String = "",
    val events: List<ToolEventDto> = emptyList(),
)

data class ChatUi(
    val sending: Boolean = false,
    val messages: List<ChatMessage> = emptyList(),
    val error: String? = null,
    /** Phase 7.3: 当前流式阶段 ("Diagnose"/"Configure"/"Verify"), 空串表示无阶段或已结束 */
    val streamingPhase: String = "",
)

class ChatViewModel : ViewModel() {
    // Phase 11: 多会话持久化, 替代单会话的 ChatHistoryPrefs。 Go 侧 ConversationHistory
    // (LLM 上下文摘要) 仍是单 active 实例, 切会话时清掉让 Agent 重新开始。
    private val store: ChatSessionStore? = com.pilotty.app.PilottyApp.appContext?.let { ChatSessionStore.get(it) }

    private val _state = MutableStateFlow(ChatUi(messages = loadActiveMessages()))
    val state: StateFlow<ChatUi> = _state.asStateFlow()

    val sessions: StateFlow<List<ChatSessionStore.Session>> =
        store?.sessions ?: MutableStateFlow(emptyList<ChatSessionStore.Session>()).asStateFlow()
    val activeId: StateFlow<String> =
        store?.activeId ?: MutableStateFlow<String>("").asStateFlow()

    // 顶栏副标实时显示 VPN 状态: tunRunning 变化触发 instant 刷, 8s ticker 兜底节点切换
    private val _vpnHint = MutableStateFlow("VPN 未启动")
    val vpnHint: StateFlow<String> = _vpnHint.asStateFlow()

    // 流式 job 句柄, 用于 cancel(). collect 取消会触发 PilottyRepository.chatStream 内部
    // ChatStreamHandle.cancel() (#M26), Go ctx.cancel → SSE 断 → LLM token 不再白烧
    private var streamJob: Job? = null

    init {
        viewModelScope.launch {
            com.pilotty.app.PilottyCore.tunRunning.collect { refreshVpnHint(it) }
        }
        viewModelScope.launch {
            while (true) {
                delay(8_000)
                refreshVpnHint(com.pilotty.app.PilottyCore.tunRunning.value)
            }
        }
    }

    private suspend fun refreshVpnHint(running: Boolean) {
        if (!running) {
            _vpnHint.value = "VPN 未启动"
            return
        }
        runCatching { PilottyRepository.status() }
            .onSuccess { s ->
                _vpnHint.value = if (s.currentNode.isNotEmpty()) "已连接 · ${s.currentNode}"
                else "VPN 已启动 · 未选节点"
            }
            .onFailure { _vpnHint.value = "VPN 已启动" }
    }

    private fun loadActiveMessages(): List<ChatMessage> =
        store?.activeMessages()?.map { ChatMessage(it.role, it.text, it.source, it.events) } ?: emptyList()

    private fun persist(messages: List<ChatMessage>) {
        store?.saveActive(messages.map { ChatSessionStore.Entry(it.role, it.text, it.source, it.events) })
    }

    fun newSession() {
        streamJob?.cancel()
        streamJob = null
        runCatching { PilottyRepository.clearHistory() }
        store?.newSession()
        _state.value = ChatUi()
    }

    fun switchSession(id: String) {
        if (id == store?.activeId?.value) return
        streamJob?.cancel()
        streamJob = null
        store?.switchTo(id)
        val msgs = loadActiveMessages()
        _state.value = ChatUi(messages = msgs)
        // 灌回该会话上下文到 Go 侧 history → Agent 重新"记得"该会话之前的对话
        runCatching {
            PilottyRepository.restoreHistory(
                msgs.map { Triple(it.role, it.text, it.source.ifEmpty { "agent" }) }
            )
        }
    }

    fun deleteSession(id: String) {
        val wasActive = id == store?.activeId?.value
        store?.delete(id)
        if (wasActive) {
            streamJob?.cancel()
            streamJob = null
            runCatching { PilottyRepository.clearHistory() }
            _state.value = ChatUi(messages = loadActiveMessages())
        }
    }

    /**
     * Phase 7.3 + 8: 流式发送, 全部走 Go Agent。 VPN 生命周期通过 Agent 的 start_vpn/stop_vpn/
     * vpn_status tools 掌控 (Phase 8, 对齐 "Agent 全局掌控" 原则), 不在 Kotlin 层做关键词拦截。
     * TextDelta 涌出时逐字追加到末尾 assistant 气泡, Tool 事件实时 append 到 events 列表,
     * Done 终结并持久化。 本地 IntentRouter 命中 (source=local) 没 stream, 直接跳 Done 事件。
     */
    // helpText 从 Composable 层 stringResource 注入, VM 无 Context 拿不到 R.string
    fun send(text: String, helpText: String = "") {
        if (text.isBlank()) return
        val trimmed = text.trim()
        // D Agent 彻查: 拦截 "/" 前缀命令, 避免被 Agent 或 LocalEngine 当成普通消息发给 LLM
        // (原先输入 "/clear abc" 会被 LLM 当用户消息处理, 返回莫名其妙的回答)。
        when {
            trimmed == "/clear" || trimmed == "/new" -> {
                clear()
                return
            }
            trimmed == "/help" -> {
                _state.value = _state.value.copy(
                    messages = _state.value.messages + ChatMessage(
                        role = "assistant",
                        text = helpText,
                        source = "local",
                    ),
                )
                return
            }
        }
        val u = ChatMessage("user", text)
        _state.value = _state.value.copy(
            sending = true,
            messages = _state.value.messages + u,
            error = null,
            streamingPhase = "",
        )
        streamJob?.cancel()
        streamJob = viewModelScope.launch {
            // 占位 assistant 气泡: text 空, 随 TextDelta 涌出
            var pending = ChatMessage(role = "assistant", text = "", source = "", events = emptyList())
            var placedPending = false

            fun replacePending(updater: (ChatMessage) -> ChatMessage) {
                pending = updater(pending)
                val msgs = _state.value.messages.toMutableList()
                if (!placedPending) {
                    msgs.add(pending)
                    placedPending = true
                } else {
                    msgs[msgs.lastIndex] = pending
                }
                _state.value = _state.value.copy(messages = msgs)
            }

            try {
                PilottyRepository.chatStream(text).collect { event ->
                    when (event) {
                        is ChatStreamEvent.PhaseStart -> {
                            _state.value = _state.value.copy(streamingPhase = event.role)
                        }
                        is ChatStreamEvent.PhaseEnd -> {
                            // 不清 phase; 下一个 PhaseStart 会覆盖; Done 最终清空
                        }
                        is ChatStreamEvent.TextDelta -> {
                            replacePending { it.copy(text = it.text + event.delta) }
                        }
                        is ChatStreamEvent.ToolStart -> {
                            replacePending {
                                it.copy(events = it.events + ToolEventDto(
                                    name = event.name,
                                    argsSummary = event.argsSummary,
                                    role = event.role,
                                ))
                            }
                        }
                        is ChatStreamEvent.ToolEnd -> {
                            // 更新最后一个同名 running 条目 (最常见: ToolStart 后紧跟 ToolEnd)
                            replacePending { pm ->
                                val newEvents = pm.events.toMutableList()
                                val idx = newEvents.indexOfLast { it.name == event.name && it.durationMs == 0L && it.error.isBlank() && it.outputPreview.isBlank() }
                                val full = ToolEventDto(
                                    name = event.name,
                                    argsSummary = event.argsSummary,
                                    durationMs = event.durationMs,
                                    outputPreview = event.outputPreview,
                                    error = event.error,
                                    role = event.role,
                                )
                                if (idx >= 0) newEvents[idx] = full else newEvents.add(full)
                                pm.copy(events = newEvents)
                            }
                        }
                        is ChatStreamEvent.Done -> {
                            // 用 reply 作权威来源覆盖 pending.text (多阶段 pipeline 的 formatFinalResponse
                            // 会加 "📋 诊断报告:" 等 section 分隔, 单阶段 reply == 累积的 TextDelta)
                            replacePending { it.copy(text = event.reply, source = event.source, events = event.events) }
                            _state.value = _state.value.copy(sending = false, streamingPhase = "")
                            persist(_state.value.messages)
                        }
                        is ChatStreamEvent.Error -> {
                            _state.value = _state.value.copy(sending = false, error = event.message, streamingPhase = "")
                            // 即使出错, 也把 pending 气泡(如果有 text 已经显示)保留
                            if (placedPending && pending.text.isBlank()) {
                                // 完全没 delta 就移除占位, 避免空白气泡
                                val msgs = _state.value.messages.toMutableList()
                                if (msgs.isNotEmpty() && msgs.last() === pending) msgs.removeAt(msgs.lastIndex)
                                _state.value = _state.value.copy(messages = msgs)
                            }
                            persist(_state.value.messages)
                        }
                    }
                }
            } catch (e: CancellationException) {
                // 用户主动 cancel: silent, 保留已收到的 delta
                _state.value = _state.value.copy(sending = false, streamingPhase = "")
                persist(_state.value.messages)
                throw e
            } catch (e: Throwable) {
                _state.value = _state.value.copy(sending = false, error = e.message, streamingPhase = "")
                persist(_state.value.messages)
            }
        }
    }

    /** 流式中按 ⏹ 中断: 取消 streamJob → collect 终止 → callbackFlow.awaitClose → handle.cancel() (#M26 链路) */
    fun cancel() {
        streamJob?.cancel()
        streamJob = null
        _state.value = _state.value.copy(sending = false, streamingPhase = "", error = null)
    }

    /** 兼容 /clear 命令: 等同新建会话 (旧会话仍保留在抽屉里, 不再 destructive 清空) */
    fun clear() {
        newSession()
    }
}

/**
 * sending=true 时的占位气泡 (Phase 7.2)。 解决用户反馈 "发了之后没反应" —— 非流式下
 * LLM 往返 5-10s 完全黑屏, 加一个 assistant-side 气泡 + 3 点波浪动画, 让用户看到 "正在工作"。
 * Phase 7.3 流式上线后本气泡会被"逐字涌出的空气泡"替代, 但两者并存也 OK。
 */
@Composable
private fun PendingBubble(phase: String = "") {
    val pc = LocalPilottyColors.current
    val transition = rememberInfiniteTransition(label = "pending-dots")
    val t by transition.animateFloat(
        initialValue = 0f,
        targetValue = 3f,
        animationSpec = infiniteRepeatable(
            animation = tween(900, easing = LinearEasing),
            repeatMode = RepeatMode.Restart,
        ),
        label = "dot-phase",
    )
    // 阶段标签: Phase 7.3 期间 RunStream 的 OnPhaseStart 提供, 空串时显示 "正在思考" 兜底
    val label = when (phase) {
        "Diagnose" -> androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.chat_stage_diagnose)
        "Configure" -> androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.chat_stage_configure)
        "Verify" -> androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.chat_stage_verify)
        else -> androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.chat_stage_thinking)
    }
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.Start,
    ) {
        Surface(
            color = pc.surface2,
            shape = RoundedCornerShape(16.dp, 16.dp, 16.dp, 4.dp),
        ) {
            Row(
                modifier = Modifier.padding(horizontal = 13.dp, vertical = 11.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(5.dp),
            ) {
                Text(
                    label,
                    color = pc.ink3,
                    fontSize = 13.sp,
                    fontFamily = FontFamily.Monospace,
                    letterSpacing = 0.3.sp,
                )
                val active = t.toInt().coerceIn(0, 2)
                repeat(3) { idx ->
                    Box(
                        modifier = Modifier
                            .size(5.dp)
                            .clip(RoundedCornerShape(50))
                            .background(if (idx == active) pc.ink else pc.ink4),
                    )
                }
            }
        }
    }
}

/**
 * D3 Tool-Call Timeline: 在 agent 回复气泡下方渲染一行折叠头, 点开展开 per-tool 详情。
 * local 来源 events 为空 → 不渲染 (优雅降级: 只显示 "via local")。
 */
@Composable
private fun ToolCallTimeline(events: List<ToolEventDto>) {
    if (events.isEmpty()) return
    val pc = LocalPilottyColors.current
    var expanded by remember { mutableStateOf(false) }
    val totalMs = events.sumOf { it.durationMs }
    val failedCount = events.count { it.error.isNotEmpty() }
    Column(modifier = Modifier.padding(top = 6.dp)) {
        Row(
            modifier = Modifier
                .clip(RoundedCornerShape(6.dp))
                .clickable { expanded = !expanded }
                .padding(vertical = 3.dp, horizontal = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(5.dp),
        ) {
            Text(
                if (expanded) "▾" else "▸",
                color = pc.ink4,
                fontSize = 10.sp,
                fontFamily = FontFamily.Monospace,
            )
            val summary = androidx.compose.ui.res.stringResource(
                com.pilotty.app.R.string.chat_tool_calls_summary_format,
                events.size,
                formatDurationMs(totalMs),
            )
            val failedSuffix = if (failedCount > 0)
                androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.chat_tool_calls_failed_format, failedCount)
            else ""
            Text(
                summary + failedSuffix,
                color = if (failedCount > 0) pc.error else pc.ink3,
                fontSize = 11.sp,
                fontFamily = FontFamily.Monospace,
            )
        }
        if (expanded) {
            Column(
                modifier = Modifier.padding(start = 12.dp, top = 4.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                events.forEach { e ->
                    // running = 还没收到 ToolEnd: 占位事件, durationMs=0 + outputPreview/error 都空
                    val running = e.durationMs == 0L && e.outputPreview.isEmpty() && e.error.isEmpty()
                    val ok = e.error.isEmpty()
                    val badge = when {
                        running -> "▸"
                        ok -> "✓"
                        else -> "✗"
                    }
                    val badgeColor = when {
                        running -> pc.ink3
                        ok -> pc.accentInk
                        else -> pc.error
                    }
                    val displayName = friendlyToolName(e.name)
                    val runningSuffix = if (running)
                        " · " + androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.chat_tool_running)
                    else ""
                    val detail = buildString {
                        append(displayName)
                        if (e.role.isNotEmpty()) append(" · ").append(e.role)
                        if (running) {
                            append(runningSuffix)
                        } else {
                            append(" · ").append(formatDurationMs(e.durationMs))
                            val extra = if (ok) e.outputPreview else e.error
                            if (extra.isNotEmpty()) {
                                append("\n").append(extra.take(140))
                                if (extra.length > 140) append("…")
                            }
                        }
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(
                            badge,
                            color = badgeColor,
                            fontSize = 11.sp,
                            fontFamily = FontFamily.Monospace,
                        )
                        Text(
                            detail,
                            color = if (ok) pc.ink3 else pc.error,
                            fontSize = 10.5.sp,
                            fontFamily = FontFamily.Monospace,
                            lineHeight = 14.sp,
                        )
                    }
                }
            }
        }
    }
}

private fun formatDurationMs(ms: Long): String = when {
    ms <= 0 -> "0ms"
    ms < 1000 -> "${ms}ms"
    ms < 10_000 -> "%.1fs".format(ms / 1000.0)
    else -> "${ms / 1000}s"
}

@Composable
fun ChatScreen(
    onNavigateAgentTrace: () -> Unit = {},
    vm: ChatViewModel = viewModel(),
) {
    val pc = LocalPilottyColors.current
    val ui by vm.state.collectAsStateWithLifecycle()
    val vpnHint by vm.vpnHint.collectAsStateWithLifecycle()
    val sessions by vm.sessions.collectAsStateWithLifecycle()
    val activeId by vm.activeId.collectAsStateWithLifecycle()
    var input by remember { mutableStateOf("") }
    val listState = rememberLazyListState()
    val helpText = androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.chat_help_message)

    val drawerState = androidx.compose.material3.rememberDrawerState(
        initialValue = androidx.compose.material3.DrawerValue.Closed,
    )
    val scope = androidx.compose.runtime.rememberCoroutineScope()
    var pendingDeleteId by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(ui.messages.size) {
        if (ui.messages.isNotEmpty()) listState.animateScrollToItem(ui.messages.size - 1)
    }

    val pendingQuery by AgentQueryBus.pending.collectAsStateWithLifecycle()
    LaunchedEffect(pendingQuery) {
        pendingQuery?.let {
            vm.send(it, helpText)
            AgentQueryBus.consume()
        }
    }

    androidx.compose.material3.ModalNavigationDrawer(
        drawerState = drawerState,
        drawerContent = {
            ChatSessionsDrawer(
                sessions = sessions,
                activeId = activeId,
                onSelect = { id ->
                    vm.switchSession(id)
                    scope.launch { drawerState.close() }
                },
                onNew = {
                    vm.newSession()
                    scope.launch { drawerState.close() }
                },
                onLongPress = { id -> pendingDeleteId = id },
                onOpenAgentTrace = {
                    scope.launch { drawerState.close() }
                    onNavigateAgentTrace()
                },
            )
        },
    ) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(pc.bg),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 10.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                modifier = Modifier.weight(1f),
            ) {
                // ≡ 打开会话抽屉
                Text(
                    "≡",
                    color = pc.ink,
                    fontSize = 22.sp,
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier
                        .clickable { scope.launch { drawerState.open() } }
                        .padding(horizontal = 4.dp, vertical = 2.dp),
                )
                Column {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        Text("Agent", color = pc.ink, fontSize = 16.sp, fontWeight = FontWeight.SemiBold, letterSpacing = (-0.16).sp)
                        if (ui.sending) LiveDot()
                    }
                    Text(
                        vpnHint,
                        color = pc.ink3,
                        fontSize = 10.5.sp,
                        fontFamily = FontFamily.Monospace,
                        letterSpacing = 0.2.sp,
                    )
                }
            }
            Surface(
                color = pc.surface2,
                shape = RoundedCornerShape(999.dp),
                onClick = { vm.newSession() },
            ) {
                Row(
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    Text("+", color = pc.ink, fontSize = 14.sp, fontWeight = FontWeight.Bold)
                    Text(androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.chat_new_session), color = pc.ink, fontSize = 12.sp, fontWeight = FontWeight.Medium)
                }
            }
        }
        HorizontalDivider(color = pc.hairline, thickness = 1.dp)

        LazyColumn(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
                .padding(horizontal = 20.dp),
            state = listState,
            verticalArrangement = Arrangement.spacedBy(14.dp),
            contentPadding = PaddingValues(vertical = 16.dp),
        ) {
            // 空态 quick query 卡: 给用户起步, 三条 Pilotty 高频意图
            if (ui.messages.isEmpty() && !ui.sending) {
                item {
                    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        Spacer(Modifier.height(8.dp))
                        Text(
                            "试试这些",
                            color = pc.ink3,
                            fontSize = 11.sp,
                            fontFamily = FontFamily.Monospace,
                            letterSpacing = 1.2.sp,
                        )
                        listOf(
                            "VPN 现在是什么状态？",
                            "诊断网络为什么连不上",
                            "切到延迟最低的节点",
                        ).forEach { q ->
                            QuickQueryCard(text = q) { vm.send(q, helpText) }
                        }
                    }
                }
            }
            items(ui.messages) { msg ->
                val isUser = msg.role == "user"
                if (isUser) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.End,
                    ) {
                        Surface(
                            color = pc.ink,
                            shape = RoundedCornerShape(16.dp, 16.dp, 4.dp, 16.dp),
                        ) {
                            Text(
                                msg.text,
                                color = pc.bg,
                                fontSize = 14.sp,
                                lineHeight = 20.sp,
                                letterSpacing = (-0.07).sp,
                                modifier = Modifier.padding(horizontal = 13.dp, vertical = 9.dp),
                            )
                        }
                    }
                } else {
                    // Mission Control 风格: 小 26dp accent 方块头像 + 纯 SansSerif 正文 + tool trace card
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.Start,
                    ) {
                        Box(
                            modifier = Modifier
                                .padding(top = 2.dp)
                                .size(26.dp)
                                .clip(MaterialTheme.shapes.small)
                                .background(pc.accent),
                            contentAlignment = Alignment.Center,
                        ) {
                            Text("◢", color = pc.accentOnBg, fontSize = 13.sp, fontWeight = FontWeight.Bold)
                        }
                        Spacer(Modifier.width(10.dp))
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                msg.text,
                                color = pc.ink,
                                fontSize = 14.sp,
                                lineHeight = 21.sp,
                                letterSpacing = (-0.07).sp,
                            )
                            if (msg.source.isNotEmpty()) {
                                Spacer(Modifier.height(6.dp))
                                val localSuffix = if (msg.source == "local")
                                    androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.chat_via_source_local_suffix)
                                else ""
                                Text(
                                    "via ${msg.source}$localSuffix",
                                    color = pc.ink4,
                                    fontSize = 10.sp,
                                    fontFamily = FontFamily.Monospace,
                                )
                                ToolCallTimeline(msg.events)
                            }
                        }
                    }
                }
            }
            // sending 占位气泡 (Phase 7.2 + 7.3): 流式下首个 TextDelta 到达前显示;
            // 之后 ChatViewModel 会添加空 assistant 气泡, 此气泡自动让位。
            if (ui.sending && ui.messages.lastOrNull()?.role != "assistant") {
                item { PendingBubble(phase = ui.streamingPhase) }
            }

            // 错误气泡: 不再用顶部红色 banner 遮挡消息, 改成 assistant 样式靠左的红色描边气泡,
            // 跟随消息一起滚动 (Phase 4 修复: 原先在 LazyColumn 上方导致消息被顶下去)
            ui.error?.let { errText ->
                item {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.Start,
                    ) {
                        Surface(
                            color = pc.surface2,
                            shape = RoundedCornerShape(16.dp, 16.dp, 16.dp, 4.dp),
                            border = androidx.compose.foundation.BorderStroke(1.dp, pc.error),
                        ) {
                            Column(Modifier.padding(horizontal = 13.dp, vertical = 9.dp)) {
                                Text(
                                    "✗ 出错了",
                                    color = pc.error,
                                    fontSize = 11.sp,
                                    fontFamily = FontFamily.Monospace,
                                    fontWeight = FontWeight.Bold,
                                )
                                Spacer(Modifier.height(4.dp))
                                Text(
                                    errText,
                                    color = pc.error,
                                    fontSize = 13.sp,
                                    lineHeight = 18.sp,
                                )
                            }
                        }
                    }
                }
            }
        }

        HorizontalDivider(color = pc.hairline, thickness = 1.dp)
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(pc.bg)
                .padding(horizontal = 16.dp, vertical = 10.dp),
        ) {
            Surface(
                color = pc.surface,
                shape = MaterialTheme.shapes.large,
                shadowElevation = if (pc.isDark) 0.dp else 4.dp,
                border = if (pc.isDark) androidx.compose.foundation.BorderStroke(1.dp, pc.hairline) else null,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Row(
                    modifier = Modifier.padding(start = 14.dp, end = 10.dp, top = 10.dp, bottom = 10.dp),
                    verticalAlignment = Alignment.Bottom,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Box(modifier = Modifier.weight(1f).heightIn(min = 24.dp, max = 120.dp)) {
                        BasicTextField(
                            value = input,
                            onValueChange = { input = it },
                            modifier = Modifier.fillMaxWidth(),
                            textStyle = TextStyle(
                                color = pc.ink,
                                fontSize = 14.5.sp,
                                lineHeight = 21.sp,
                                letterSpacing = (-0.07).sp,
                            ),
                            cursorBrush = SolidColor(pc.ink),
                            maxLines = 6,
                            enabled = !ui.sending,
                        )
                        if (input.isEmpty()) {
                            Text(
                                androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.chat_reply_hint),
                                color = pc.ink3,
                                fontSize = 14.5.sp,
                                lineHeight = 21.sp,
                            )
                        }
                    }
                    // 状态机: sending → ⏹ accent (可点 cancel); 有输入 → ↑ accent (send); 空 → 灰
                    val canAct = ui.sending || input.isNotBlank()
                    Box(
                        modifier = Modifier
                            .size(36.dp)
                            .clip(MaterialTheme.shapes.medium)
                            .background(if (canAct) pc.accent else pc.surface2)
                            .clickable(enabled = canAct) {
                                if (ui.sending) {
                                    vm.cancel()
                                } else {
                                    vm.send(input, helpText)
                                    input = ""
                                }
                            },
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(
                            if (ui.sending) "■" else "↑",
                            color = if (canAct) pc.accentOnBg else pc.ink3,
                            fontSize = if (ui.sending) 13.sp else 16.sp,
                            fontWeight = FontWeight.Bold,
                        )
                    }
                }
            }
        }
    }
    } // ModalNavigationDrawer trailing lambda close

    // 删除会话二次确认
    pendingDeleteId?.let { id ->
        val target = sessions.firstOrNull { it.id == id }
        androidx.compose.material3.AlertDialog(
            onDismissRequest = { pendingDeleteId = null },
            title = { Text("删除会话?") },
            text = {
                Text(
                    "确定删除 \"${target?.title?.ifEmpty { "(空会话)" } ?: "会话"}\" 吗? 此操作不可撤销。",
                    color = pc.ink2,
                    fontSize = 13.sp,
                )
            },
            confirmButton = {
                androidx.compose.material3.TextButton(onClick = {
                    vm.deleteSession(id)
                    pendingDeleteId = null
                }) { Text("删除", color = pc.error) }
            },
            dismissButton = {
                androidx.compose.material3.TextButton(onClick = { pendingDeleteId = null }) {
                    Text("取消")
                }
            },
        )
    }
}

@OptIn(androidx.compose.foundation.ExperimentalFoundationApi::class)
@Composable
private fun ChatSessionsDrawer(
    sessions: List<com.pilotty.app.chat.ChatSessionStore.Session>,
    activeId: String,
    onSelect: (String) -> Unit,
    onNew: () -> Unit,
    onLongPress: (String) -> Unit,
    onOpenAgentTrace: () -> Unit,
) {
    val pc = LocalPilottyColors.current
    androidx.compose.material3.ModalDrawerSheet(
        drawerContainerColor = pc.surface,
        modifier = Modifier.fillMaxHeight(),
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = 12.dp, vertical = 16.dp),
        ) {
            Text(
                "会话",
                color = pc.ink,
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                modifier = Modifier.padding(start = 8.dp, bottom = 12.dp),
            )

            Surface(
                color = pc.surface2,
                shape = MaterialTheme.shapes.medium,
                onClick = onNew,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Row(
                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Text("+", color = pc.accent, fontSize = 18.sp, fontWeight = FontWeight.Bold)
                    Text("新会话", color = pc.ink, fontSize = 14.sp, fontWeight = FontWeight.Medium)
                }
            }

            Spacer(Modifier.height(16.dp))

            if (sessions.isEmpty()) {
                Text("暂无会话", color = pc.ink3, fontSize = 12.sp, modifier = Modifier.padding(8.dp))
            } else {
                LazyColumn(
                    verticalArrangement = Arrangement.spacedBy(4.dp),
                    modifier = Modifier.weight(1f),
                ) {
                    items(sessions, key = { it.id }) { s ->
                        val isActive = s.id == activeId
                        Surface(
                            color = if (isActive) pc.surface3 else pc.surface,
                            shape = MaterialTheme.shapes.medium,
                            modifier = Modifier
                                .fillMaxWidth()
                                .combinedClickable(
                                    onClick = { onSelect(s.id) },
                                    onLongClick = { onLongPress(s.id) },
                                ),
                        ) {
                            Row(
                                modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                            ) {
                                Column(modifier = Modifier.weight(1f)) {
                                    Text(
                                        s.title.ifEmpty { "(空会话)" },
                                        color = if (isActive) pc.ink else pc.ink2,
                                        fontSize = 13.sp,
                                        fontWeight = if (isActive) FontWeight.SemiBold else FontWeight.Normal,
                                        maxLines = 1,
                                    )
                                    Text(
                                        formatRelativeTime(s.lastUpdatedAt),
                                        color = pc.ink3,
                                        fontSize = 10.sp,
                                        fontFamily = FontFamily.Monospace,
                                    )
                                }
                                Text(
                                    "${s.messages.size}",
                                    color = pc.ink3,
                                    fontSize = 10.sp,
                                    fontFamily = FontFamily.Monospace,
                                )
                            }
                        }
                    }
                }
            }

            HorizontalDivider(color = pc.hairline, modifier = Modifier.padding(vertical = 8.dp))
            Surface(
                color = pc.surface,
                shape = MaterialTheme.shapes.medium,
                onClick = onOpenAgentTrace,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Row(
                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.SpaceBetween,
                ) {
                    Text("Agent 最近活动", color = pc.ink, fontSize = 13.sp)
                    Text("›", color = pc.ink3, fontSize = 16.sp, fontWeight = FontWeight.Bold)
                }
            }
            Text(
                "长按会话条目可删除 · 切到旧会话时 Agent 不再记得当时的上下文",
                color = pc.ink4,
                fontSize = 9.5.sp,
                fontFamily = FontFamily.Monospace,
                modifier = Modifier.padding(horizontal = 8.dp, vertical = 6.dp),
            )
        }
    }
}

private fun formatRelativeTime(ts: Long): String {
    val now = System.currentTimeMillis()
    val diff = (now - ts) / 1000
    return when {
        diff < 60 -> "刚刚"
        diff < 3600 -> "${diff / 60}m ago"
        diff < 86400 -> "${diff / 3600}h ago"
        diff < 86400 * 7 -> "${diff / 86400}d ago"
        else -> {
            val cal = java.util.Calendar.getInstance().apply { timeInMillis = ts }
            "%02d-%02d".format(cal.get(java.util.Calendar.MONTH) + 1, cal.get(java.util.Calendar.DAY_OF_MONTH))
        }
    }
}

@Composable
private fun QuickQueryCard(text: String, onClick: () -> Unit) {
    val pc = LocalPilottyColors.current
    Surface(
        color = pc.surface,
        shape = MaterialTheme.shapes.large,
        border = androidx.compose.foundation.BorderStroke(1.dp, pc.hairline),
        onClick = onClick,
        modifier = Modifier.fillMaxWidth(),
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 14.dp, vertical = 13.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Text(text, color = pc.ink, fontSize = 14.sp, lineHeight = 20.sp)
            Text("→", color = pc.ink3, fontSize = 16.sp, fontWeight = FontWeight.Bold)
        }
    }
}

/* ============================= Nodes ============================= */

data class NodesUi(
    val loading: Boolean = false,
    val nodes: List<NodeDto> = emptyList(),
    val query: String = "",
    val error: String? = null,
    val toast: String? = null,
    /** A1 测速矩阵: 正在测的 tag 集合, UI 以此切换 NodeCard 到 "测试中…" 态 */
    val testing: Set<String> = emptySet(),
    /** A1 测速矩阵: 本轮测完后延迟最低的 tag, UI 打 "最快" 标 + accent 描边 */
    val fastestTag: String? = null,
)

class NodesViewModel : ViewModel() {
    private val _state = MutableStateFlow(NodesUi())
    val state: StateFlow<NodesUi> = _state.asStateFlow()
    @Volatile private var autoTestedOnce = false

    init { refresh() }

    fun refresh() = viewModelScope.launch {
        _state.value = _state.value.copy(loading = true, error = null)
        try {
            val n = PilottyRepository.nodes()
            _state.value = _state.value.copy(loading = false, nodes = n)
            // 首次打开且 VPN 已在运行 + 所有节点都未测 → 自动测一次。 VPN 未运行时不自动测,
             // 否则会静默拉起 VPN (系统 consent 对话框会突然弹出), 属于预期外副作用。
            if (!autoTestedOnce && n.isNotEmpty() && n.all { it.latency == 0 }
                && com.pilotty.app.PilottyCore.tunRunning.value) {
                autoTestedOnce = true
                testAll()
            }
        } catch (e: Throwable) {
            _state.value = _state.value.copy(loading = false, error = e.message)
        }
    }

    fun setQuery(q: String) { _state.value = _state.value.copy(query = q) }

    // i18n: Composable 层用 stringResource 覆盖, VM 无 Context 拿不到 R.string
    var needVpnSwitchMsg: String = "请先启动 VPN 再切换节点"

    fun switchTo(tag: String) = viewModelScope.launch {
        // Clash API 要求 VPN 运行, 否则 switchNode 会撞 connection refused。
        if (!com.pilotty.app.PilottyCore.tunRunning.value) {
            _state.value = _state.value.copy(toast = needVpnSwitchMsg)
            return@launch
        }
        try { PilottyRepository.switchNode("proxy-group", tag); refresh() }
        catch (e: Throwable) { _state.value = _state.value.copy(error = e.message) }
    }

    /**
     * 测速。 统一走 Clash API URL 测试 (过代理, 延迟反映真实体验)。 若 VPN 未启动,
     * 自动请求启动 VPN → 等到 libbox ready → 再测。 这样用户不用手动先开 VPN, 得到的
     * 数字也与 VPN 开启后测量一致 (避免 TCP ping 数字失真引起困惑)。
     *
     * latency 三态:
     *   >0: 成功 (ms)
     *    0: 未测 (初始值, 或 Clash API 没回 history)
     *   -1: 此字段目前不再使用, 保留语义向前兼容
     */
    fun testAll() = viewModelScope.launch {
        try {
            if (!com.pilotty.app.PilottyCore.tunRunning.value) {
                _state.value = _state.value.copy(toast = "正在启动 VPN 以测速...")
                com.pilotty.app.vpn.StartVpnBus.request()
                // 等 tunRunning 变 true, 最多 15s。 libbox 首次冷启 5-10s, 加 TUN establish
                // 可能再 1-2s, 15s 留安全边际。
                val started = kotlinx.coroutines.withTimeoutOrNull(15_000) {
                    com.pilotty.app.PilottyCore.tunRunning.first { it }
                }
                if (started != true) {
                    _state.value = _state.value.copy(
                        toast = "VPN 启动超时, 测速取消。 请手动启动后重试。",
                    )
                    return@launch
                }
                // Clash API server 在 libbox 起来的同时暴露, 但首次 /proxies 可能还没填好节点
                // 状态缓存, 等 1.5s 再发测速请求以免拿到部分 NaN 结果。
                kotlinx.coroutines.delay(1_500)
            }

            // A1 测速矩阵: 不再用 test_latency_all 一次性阻塞, 改为客户端 fan-out
            // 并发调单节点 test_latency, 每个节点独立汇报 "完成", UI 能看到进度。
            val tags = _state.value.nodes.map { it.tag }
            if (tags.isEmpty()) return@launch
            _state.value = _state.value.copy(testing = tags.toSet(), fastestTag = null)

            coroutineScope {
                tags.map { tag ->
                    async {
                        try { PilottyRepository.testLatency(tag) }
                        catch (_: Throwable) { /* 单点失败不阻塞整体, 下一轮 nodes() 刷新 latency=-1 */ }
                        _state.update { it.copy(testing = it.testing - tag) }
                    }
                }.awaitAll()
            }

            // 全部测完统一拉一次最新 latency, 并标出 fastest
            val n2 = PilottyRepository.nodes()
            val fastest = n2.filter { it.latency > 0 }.minByOrNull { it.latency }?.tag
            _state.value = _state.value.copy(
                nodes = n2,
                testing = emptySet(),
                fastestTag = fastest,
            )
        } catch (e: Throwable) {
            _state.value = _state.value.copy(testing = emptySet(), error = e.message)
        }
    }

    fun dismissToast() { _state.value = _state.value.copy(toast = null) }

    /**
     * Phase 10-F-2: 单节点测速。 长按节点卡触发 —— 只测这一个, 不折腾别人。
     * VPN 未启动时不自动拉, 直接提示用户 (单节点测速多数是"验证刚切的节点是否恢复", VPN
     * 此时通常已在跑, 不用像 testAll 那样做 auto-start 流程)。
     */
    fun testOne(tag: String) = viewModelScope.launch {
        if (!com.pilotty.app.PilottyCore.tunRunning.value) {
            _state.value = _state.value.copy(toast = "VPN 未运行, 单节点测速需要先开 VPN")
            return@launch
        }
        _state.value = _state.value.copy(toast = "正在测 $tag ...")
        try {
            PilottyRepository.testLatency(tag)
            // 测完刷新节点列表拿最新 latency
            val n2 = PilottyRepository.nodes()
            _state.value = _state.value.copy(nodes = n2, toast = "$tag 测速完成")
        } catch (e: Throwable) {
            _state.value = _state.value.copy(toast = "测速失败: ${e.message}")
        }
    }
}

@Composable
fun NodesScreen(
    onNavigateSubs: () -> Unit = {},
    onOpenDetail: (String) -> Unit = {},
    vm: NodesViewModel = viewModel(),
) {
    val pc = LocalPilottyColors.current
    val ui by vm.state.collectAsStateWithLifecycle()
    // i18n: VM 无 Context, 在 Compose 层用 stringResource 回填
    vm.needVpnSwitchMsg = androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.nodes_need_vpn_to_switch)
    val filtered = remember(ui.nodes, ui.query) {
        // 排序: 成功 (>0 ms 升序) → 未测 (0) → 失败 (-1)。 失败排最后方便用户忽略。
        val sorted = ui.nodes.sortedWith(compareBy {
            when {
                it.latency > 0 -> it.latency
                it.latency == 0 -> Int.MAX_VALUE - 1
                else -> Int.MAX_VALUE
            }
        })
        if (ui.query.isBlank()) sorted
        else sorted.filter {
            it.tag.contains(ui.query, true) || it.server.contains(ui.query, true)
        }
    }
    // toast 自动消失
    ui.toast?.let {
        LaunchedEffect(it) {
            kotlinx.coroutines.delay(2500)
            vm.dismissToast()
        }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(pc.bg),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp, vertical = 12.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column {
                Text(
                    androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.nodes_title),
                    color = pc.ink,
                    fontSize = 22.sp,
                    fontWeight = FontWeight.Bold,
                    letterSpacing = (-0.44).sp,
                )
                Text(
                    androidx.compose.ui.res.stringResource(
                        com.pilotty.app.R.string.nodes_subtitle_format,
                        filtered.size,
                        ui.nodes.size,
                    ),
                    color = pc.ink3,
                    fontSize = 10.5.sp,
                    fontFamily = FontFamily.Monospace,
                    letterSpacing = 0.2.sp,
                )
            }
            PilottyButton(
                text = androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.nodes_test_all),
                onClick = { vm.testAll() },
                variant = PilottyButtonVariant.Mono,
            )
        }

        // 搜索栏 — Surface + shadow, 不再是 border outline
        Surface(
            color = pc.surface,
            shape = MaterialTheme.shapes.medium,
            shadowElevation = if (pc.isDark) 0.dp else 4.dp,
            border = if (pc.isDark) androidx.compose.foundation.BorderStroke(1.dp, pc.hairline) else null,
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp),
        ) {
            Row(
                modifier = Modifier.padding(horizontal = 12.dp).height(40.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Text("⌕", color = pc.ink3, fontSize = 14.sp)
                Box(modifier = Modifier.fillMaxWidth()) {
                    BasicTextField(
                        value = ui.query,
                        onValueChange = { vm.setQuery(it) },
                        textStyle = TextStyle(
                            color = pc.ink,
                            fontSize = 13.5.sp,
                            letterSpacing = (-0.05).sp,
                        ),
                        cursorBrush = SolidColor(pc.ink),
                        modifier = Modifier.fillMaxWidth(),
                        singleLine = true,
                    )
                    if (ui.query.isEmpty()) {
                        Text(androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.nodes_search_hint), color = pc.ink3, fontSize = 13.5.sp)
                    }
                }
            }
        }

        Spacer(Modifier.height(10.dp))

        ui.error?.let {
            Surface(
                color = pc.surface2,
                shape = MaterialTheme.shapes.medium,
                border = androidx.compose.foundation.BorderStroke(1.dp, pc.error),
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(
                    "✗ $it",
                    color = pc.error,
                    fontSize = 12.sp,
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
                )
            }
            Spacer(Modifier.height(8.dp))
        }
        ui.toast?.let {
            Surface(
                color = pc.surface2,
                shape = MaterialTheme.shapes.medium,
                border = androidx.compose.foundation.BorderStroke(1.dp, pc.hairlineStrong),
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(
                    it,
                    color = pc.ink2,
                    fontSize = 12.sp,
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
                )
            }
            Spacer(Modifier.height(8.dp))
        }
        if (ui.loading) {
            LinearProgressIndicator(
                modifier = Modifier.fillMaxWidth(),
                color = pc.accent,
                trackColor = pc.surface3,
            )
            Spacer(Modifier.height(4.dp))
        }

        // Phase 4: 0 节点 empty state, 引导去 Subs tab 导入订阅
        if (!ui.loading && ui.nodes.isEmpty() && ui.error == null) {
            PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
                Column(
                    modifier = Modifier
                        .padding(horizontal = 20.dp, vertical = 24.dp)
                        .fillMaxWidth(),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    Text(
                        androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.nodes_empty_title),
                        color = pc.ink,
                        fontSize = 16.sp,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Text(
                        "添加一个订阅链接或导入节点 URI, 就能在这里切换。",
                        color = pc.ink3,
                        fontSize = 13.sp,
                        lineHeight = 18.sp,
                    )
                    Spacer(Modifier.height(2.dp))
                    PilottyButton(
                        text = androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.nodes_empty_cta),
                        onClick = onNavigateSubs,
                        variant = PilottyButtonVariant.Primary,
                        small = true,
                    )
                }
            }
        } else {
            // Mission Control: 单一 card-group 容纳所有行 (非独立卡片)
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = 12.dp, vertical = 4.dp),
            ) {
                item {
                    Surface(
                        color = pc.surface,
                        shape = MaterialTheme.shapes.large,
                        shadowElevation = if (pc.isDark) 0.dp else 4.dp,
                        border = if (pc.isDark) androidx.compose.foundation.BorderStroke(1.dp, pc.hairline) else null,
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Column {
                            filtered.forEachIndexed { i, node ->
                                if (i > 0) HorizontalDivider(color = pc.hairline, thickness = 1.dp)
                                NodeCard(
                                    node = node,
                                    isTesting = node.tag in ui.testing,
                                    isFastest = node.tag == ui.fastestTag,
                                    onClick = { vm.switchTo(node.tag) },
                                    onLongClick = { vm.testOne(node.tag) },
                                    onDetail = { onOpenDetail(node.tag) },
                                )
                            }
                        }
                    }
                }
                item { Spacer(Modifier.height(24.dp)) }
            }
        }
    }
}

@OptIn(androidx.compose.foundation.ExperimentalFoundationApi::class)
@Composable
private fun NodeCard(
    node: NodeDto,
    isTesting: Boolean = false,
    isFastest: Boolean = false,
    onClick: () -> Unit,
    onLongClick: () -> Unit = {},
    onDetail: () -> Unit = {},
) {
    val pc = LocalPilottyColors.current
    val cc = node.tag.take(2).uppercase()
    // Mission Control 三档: <100 nominal / <300 warn / >=300 alert
    val latTier = when {
        node.latency <= 0 -> "none"
        node.latency < 100 -> "nominal"
        node.latency < 300 -> "warn"
        else -> "alert"
    }
    val latColor = when (latTier) {
        "nominal" -> pc.ink
        "warn" -> pc.warn
        "alert" -> pc.error
        else -> pc.ink3
    }
    // A1 测速中 "…" 脉冲 (alpha 0.35 ↔ 1.0, 900ms 循环)
    val pulse = rememberInfiniteTransition(label = "node-pulse")
    val pulseAlpha by pulse.animateFloat(
        initialValue = 0.35f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 900, easing = LinearEasing),
            repeatMode = RepeatMode.Reverse,
        ),
        label = "pulse-alpha",
    )
    val rowBg = when {
        node.active -> pc.surface2
        isFastest -> pc.accent.copy(alpha = 0.10f)
        else -> androidx.compose.ui.graphics.Color.Transparent
    }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(rowBg)
            .combinedClickable(onClick = onClick, onLongClick = onLongClick)
            .padding(horizontal = 16.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        FlagChip(code = cc)
        Column(modifier = Modifier.weight(1f)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                Text(
                    node.tag,
                    color = pc.ink,
                    fontSize = 14.sp,
                    fontWeight = if (node.active) FontWeight.SemiBold else FontWeight.Medium,
                    letterSpacing = (-0.05).sp,
                    maxLines = 1,
                )
                if (node.active) LiveDot()
                if (isFastest) {
                    Surface(
                        color = pc.accent,
                        shape = MaterialTheme.shapes.extraSmall,
                    ) {
                        Text(
                            "FASTEST",
                            color = pc.accentOnBg,
                            fontSize = 8.5.sp,
                            fontWeight = FontWeight.Bold,
                            fontFamily = FontFamily.Monospace,
                            letterSpacing = 0.4.sp,
                            modifier = Modifier.padding(horizontal = 4.dp, vertical = 1.dp),
                        )
                    }
                }
            }
            Spacer(Modifier.height(3.dp))
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Text(
                    "${node.server}:${node.port}",
                    color = pc.ink3,
                    fontSize = 10.5.sp,
                    fontFamily = FontFamily.Monospace,
                    letterSpacing = 0.2.sp,
                    maxLines = 1,
                    modifier = Modifier.weight(1f, fill = false),
                )
                ProtoBadge(name = node.type)
            }
        }
        Column(horizontalAlignment = Alignment.End, verticalArrangement = Arrangement.spacedBy(5.dp)) {
            if (isTesting) {
                Text(
                    text = androidx.compose.ui.res.stringResource(com.pilotty.app.R.string.nodes_testing),
                    color = pc.accent.copy(alpha = pulseAlpha),
                    fontSize = 11.5.sp,
                    fontWeight = FontWeight.Bold,
                    fontFamily = FontFamily.Monospace,
                    letterSpacing = 0.3.sp,
                )
            } else {
                Text(
                    text = when {
                        node.latency > 0 -> "${node.latency}ms"
                        node.latency < 0 -> "失败"
                        else -> "—"
                    },
                    color = if (isFastest) pc.accent else latColor,
                    fontSize = 13.sp,
                    fontWeight = FontWeight.Bold,
                    fontFamily = FontFamily.Monospace,
                )
                // load bar 占位 (当前无真实 load 数据; 设计稿用 0-100 pct)
                if (node.latency > 0) {
                    LoadBar(pct = (node.latency / 4).coerceIn(10, 100), tone = if (latTier == "alert") "alert" else "nominal")
                }
            }
        }
        // 打开 NodeDetail 的入口 (分享 QR 按钮 / 详细 metrics 在里面)
        Text(
            "›",
            color = pc.ink3,
            fontSize = 20.sp,
            fontWeight = FontWeight.Medium,
            modifier = Modifier
                .clip(RoundedCornerShape(6.dp))
                .clickable { onDetail() }
                .padding(horizontal = 6.dp, vertical = 2.dp),
        )
    }
}

/* ============================= Rules (embedded in Settings) ============================= */

data class RulesUi(
    val loading: Boolean = false,
    val rules: List<RouteRuleDto> = emptyList(),
    val templates: List<TemplateDto> = emptyList(),
    /** AddRule 时 chip 可选的出口: 内置 direct/reject + 订阅里存在的 group tag (去重). */
    val availableOutbounds: List<String> = listOf("direct", "reject"),
    // Phase 10-A2 rule-set
    val ruleSets: List<RuleSetConfigDto> = emptyList(),
    val builtinRuleSets: List<RuleSetConfigDto> = emptyList(),
    val toast: String? = null,
    val error: String? = null,
)

class RulesViewModel : ViewModel() {
    private val _state = MutableStateFlow(RulesUi())
    val state: StateFlow<RulesUi> = _state.asStateFlow()

    init { refresh() }

    fun refresh() = viewModelScope.launch {
        _state.value = _state.value.copy(loading = true, error = null)
        try {
            val r = PilottyRepository.rules()
            val t = PilottyRepository.templates()
            val rs = runCatching { PilottyRepository.ruleSets() }.getOrDefault(emptyList())
            val builtin = runCatching { PilottyRepository.listBuiltinRuleSets() }.getOrDefault(emptyList())
            // Phase 4: 从当前节点列表动态推导可用出口 group
            //   - direct / reject 是 sing-box 内置 outbound, 永远可用
            //   - 去掉 "" (overlay 里可能有无 groupTag 的节点, 当内置处理)
            //   - 结合节点 tag 可加, 但太长了 UX 差, 先只收集 group 维度
            val nodes = runCatching { PilottyRepository.nodes() }.getOrDefault(emptyList())
            val groups = nodes.mapNotNull { it.groupTag.takeIf { g -> g.isNotBlank() } }.distinct()
            val outbounds = (listOf("direct", "reject") + groups).distinct()
            _state.value = _state.value.copy(
                loading = false,
                rules = r,
                templates = t,
                availableOutbounds = outbounds,
                ruleSets = rs,
                builtinRuleSets = builtin,
            )
        } catch (e: Throwable) {
            _state.value = _state.value.copy(loading = false, error = e.message)
        }
    }

    // Phase 10-A2 rule-set CRUD
    fun enableBuiltinRuleSet(alias: String) = viewModelScope.launch {
        try {
            val m = PilottyRepository.enableBuiltinRuleSet(alias)
            _state.value = _state.value.copy(toast = m.message.ifEmpty { "已启用 $alias" })
            refresh()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(error = e.message)
        }
    }

    fun removeRuleSet(tag: String) = viewModelScope.launch {
        try {
            val m = PilottyRepository.removeRuleSet(tag)
            _state.value = _state.value.copy(toast = m.message.ifEmpty { "已移除 $tag" })
            refresh()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(error = e.message)
        }
    }

    /**
     * Phase 10-F-3: 添加自定义 remote rule-set (如 MetaCubeX 国内镜像 https://github.com/
     * MetaCubeX/meta-rules-dat/raw/sing/geo/geoip/cn.srs)。 之前只有 5 个 SagerNet builtin,
     * UI 零入口, 用户想换镜像得改 overlay.json。
     */
    fun addCustomRuleSet(tag: String, url: String, updateInterval: String) = viewModelScope.launch {
        try {
            val t = tag.trim()
            val u = url.trim()
            val ui = updateInterval.trim().ifEmpty { "7d" }
            if (t.isEmpty() || u.isEmpty()) {
                _state.value = _state.value.copy(error = "tag 和 URL 不能为空")
                return@launch
            }
            // type=remote 由后端强制校验, source 会被 mobile 层固定覆盖成 "user"
            val escT = t.replace("\\", "\\\\").replace("\"", "\\\"")
            val escU = u.replace("\\", "\\\\").replace("\"", "\\\"")
            val escI = ui.replace("\\", "\\\\").replace("\"", "\\\"")
            val json = """{"tag":"$escT","type":"remote","format":"binary","url":"$escU","update_interval":"$escI"}"""
            val m = PilottyRepository.addRuleSet(json)
            _state.value = _state.value.copy(toast = m.message.ifEmpty { "已添加 $t" })
            refresh()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(error = e.message)
        }
    }

    fun applyTemplate(id: String) = viewModelScope.launch {
        try {
            val m = PilottyRepository.applyTemplate(id)
            _state.value = _state.value.copy(toast = m.message)
            refresh()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(error = e.message)
        }
    }

    /** M18 用户自定义规则。 ruleJSON 由 UI 端拼好,已包含 tag/matcher/outbound/description。 */
    fun addRule(ruleJSON: String) = viewModelScope.launch {
        try {
            val m = PilottyRepository.addRule(ruleJSON)
            _state.value = _state.value.copy(toast = m.message.ifEmpty { "规则已添加" })
            refresh()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(error = e.message)
        }
    }

    fun removeRule(tag: String) = viewModelScope.launch {
        try {
            val m = PilottyRepository.removeRule(tag)
            _state.value = _state.value.copy(toast = m.message.ifEmpty { "规则已删除" })
            refresh()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(error = e.message)
        }
    }

    fun dismissToast() { _state.value = _state.value.copy(toast = null) }
    fun dismissError() { _state.value = _state.value.copy(error = null) }
}

@Composable
fun RulesSection(vm: RulesViewModel = viewModel()) {
    val pc = LocalPilottyColors.current
    val ui by vm.state.collectAsStateWithLifecycle()
    var addDialogOpen by remember { mutableStateOf(false) }
    var deleteTarget by remember { mutableStateOf<RouteRuleDto?>(null) }
    var enableRSDialogOpen by remember { mutableStateOf(false) }
    var deleteRSTarget by remember { mutableStateOf<RuleSetConfigDto?>(null) }

    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        ui.error?.let { Text("错误: $it", color = pc.error, fontSize = 12.sp) }
        ui.toast?.let {
            Text("· $it", color = pc.ink3, fontSize = 11.sp)
            LaunchedEffect(it) {
                kotlinx.coroutines.delay(2000)
                vm.dismissToast()
            }
        }

        Kicker("Templates · ${ui.templates.size}")
        ui.templates.forEach { t ->
            PilottyCard(
                modifier = Modifier
                    .fillMaxWidth()
                    .clickable { vm.applyTemplate(t.id) },
                soft = true,
            ) {
                Column(Modifier.padding(12.dp)) {
                    Text(t.name, color = pc.ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
                    if (t.description.isNotEmpty()) {
                        Spacer(Modifier.height(2.dp))
                        Text(t.description, color = pc.ink3, fontSize = 12.sp)
                    }
                }
            }
        }

        // Phase 10-A2: Rule sets (geoip / geosite / 自定义远程规则包)
        Spacer(Modifier.height(8.dp))
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Kicker("Rule sets · ${ui.ruleSets.size}")
            PilottyButton(
                text = "+ 启用内置",
                onClick = { enableRSDialogOpen = true },
                variant = PilottyButtonVariant.Outline,
                small = true,
            )
        }
        if (ui.ruleSets.isEmpty()) {
            Text(
                "尚未启用规则集。 geoip-cn / geosite-cn 等预设可实现 \"国内直连,国外代理\" 等分流场景。",
                color = pc.ink4,
                fontSize = 11.5.sp,
            )
        } else {
            ui.ruleSets.forEach { rs ->
                PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 12.dp, vertical = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column(Modifier.weight(1f)) {
                            Text(
                                rs.tag,
                                color = pc.ink,
                                fontSize = 13.sp,
                                fontWeight = FontWeight.Medium,
                                fontFamily = FontFamily.Monospace,
                            )
                            Spacer(Modifier.height(2.dp))
                            val hint = buildString {
                                append(rs.type)
                                if (rs.format.isNotEmpty()) append(" · ${rs.format}")
                                if (rs.updateInterval.isNotEmpty()) append(" · ${rs.updateInterval}")
                            }
                            Text(hint, color = pc.ink3, fontSize = 11.sp, fontFamily = FontFamily.Monospace)
                        }
                        Text(
                            rs.source.ifEmpty { "-" },
                            color = pc.ink4,
                            fontSize = 10.sp,
                            fontFamily = FontFamily.Monospace,
                        )
                        Spacer(Modifier.width(6.dp))
                        Text(
                            "×",
                            color = pc.error,
                            fontSize = 16.sp,
                            fontWeight = FontWeight.Bold,
                            modifier = Modifier
                                .clickable { deleteRSTarget = rs }
                                .padding(horizontal = 6.dp),
                        )
                    }
                }
            }
        }

        Spacer(Modifier.height(8.dp))
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Kicker("Active rules · ${ui.rules.size}")
            PilottyButton(
                text = "+ 自定义规则",
                onClick = { addDialogOpen = true },
                variant = PilottyButtonVariant.Outline,
                small = true,
            )
        }
        ui.rules.forEach { r ->
            PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
                Column(Modifier.padding(horizontal = 12.dp, vertical = 10.dp)) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(
                            r.description.ifEmpty { r.tag },
                            color = pc.ink,
                            fontSize = 13.sp,
                            fontWeight = FontWeight.Medium,
                            modifier = Modifier.weight(1f),
                        )
                        Text(
                            r.source,
                            color = pc.ink4,
                            fontSize = 10.sp,
                            fontFamily = FontFamily.Monospace,
                        )
                        Spacer(Modifier.width(6.dp))
                        Text(
                            "×",
                            color = pc.error,
                            fontSize = 16.sp,
                            fontWeight = FontWeight.Bold,
                            modifier = Modifier
                                .clickable { deleteTarget = r }
                                .padding(horizontal = 6.dp),
                        )
                    }
                    Spacer(Modifier.height(4.dp))
                    val parts = buildList {
                        if (r.domainSuffix.isNotEmpty()) add("suffix×${r.domainSuffix.size}")
                        if (r.domain.isNotEmpty()) add("domain×${r.domain.size}")
                        if (r.domainKeyword.isNotEmpty()) add("kw×${r.domainKeyword.size}")
                        if (r.domainRegex.isNotEmpty()) add("regex×${r.domainRegex.size}")
                        if (r.ipCidr.isNotEmpty()) add("ip×${r.ipCidr.size}")
                        if (r.processName.isNotEmpty()) add("proc×${r.processName.size}")
                        if (r.port.isNotEmpty()) add("port×${r.port.size}")
                        if (r.portRange.isNotEmpty()) add("portR×${r.portRange.size}")
                        if (r.network.isNotEmpty()) add("net=${r.network.joinToString("/")}")
                        if (r.protocol.isNotEmpty()) add("proto=${r.protocol.joinToString("/")}")
                        if (r.ruleSet.isNotEmpty()) add("rs=${r.ruleSet.joinToString(",")}")
                        if (r.geoip.isNotEmpty()) add("geoip=${r.geoip.joinToString(",")}")
                        if (r.geosite.isNotEmpty()) add("geosite=${r.geosite.joinToString(",")}")
                    }
                    Text(
                        "→ ${r.outbound}  ${parts.joinToString("  ")}",
                        color = pc.ink3,
                        fontSize = 11.5.sp,
                        fontFamily = FontFamily.Monospace,
                    )
                }
            }
        }
    }

    if (addDialogOpen) {
        AddRuleDialog(
            availableOutbounds = ui.availableOutbounds,
            availableRuleSetTags = ui.ruleSets.map { it.tag },
            onDismiss = { addDialogOpen = false },
            onConfirm = { ruleJSON ->
                vm.addRule(ruleJSON)
                addDialogOpen = false
            },
        )
    }

    deleteTarget?.let { target ->
        AlertDialog(
            onDismissRequest = { deleteTarget = null },
            containerColor = pc.surface,
            title = { Text("删除规则", color = pc.ink) },
            text = { Text("确定删除规则「${target.description.ifEmpty { target.tag }}」?", color = pc.ink2) },
            confirmButton = {
                TextButton(onClick = {
                    vm.removeRule(target.tag)
                    deleteTarget = null
                }) { Text("删除", color = pc.error) }
            },
            dismissButton = {
                TextButton(onClick = { deleteTarget = null }) { Text("取消", color = pc.ink2) }
            },
        )
    }

    if (enableRSDialogOpen) {
        EnableBuiltinRuleSetDialog(
            builtins = ui.builtinRuleSets,
            enabledTags = ui.ruleSets.map { it.tag }.toSet(),
            onDismiss = { enableRSDialogOpen = false },
            onEnable = { alias ->
                vm.enableBuiltinRuleSet(alias)
                enableRSDialogOpen = false
            },
            onCustom = { tag, url, interval ->
                vm.addCustomRuleSet(tag, url, interval)
                enableRSDialogOpen = false
            },
        )
    }

    deleteRSTarget?.let { target ->
        AlertDialog(
            onDismissRequest = { deleteRSTarget = null },
            containerColor = pc.surface,
            title = { Text("移除规则集", color = pc.ink) },
            text = {
                Column {
                    Text("确定移除规则集「${target.tag}」?", color = pc.ink2)
                    Spacer(Modifier.height(4.dp))
                    Text(
                        "若仍有 rule 引用此 tag, sing-box 重载会失败;请先在 Active rules 里调整对应规则。",
                        color = pc.ink4,
                        fontSize = 11.sp,
                    )
                }
            },
            confirmButton = {
                TextButton(onClick = {
                    vm.removeRuleSet(target.tag)
                    deleteRSTarget = null
                }) { Text("移除", color = pc.error) }
            },
            dismissButton = {
                TextButton(onClick = { deleteRSTarget = null }) { Text("取消", color = pc.ink2) }
            },
        )
    }
}

@Composable
private fun EnableBuiltinRuleSetDialog(
    builtins: List<RuleSetConfigDto>,
    enabledTags: Set<String>,
    onDismiss: () -> Unit,
    onEnable: (alias: String) -> Unit,
    onCustom: (tag: String, url: String, updateInterval: String) -> Unit = { _, _, _ -> },
) {
    val pc = LocalPilottyColors.current
    // Phase 10-F-3: 切换"内置"和"自定义"两个 mode, 都在同一 Dialog 里避免多层弹框
    var customMode by remember { mutableStateOf(false) }
    var customTag by remember { mutableStateOf("") }
    var customUrl by remember { mutableStateOf("") }
    var customInterval by remember { mutableStateOf("7d") }

    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = pc.surface,
        title = { Text(if (customMode) "添加自定义规则集" else "启用规则集", color = pc.ink) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                // 模式切换 chip
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    PilottyChip(
                        text = "内置预设",
                        selected = !customMode,
                        onClick = { customMode = false },
                    )
                    PilottyChip(
                        text = "自定义 URL",
                        selected = customMode,
                        onClick = { customMode = true },
                    )
                }

                if (!customMode) {
                    Text(
                        "选一个预置规则集启用。 SagerNet 官方二进制格式, 每 7 天自动更新。",
                        color = pc.ink3,
                        fontSize = 11.sp,
                    )
                    builtins.forEach { rs ->
                        val already = rs.tag in enabledTags
                        PilottyCard(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable(enabled = !already) { onEnable(rs.tag) },
                            soft = true,
                        ) {
                            Column(Modifier.padding(10.dp)) {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Text(
                                        rs.tag,
                                        color = if (already) pc.ink4 else pc.ink,
                                        fontSize = 13.sp,
                                        fontFamily = FontFamily.Monospace,
                                        fontWeight = FontWeight.Medium,
                                    )
                                    if (already) {
                                        Spacer(Modifier.width(6.dp))
                                        Text("已启用", color = pc.accentInk, fontSize = 10.sp)
                                    }
                                }
                                Spacer(Modifier.height(2.dp))
                                Text(
                                    rs.url.ifEmpty { rs.path },
                                    color = pc.ink4,
                                    fontSize = 10.sp,
                                    fontFamily = FontFamily.Monospace,
                                )
                            }
                        }
                    }
                } else {
                    Text(
                        "用 MetaCubeX 国内镜像或自建 ruleset。 URL 必须以 .srs / .json 结尾, sing-box 按扩展名判格式。",
                        color = pc.ink3,
                        fontSize = 11.sp,
                    )
                    OutlinedTextField(
                        value = customTag,
                        onValueChange = { customTag = it },
                        label = { Text("tag (规则集标识)") },
                        placeholder = { Text("例如: geoip-cn-metacubex") },
                        singleLine = true,
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedBorderColor = pc.ink,
                            unfocusedBorderColor = pc.hairlineStrong,
                        ),
                    )
                    OutlinedTextField(
                        value = customUrl,
                        onValueChange = { customUrl = it },
                        label = { Text("URL") },
                        placeholder = { Text("https://.../xx.srs") },
                        singleLine = true,
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedBorderColor = pc.ink,
                            unfocusedBorderColor = pc.hairlineStrong,
                        ),
                    )
                    OutlinedTextField(
                        value = customInterval,
                        onValueChange = { customInterval = it },
                        label = { Text("更新间隔 (如 7d / 24h)") },
                        singleLine = true,
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedBorderColor = pc.ink,
                            unfocusedBorderColor = pc.hairlineStrong,
                        ),
                    )
                }
            }
        },
        confirmButton = {
            if (customMode) {
                TextButton(
                    onClick = {
                        onCustom(customTag, customUrl, customInterval)
                        customTag = ""; customUrl = ""
                    },
                    enabled = customTag.isNotBlank() && customUrl.isNotBlank(),
                ) {
                    Text(
                        "添加",
                        color = if (customTag.isNotBlank() && customUrl.isNotBlank()) pc.accentInk else pc.ink4,
                        fontWeight = FontWeight.SemiBold,
                    )
                }
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("关闭", color = pc.ink2) }
        },
    )
}

@Composable
private fun AddRuleDialog(
    availableOutbounds: List<String>,
    availableRuleSetTags: List<String>,
    onDismiss: () -> Unit,
    onConfirm: (ruleJSON: String) -> Unit,
) {
    val pc = LocalPilottyColors.current
    var description by remember { mutableStateOf("") }
    var matcherType by remember { mutableStateOf("domain_suffix") }
    var matcherValue by remember { mutableStateOf("") }
    // rule_set 是多选 tag, 其它 matcher 走 matcherValue 文本框
    var ruleSetSelected by remember { mutableStateOf(setOf<String>()) }
    // 默认选 availableOutbounds 里的首项 (direct)
    var outbound by remember { mutableStateOf(availableOutbounds.firstOrNull() ?: "direct") }

    // Phase 10-A2 扩到 13 个 matcher。 顺序按"最常用 → 最罕见"
    val matcherTypes = listOf(
        "domain_suffix", "domain", "domain_keyword", "domain_regex",
        "ip_cidr", "geoip", "geosite", "rule_set",
        "port", "port_range", "network", "protocol", "process_name",
    )
    val matcherLabel: (String) -> String = {
        when (it) {
            "domain_suffix" -> "后缀"
            "domain" -> "域名"
            "domain_keyword" -> "关键词"
            "domain_regex" -> "正则"
            "ip_cidr" -> "IP 段"
            "process_name" -> "进程"
            "port" -> "端口"
            "port_range" -> "端口段"
            "network" -> "网络"
            "protocol" -> "协议"
            "rule_set" -> "规则集"
            "geoip" -> "GeoIP"
            "geosite" -> "GeoSite"
            else -> it
        }
    }
    val outbounds = availableOutbounds.ifEmpty { listOf("direct", "reject") }

    fun buildRuleJSON(): String? {
        val tag = "user-" + System.currentTimeMillis().toString(36)

        // 拼 matcher 字段
        val matcherFieldJSON: String = when (matcherType) {
            "rule_set" -> {
                if (ruleSetSelected.isEmpty()) return null
                val arr = ruleSetSelected.joinToString(",") { "\"" + it + "\"" }
                "\"rule_set\":[$arr]"
            }
            "port" -> {
                val ints = matcherValue.split(",").mapNotNull { it.trim().toIntOrNull() }
                if (ints.isEmpty()) return null
                "\"port\":[${ints.joinToString(",")}]"
            }
            else -> {
                val value = matcherValue.trim()
                if (value.isEmpty()) return null
                val escapedVals = value.split(",").map { it.trim() }.filter { it.isNotEmpty() }
                    .joinToString(",") { "\"" + it.replace("\\", "\\\\").replace("\"", "\\\"") + "\"" }
                "\"$matcherType\":[$escapedVals]"
            }
        }
        val descText = description.trim().ifEmpty {
            if (matcherType == "rule_set") "rule_set=${ruleSetSelected.joinToString(",")} → $outbound"
            else "$matcherType=${matcherValue.trim()} → $outbound"
        }
        val escapedDesc = descText.replace("\"", "\\\"")
        return """{"tag":"$tag",$matcherFieldJSON,"outbound":"$outbound","description":"$escapedDesc"}"""
    }

    val canConfirm: Boolean = when (matcherType) {
        "rule_set" -> ruleSetSelected.isNotEmpty()
        else -> matcherValue.isNotBlank()
    }

    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = pc.surface,
        title = { Text("自定义路由规则", color = pc.ink) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                OutlinedTextField(
                    value = description,
                    onValueChange = { description = it },
                    label = { Text("描述 (可选)") },
                    singleLine = true,
                    colors = OutlinedTextFieldDefaults.colors(
                        focusedBorderColor = pc.ink,
                        unfocusedBorderColor = pc.hairlineStrong,
                    ),
                )
                Text("匹配条件", color = pc.ink4, fontSize = 11.sp)
                Row(
                    modifier = Modifier.horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    matcherTypes.forEach { mt ->
                        PilottyChip(
                            text = matcherLabel(mt),
                            selected = matcherType == mt,
                            onClick = { matcherType = mt },
                        )
                    }
                }

                if (matcherType == "rule_set") {
                    if (availableRuleSetTags.isEmpty()) {
                        Text(
                            "当前没有已启用的规则集。 先到上方 Rule sets 区启用 geoip-cn / geosite-cn 等内置项,再回来引用。",
                            color = pc.ink4,
                            fontSize = 11.5.sp,
                        )
                    } else {
                        Text("选择规则集 (可多选)", color = pc.ink4, fontSize = 11.sp)
                        Row(
                            modifier = Modifier.horizontalScroll(rememberScrollState()),
                            horizontalArrangement = Arrangement.spacedBy(6.dp),
                        ) {
                            availableRuleSetTags.forEach { t ->
                                val selected = t in ruleSetSelected
                                PilottyChip(
                                    text = t,
                                    selected = selected,
                                    onClick = {
                                        ruleSetSelected = if (selected) ruleSetSelected - t
                                        else ruleSetSelected + t
                                    },
                                )
                            }
                        }
                    }
                } else {
                    OutlinedTextField(
                        value = matcherValue,
                        onValueChange = { matcherValue = it },
                        label = {
                            Text(
                                when (matcherType) {
                                    "domain_suffix" -> "值 (逗号分隔,如 google.com,youtube.com)"
                                    "domain" -> "值 (完整域名,逗号分隔)"
                                    "domain_keyword" -> "值 (域名子串,如 google,youtube)"
                                    "domain_regex" -> "值 (Go 正则, 逗号分隔)"
                                    "ip_cidr" -> "值 (如 10.0.0.0/8,192.168.0.0/16)"
                                    "process_name" -> "值 (如 com.tencent.mm)"
                                    "port" -> "值 (逗号分隔整数,如 80,443,853)"
                                    "port_range" -> "值 (如 8000:9000, 可多段逗号分隔)"
                                    "network" -> "值 (tcp / udp, 逗号分隔)"
                                    "protocol" -> "值 (http / tls / quic / dns, 逗号分隔)"
                                    "geoip" -> "值 (如 cn,private —— 仍需在 Rule sets 区启用 geoip-xxx)"
                                    "geosite" -> "值 (如 cn,category-ads-all —— 仍需启用对应 geosite-xxx)"
                                    else -> "值"
                                }
                            )
                        },
                        singleLine = true,
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedBorderColor = pc.ink,
                            unfocusedBorderColor = pc.hairlineStrong,
                        ),
                    )
                }

                Text("出口", color = pc.ink4, fontSize = 11.sp)
                Row(
                    modifier = Modifier.horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    outbounds.forEach { ob ->
                        PilottyChip(
                            text = ob,
                            selected = outbound == ob,
                            onClick = { outbound = ob },
                        )
                    }
                }
            }
        },
        confirmButton = {
            TextButton(
                onClick = { buildRuleJSON()?.let(onConfirm) },
                enabled = canConfirm,
            ) {
                Text(
                    "添加",
                    color = if (canConfirm) pc.accentInk else pc.ink4,
                    fontWeight = FontWeight.SemiBold,
                )
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("取消", color = pc.ink2) }
        },
    )
}
