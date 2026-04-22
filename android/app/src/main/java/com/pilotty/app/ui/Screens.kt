package com.pilotty.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
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
import com.pilotty.app.data.*
import com.pilotty.app.ui.agent.AgentQueryBus
import com.pilotty.app.ui.components.*
import com.pilotty.app.ui.theme.LocalPilottyColors
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
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
)

class ChatViewModel : ViewModel() {
    private val _state = MutableStateFlow(ChatUi())
    val state: StateFlow<ChatUi> = _state.asStateFlow()

    fun send(text: String) {
        if (text.isBlank()) return
        val u = ChatMessage("user", text)
        _state.value = _state.value.copy(
            sending = true,
            messages = _state.value.messages + u,
            error = null,
        )
        viewModelScope.launch {
            try {
                val r = PilottyRepository.chat(text)
                _state.value = _state.value.copy(
                    sending = false,
                    messages = _state.value.messages + ChatMessage(
                        role = "assistant",
                        text = r.reply,
                        source = r.source,
                        events = r.events,
                    ),
                )
            } catch (e: Throwable) {
                _state.value = _state.value.copy(sending = false, error = e.message)
            }
        }
    }

    fun clear() {
        PilottyRepository.clearHistory()
        _state.value = ChatUi()
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
            Text(
                buildString {
                    append("调用 ${events.size} 个工具 · ${formatDurationMs(totalMs)}")
                    if (failedCount > 0) append(" · ⚠ ${failedCount} 失败")
                },
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
                    val ok = e.error.isEmpty()
                    val badge = if (ok) "✓" else "✗"
                    val badgeColor = if (ok) pc.accentInk else pc.error
                    val detail = buildString {
                        append(e.name)
                        if (e.role.isNotEmpty()) append(" · ").append(e.role)
                        append(" · ").append(formatDurationMs(e.durationMs))
                        val extra = if (ok) e.outputPreview else e.error
                        if (extra.isNotEmpty()) {
                            append("\n").append(extra.take(140))
                            if (extra.length > 140) append("…")
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
fun ChatScreen(vm: ChatViewModel = viewModel()) {
    val pc = LocalPilottyColors.current
    val ui by vm.state.collectAsStateWithLifecycle()
    var input by remember { mutableStateOf("") }
    val listState = rememberLazyListState()

    LaunchedEffect(ui.messages.size) {
        if (ui.messages.isNotEmpty()) listState.animateScrollToItem(ui.messages.size - 1)
    }

    // D1: Home "Ask the agent" → Chat 自动发送。 收到 pending query 就丢 vm.send + consume
    val pendingQuery by AgentQueryBus.pending.collectAsStateWithLifecycle()
    LaunchedEffect(pendingQuery) {
        pendingQuery?.let {
            vm.send(it)
            AgentQueryBus.consume()
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
                Text("Agent", color = pc.ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, letterSpacing = (-0.15).sp)
                Text(
                    "deepseek-chat · ${ui.messages.count { it.role == "user" }} msgs",
                    color = pc.ink3,
                    fontSize = 10.5.sp,
                    fontFamily = FontFamily.Monospace,
                    letterSpacing = 0.2.sp,
                )
            }
            TextButton(onClick = { vm.clear() }) {
                Text("清空", color = pc.ink3, fontSize = 12.sp)
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
            items(ui.messages) { msg ->
                val isUser = msg.role == "user"
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = if (isUser) Arrangement.End else Arrangement.Start,
                ) {
                    Surface(
                        color = if (isUser) pc.ink else pc.surface2,
                        shape = if (isUser) RoundedCornerShape(16.dp, 16.dp, 4.dp, 16.dp)
                                else RoundedCornerShape(16.dp, 16.dp, 16.dp, 4.dp),
                    ) {
                        Column(Modifier.padding(horizontal = 13.dp, vertical = 9.dp)) {
                            Text(
                                msg.text,
                                color = if (isUser) pc.bg else pc.ink,
                                fontSize = 14.sp,
                                lineHeight = 20.sp,
                                letterSpacing = (-0.07).sp,
                            )
                            if (!isUser && msg.source.isNotEmpty()) {
                                Spacer(Modifier.height(4.dp))
                                Text(
                                    "via ${msg.source}${if (msg.source == "local") " · 未调 Agent" else ""}",
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

        if (ui.sending) LinearProgressIndicator(
            modifier = Modifier.fillMaxWidth(),
            color = pc.accent,
            trackColor = pc.surface3,
        )

        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(pc.surface)
                .padding(horizontal = 16.dp, vertical = 12.dp)
                .padding(bottom = 96.dp),
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(16.dp))
                    .background(pc.bg)
                    .border(1.dp, pc.hairlineStrong, RoundedCornerShape(16.dp))
                    .padding(start = 14.dp, end = 8.dp, top = 8.dp, bottom = 8.dp),
                verticalAlignment = Alignment.Bottom,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Box(modifier = Modifier.weight(1f).heightIn(min = 36.dp, max = 140.dp)) {
                    BasicTextField(
                        value = input,
                        onValueChange = { input = it },
                        modifier = Modifier.fillMaxWidth(),
                        textStyle = TextStyle(
                            color = pc.ink,
                            fontSize = 14.sp,
                            lineHeight = 20.sp,
                            letterSpacing = (-0.07).sp,
                        ),
                        cursorBrush = SolidColor(pc.ink),
                        maxLines = 6,
                        enabled = !ui.sending,
                    )
                    if (input.isEmpty()) {
                        Text(
                            "Reply to agent…",
                            color = pc.ink4,
                            fontSize = 14.sp,
                            lineHeight = 20.sp,
                        )
                    }
                }
                Box(
                    modifier = Modifier
                        .size(34.dp)
                        .clip(RoundedCornerShape(10.dp))
                        .background(if (input.isNotBlank()) pc.accent else pc.surface3)
                        .clickable(enabled = !ui.sending && input.isNotBlank()) {
                            vm.send(input)
                            input = ""
                        },
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        "↑",
                        color = if (input.isNotBlank()) pc.accentOnBg else pc.ink4,
                        fontSize = 16.sp,
                        fontWeight = FontWeight.Bold,
                    )
                }
            }
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

    fun switchTo(tag: String) = viewModelScope.launch {
        // Clash API 要求 VPN 运行, 否则 switchNode 会撞 connection refused。
        if (!com.pilotty.app.PilottyCore.tunRunning.value) {
            _state.value = _state.value.copy(toast = "请先启动 VPN 再切换节点")
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
        _state.value = _state.value.copy(loading = true)
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
                        loading = false,
                        toast = "VPN 启动超时, 测速取消。 请手动启动后重试。",
                    )
                    return@launch
                }
                // Clash API server 在 libbox 起来的同时暴露, 但首次 /proxies 可能还没填好节点
                // 状态缓存, 等 1.5s 再发测速请求以免拿到部分 NaN 结果。
                kotlinx.coroutines.delay(1_500)
            }
            PilottyRepository.testLatencyAll()
            val n2 = PilottyRepository.nodes()
            _state.value = _state.value.copy(loading = false, nodes = n2)
        } catch (e: Throwable) {
            _state.value = _state.value.copy(loading = false, error = e.message)
        }
    }

    fun dismissToast() { _state.value = _state.value.copy(toast = null) }
}

@Composable
fun NodesScreen(
    onNavigateSubs: () -> Unit = {},
    vm: NodesViewModel = viewModel(),
) {
    val pc = LocalPilottyColors.current
    val ui by vm.state.collectAsStateWithLifecycle()
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
            .background(pc.bg)
            .padding(horizontal = 20.dp),
    ) {
        Spacer(Modifier.height(12.dp))
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column {
                Text("Nodes", color = pc.ink, fontSize = 20.sp, fontWeight = FontWeight.SemiBold, letterSpacing = (-0.3).sp)
                Text(
                    "${filtered.size} of ${ui.nodes.size} · sorted by latency",
                    color = pc.ink3,
                    fontSize = 10.5.sp,
                    fontFamily = FontFamily.Monospace,
                    letterSpacing = 0.2.sp,
                )
            }
            PilottyButton(
                text = "测速全部",
                onClick = { vm.testAll() },
                variant = PilottyButtonVariant.Outline,
                small = true,
            )
        }

        Spacer(Modifier.height(12.dp))
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(36.dp)
                .clip(RoundedCornerShape(10.dp))
                .background(pc.bg)
                .border(1.dp, pc.hairlineStrong, RoundedCornerShape(10.dp))
                .padding(horizontal = 12.dp),
            contentAlignment = Alignment.CenterStart,
        ) {
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
                Text("Search nodes…", color = pc.ink4, fontSize = 13.5.sp)
            }
        }

        Spacer(Modifier.height(10.dp))

        ui.error?.let {
            Surface(
                color = pc.surface2,
                shape = RoundedCornerShape(12.dp),
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
                shape = RoundedCornerShape(12.dp),
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
                        "还没有节点",
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
                        text = "去 Subs 导入订阅",
                        onClick = onNavigateSubs,
                        variant = PilottyButtonVariant.Primary,
                        small = true,
                    )
                }
            }
        } else {
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.spacedBy(8.dp),
                contentPadding = PaddingValues(bottom = 96.dp),
            ) {
                items(filtered, key = { it.tag }) { node ->
                    NodeCard(node = node, onClick = { vm.switchTo(node.tag) })
                }
            }
        }
    }
}

