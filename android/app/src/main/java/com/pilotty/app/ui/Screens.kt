package com.pilotty.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
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
import com.pilotty.app.ui.components.*
import com.pilotty.app.ui.theme.LocalPilottyColors
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/* ============================= Chat ============================= */

data class ChatMessage(val role: String, val text: String, val source: String = "")

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
                    messages = _state.value.messages + ChatMessage("assistant", r.reply, r.source),
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

@Composable
fun ChatScreen(vm: ChatViewModel = viewModel()) {
    val pc = LocalPilottyColors.current
    val ui by vm.state.collectAsStateWithLifecycle()
    var input by remember { mutableStateOf("") }
    val listState = rememberLazyListState()

    LaunchedEffect(ui.messages.size) {
        if (ui.messages.isNotEmpty()) listState.animateScrollToItem(ui.messages.size - 1)
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

        ui.error?.let {
            Text(
                "错误: $it",
                color = pc.error,
                modifier = Modifier.padding(horizontal = 20.dp, vertical = 8.dp),
            )
        }

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
                                    "via ${msg.source}",
                                    color = pc.ink4,
                                    fontSize = 10.sp,
                                    fontFamily = FontFamily.Monospace,
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
            if (!autoTestedOnce &&
                com.pilotty.app.PilottyCore.tunRunning.value &&
                n.isNotEmpty() && n.all { it.latency == 0 }) {
                autoTestedOnce = true
                runCatching { PilottyRepository.testLatencyAll() }
                val n2 = runCatching { PilottyRepository.nodes() }.getOrNull()
                if (n2 != null) _state.value = _state.value.copy(nodes = n2)
            }
        } catch (e: Throwable) {
            _state.value = _state.value.copy(loading = false, error = e.message)
        }
    }

    fun setQuery(q: String) { _state.value = _state.value.copy(query = q) }

    fun switchTo(tag: String) = viewModelScope.launch {
        try { PilottyRepository.switchNode("proxy-group", tag); refresh() }
        catch (e: Throwable) { _state.value = _state.value.copy(error = e.message) }
    }

    fun testAll() = viewModelScope.launch {
        try { PilottyRepository.testLatencyAll(); refresh() }
        catch (e: Throwable) { _state.value = _state.value.copy(error = e.message) }
    }
}

@Composable
fun NodesScreen(vm: NodesViewModel = viewModel()) {
    val pc = LocalPilottyColors.current
    val ui by vm.state.collectAsStateWithLifecycle()
    val filtered = remember(ui.nodes, ui.query) {
        val sorted = ui.nodes.sortedWith(compareBy { if (it.latency == 0) Int.MAX_VALUE else it.latency })
        if (ui.query.isBlank()) sorted
        else sorted.filter {
            it.tag.contains(ui.query, true) || it.server.contains(ui.query, true)
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
            Text("错误: $it", color = pc.error, fontSize = 12.sp)
            Spacer(Modifier.height(4.dp))
        }
        if (ui.loading) {
            LinearProgressIndicator(
                modifier = Modifier.fillMaxWidth(),
                color = pc.accent,
                trackColor = pc.surface3,
            )
            Spacer(Modifier.height(4.dp))
        }

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
                    text = if (node.latency > 0) "${node.latency}ms" else "—",
                    color = when {
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
            _state.value = _state.value.copy(loading = false, rules = r, templates = t)
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

    fun dismissToast() { _state.value = _state.value.copy(toast = null) }
}

@Composable
fun RulesSection(vm: RulesViewModel = viewModel()) {
    val pc = LocalPilottyColors.current
    val ui by vm.state.collectAsStateWithLifecycle()

    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        ui.error?.let { Text("错误: $it", color = pc.error, fontSize = 12.sp) }

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
        Kicker("Active rules · ${ui.rules.size}")
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
                        )
                        Text(
                            r.source,
                            color = pc.ink4,
                            fontSize = 10.sp,
                            fontFamily = FontFamily.Monospace,
                        )
                    }
                    Spacer(Modifier.height(4.dp))
                    val parts = buildList {
                        if (r.domainSuffix.isNotEmpty()) add("domain×${r.domainSuffix.size}")
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
}