@Composable
private fun NodeCard(node: NodeDto, onClick: () -> Unit) {
    val pc = LocalPilottyColors.current
    val slow = node.latency > 300
    val cc = node.tag.take(2).uppercase()
    Box(modifier = Modifier.fillMaxWidth()) {
        PilottyCard(
            modifier = Modifier
                .fillMaxWidth()
                .clickable(onClick = onClick),
            strong = node.active,
            soft = !node.active,
        ) {
            Row(
                modifier = Modifier.padding(start = 14.dp, end = 12.dp, top = 12.dp, bottom = 12.dp),
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
                        )
                        if (node.active) LiveDot()
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
                        Box(
                            modifier = Modifier
                                .clip(RoundedCornerShape(3.dp))
                                .border(1.dp, pc.hairlineStrong, RoundedCornerShape(3.dp))
                                .padding(horizontal = 5.dp, vertical = 1.dp),
                        ) {
                            Text(
                                node.type.uppercase(),
                                color = pc.ink2,
                                fontSize = 9.sp,
                                fontWeight = FontWeight.SemiBold,
                                letterSpacing = 0.4.sp,
                            )
                        }
                    }
                }
                Text(
                    text = when {
                        node.latency > 0 -> "${node.latency}ms"
                        node.latency < 0 -> "失败"
                        else -> "—"
                    },
                    color = when {
                        node.latency < 0 -> pc.error
                        slow -> pc.error
                        node.active -> pc.accentInk
                        else -> pc.ink
                    },
                    fontSize = 12.5.sp,
                    fontWeight = if (node.active) FontWeight.Bold else FontWeight.SemiBold,
                    fontFamily = FontFamily.Monospace,
                )
            }
        }
        if (node.active) {
            Box(
                modifier = Modifier
                    .width(3.dp)
                    .fillMaxHeight()
                    .padding(vertical = 10.dp)
                    .clip(RoundedCornerShape(2.dp))
                    .background(pc.accent)
                    .align(Alignment.CenterStart),
            )
        }
    }
}

/* ============================= Rules (embedded in Settings) ============================= */

data class RulesUi(
    val loading: Boolean = false,
    val rules: List<RouteRuleDto> = emptyList(),
    val templates: List<TemplateDto> = emptyList(),
    /** AddRule 时 chip 可选的出口: 内置 direct/reject + 订阅里存在的 group tag (去重). */
    val availableOutbounds: List<String> = listOf("direct", "reject"),
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
            )
        } catch (e: Throwable) {
            _state.value = _state.value.copy(loading = false, error = e.message)
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
                        if (r.ipCidr.isNotEmpty()) add("ip×${r.ipCidr.size}")
                        if (r.processName.isNotEmpty()) add("proc×${r.processName.size}")
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
}

@Composable
private fun AddRuleDialog(
    availableOutbounds: List<String>,
    onDismiss: () -> Unit,
    onConfirm: (ruleJSON: String) -> Unit,
) {
    val pc = LocalPilottyColors.current
    var description by remember { mutableStateOf("") }
    var matcherType by remember { mutableStateOf("domain_suffix") }
    var matcherValue by remember { mutableStateOf("") }
    // 默认选 availableOutbounds 里的首项 (direct) — 不再写死 "direct" (兼容未来 reject 消失的情况)
    var outbound by remember { mutableStateOf(availableOutbounds.firstOrNull() ?: "direct") }

    val matcherTypes = listOf("domain_suffix", "domain", "ip_cidr", "process_name")
    // Phase 4: outbounds 从 RulesViewModel 注入, 动态反映当前节点的 group tag (含 direct/reject 内置)
    val outbounds = availableOutbounds.ifEmpty { listOf("direct", "reject") }

    fun buildRuleJSON(): String? {
        val value = matcherValue.trim()
        if (value.isEmpty()) return null
        val tag = "user-" + System.currentTimeMillis().toString(36)
        val desc = description.trim().ifEmpty { "$matcherType=$value → $outbound" }
        val escapedVals = value.split(",").map { it.trim() }.filter { it.isNotEmpty() }
            .joinToString(",") { "\"" + it.replace("\\", "\\\\").replace("\"", "\\\"") + "\"" }
        val matcherField = matcherType
        return """{"tag":"$tag","$matcherField":[$escapedVals],"outbound":"$outbound","description":"${desc.replace("\"", "\\\"")}"}"""
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
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    matcherTypes.forEach { mt ->
                        PilottyChip(
                            text = when (mt) {
                                "domain_suffix" -> "域名后缀"
                                "domain" -> "完整域名"
                                "ip_cidr" -> "IP 段"
                                "process_name" -> "进程名"
                                else -> mt
                            },
                            selected = matcherType == mt,
                            onClick = { matcherType = mt },
                        )
                    }
                }
                OutlinedTextField(
                    value = matcherValue,
                    onValueChange = { matcherValue = it },
                    label = { Text(when (matcherType) {
                        "domain_suffix" -> "值 (逗号分隔,如 google.com,youtube.com)"
                        "domain" -> "值 (完整域名,逗号分隔)"
                        "ip_cidr" -> "值 (如 10.0.0.0/8,192.168.0.0/16)"
                        "process_name" -> "值 (如 com.tencent.mm)"
                        else -> "值"
                    }) },
                    singleLine = true,
                    colors = OutlinedTextFieldDefaults.colors(
                        focusedBorderColor = pc.ink,
                        unfocusedBorderColor = pc.hairlineStrong,
                    ),
                )
                Text("出口", color = pc.ink4, fontSize = 11.sp)
                // 可横向滚动 (group 可能很多,长行吃不下时允许划动)
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
                enabled = matcherValue.isNotBlank(),
            ) { Text("添加", color = if (matcherValue.isNotBlank()) pc.accentInk else pc.ink4, fontWeight = FontWeight.SemiBold) }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("取消", color = pc.ink2) }
        },
    )
}
