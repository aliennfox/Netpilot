package com.pilotty.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.compose.viewModel
import com.pilotty.app.data.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/* ---------- Dashboard ---------- */

data class DashboardUi(
    val loading: Boolean = false,
    val status: StatusDto? = null,
    val tunRunning: Boolean = false,
    val error: String? = null,
)

class DashboardViewModel : ViewModel() {
    private val _state = MutableStateFlow(DashboardUi())
    val state: StateFlow<DashboardUi> = _state.asStateFlow()

    init {
        refresh()
        // 订阅 PilottyCore.tunRunning StateFlow, VpnService 状态变化时 UI 自动刷新 (#M13)
        viewModelScope.launch {
            com.pilotty.app.PilottyCore.tunRunning.collect { running ->
                _state.value = _state.value.copy(tunRunning = running)
            }
        }
    }

    fun refresh() = viewModelScope.launch {
        _state.value = _state.value.copy(loading = true, error = null)
        try {
            val s = PilottyRepository.status()
            _state.value = _state.value.copy(
                loading = false,
                status = s,
                tunRunning = com.pilotty.app.PilottyCore.tunRunning.value,
            )
        } catch (e: Throwable) {
            _state.value = _state.value.copy(loading = false, error = e.message)
        }
    }

    fun setMode(mode: String) = viewModelScope.launch {
        try { PilottyRepository.setMode(mode); refresh() }
        catch (e: Throwable) { _state.value = _state.value.copy(error = e.message) }
    }
}

@Composable
fun DashboardScreen(
    onStartVpn: () -> Unit,
    onStopVpn: () -> Unit,
    vm: DashboardViewModel = viewModel(),
) {
    val ui by vm.state.collectAsStateWithLifecycle()
    Column(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        ui.error?.let { Text("错误: $it", color = MaterialTheme.colorScheme.error) }
        if (ui.loading) LinearProgressIndicator(modifier = Modifier.fillMaxWidth())

        ElevatedCard(modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                val s = ui.status
                Text("当前节点", style = MaterialTheme.typography.labelMedium)
                Text(
                    s?.currentNode?.ifEmpty { "—" } ?: "—",
                    style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.Bold,
                )
                Spacer(Modifier.height(4.dp))
                Text("模式: ${s?.mode ?: "—"}  ·  节点数: ${s?.nodeCount ?: 0}")
                Text("活跃连接: ${s?.connections ?: 0}")
                Text("⬆ ${s?.upload ?: 0} B   ⬇ ${s?.download ?: 0} B")
                Text(
                    "Agent: ${if (s?.agentReady == true) "就绪" else "未启用"}",
                    style = MaterialTheme.typography.bodySmall,
                )
                Text(
                    "TUN: ${if (ui.tunRunning) "运行中" else "未启动"}",
                    style = MaterialTheme.typography.bodySmall,
                )
            }
        }

        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = onStartVpn) { Text("启动 VPN") }
            OutlinedButton(onClick = onStopVpn) { Text("停止") }
            OutlinedButton(onClick = { vm.refresh() }) { Text("刷新") }
        }

        Text("代理模式", style = MaterialTheme.typography.titleMedium)
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            listOf("rule" to "规则", "global" to "全局", "direct" to "直连").forEach { (id, label) ->
                FilterChip(
                    selected = ui.status?.mode.equals(id, true),
                    onClick = { vm.setMode(id) },
                    label = { Text(label) },
                )
            }
        }
    }
}

/* ---------- Chat ---------- */

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
    val ui by vm.state.collectAsStateWithLifecycle()
    var input by remember { mutableStateOf("") }
    val listState = rememberLazyListState()

    LaunchedEffect(ui.messages.size) {
        if (ui.messages.isNotEmpty()) listState.animateScrollToItem(ui.messages.size - 1)
    }

    Column(modifier = Modifier.fillMaxSize().padding(12.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text("对话", style = MaterialTheme.typography.titleMedium)
            TextButton(onClick = { vm.clear() }) { Text("清空") }
        }
        ui.error?.let { Text("错误: $it", color = MaterialTheme.colorScheme.error) }

        LazyColumn(
            modifier = Modifier.weight(1f).fillMaxWidth(),
            state = listState,
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            items(ui.messages) { msg ->
                val isUser = msg.role == "user"
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = if (isUser) Arrangement.End else Arrangement.Start,
                ) {
                    Surface(
                        color = if (isUser) MaterialTheme.colorScheme.primaryContainer
                                else MaterialTheme.colorScheme.surfaceVariant,
                        shape = RoundedCornerShape(12.dp),
                    ) {
                        Column(Modifier.padding(10.dp)) {
                            Text(msg.text, style = MaterialTheme.typography.bodyMedium)
                            if (!isUser && msg.source.isNotEmpty()) {
                                Text(
                                    "via ${msg.source}",
                                    style = MaterialTheme.typography.labelSmall,
                                )
                            }
                        }
                    }
                }
            }
        }

        if (ui.sending) LinearProgressIndicator(modifier = Modifier.fillMaxWidth())

        Row(
            modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            OutlinedTextField(
                value = input,
                onValueChange = { input = it },
                modifier = Modifier.weight(1f),
                placeholder = { Text("跟 Agent 聊聊…") },
                singleLine = true,
                enabled = !ui.sending,
            )
            Button(
                onClick = { vm.send(input); input = "" },
                enabled = !ui.sending && input.isNotBlank(),
            ) { Text("发送") }
        }
    }
}

/* ---------- Nodes ---------- */

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
            // Clash API 的 history 只在显式 /proxies/{tag}/delay 后才填充, 所以首次拿到节点
            // 列表时若延迟都是 0 且 VPN 在跑, 自动触发一次测速 (~3-5s), 测完再刷新一次 UI。
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
    val ui by vm.state.collectAsStateWithLifecycle()
    val filtered = remember(ui.nodes, ui.query) {
        if (ui.query.isBlank()) ui.nodes
        else ui.nodes.filter {
            it.tag.contains(ui.query, true) || it.server.contains(ui.query, true)
        }
    }
    Column(modifier = Modifier.fillMaxSize().padding(12.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            OutlinedTextField(
                value = ui.query,
                onValueChange = { vm.setQuery(it) },
                modifier = Modifier.weight(1f),
                placeholder = { Text("搜索节点…") },
                singleLine = true,
            )
            Button(onClick = { vm.testAll() }) { Text("测速") }
            OutlinedButton(onClick = { vm.refresh() }) { Text("刷新") }
        }
        Spacer(Modifier.height(8.dp))
        ui.error?.let { Text("错误: $it", color = MaterialTheme.colorScheme.error) }
        if (ui.loading) LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
        Text("节点 (${filtered.size})", style = MaterialTheme.typography.labelLarge)
        LazyColumn(verticalArrangement = Arrangement.spacedBy(6.dp)) {
            items(filtered, key = { it.tag }) { node ->
                ListItem(
                    headlineContent = {
                        Text(
                            node.tag,
                            fontWeight = if (node.active) FontWeight.Bold else FontWeight.Normal,
                        )
                    },
                    supportingContent = { Text("${node.type} · ${node.server}:${node.port}") },
                    leadingContent = {
                        Surface(
                            color = MaterialTheme.colorScheme.secondaryContainer,
                            shape = RoundedCornerShape(6.dp),
                        ) {
                            Text(
                                node.type.uppercase(),
                                modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp),
                                style = MaterialTheme.typography.labelSmall,
                            )
                        }
                    },
                    trailingContent = {
                        Text(if (node.latency > 0) "${node.latency} ms" else "—")
                    },
                    modifier = Modifier.clickable { vm.switchTo(node.tag) },
                )
            }
        }
    }
}

/* ---------- Rules ---------- */

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
fun RulesScreen(vm: RulesViewModel = viewModel()) {
    val ui by vm.state.collectAsStateWithLifecycle()
    val snackbar = remember { SnackbarHostState() }
    LaunchedEffect(ui.toast) {
        ui.toast?.let { snackbar.showSnackbar(it); vm.dismissToast() }
    }
    Scaffold(snackbarHost = { SnackbarHost(snackbar) }) { pad ->
        Column(modifier = Modifier.fillMaxSize().padding(pad).padding(12.dp)) {
            ui.error?.let { Text("错误: $it", color = MaterialTheme.colorScheme.error) }
            if (ui.loading) LinearProgressIndicator(modifier = Modifier.fillMaxWidth())

            Text("分流模板", style = MaterialTheme.typography.titleMedium)
            LazyColumn(
                modifier = Modifier.heightIn(max = 240.dp),
                verticalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                items(ui.templates, key = { it.id }) { t ->
                    ElevatedCard(
                        modifier = Modifier.fillMaxWidth().clickable { vm.applyTemplate(t.id) },
                    ) {
                        Column(Modifier.padding(12.dp)) {
                            Text(t.name, fontWeight = FontWeight.Bold)
                            Text(t.description, style = MaterialTheme.typography.bodySmall)
                        }
                    }
                }
            }

            Spacer(Modifier.height(12.dp))
            HorizontalDivider()
            Spacer(Modifier.height(8.dp))
            Text("当前规则 (${ui.rules.size})", style = MaterialTheme.typography.titleMedium)
            LazyColumn(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                items(ui.rules, key = { it.tag.ifEmpty { it.description } + it.outbound }) { r ->
                    ListItem(
                        headlineContent = { Text(r.description.ifEmpty { r.tag }) },
                        supportingContent = {
                            val parts = buildList {
                                if (r.domainSuffix.isNotEmpty()) add("domain×${r.domainSuffix.size}")
                                if (r.ipCidr.isNotEmpty()) add("ip×${r.ipCidr.size}")
                                if (r.processName.isNotEmpty()) add("proc×${r.processName.size}")
                            }
                            Text("→ ${r.outbound}  ${parts.joinToString("  ")}")
                        },
                        trailingContent = {
                            Text(r.source, style = MaterialTheme.typography.labelSmall)
                        },
                    )
                }
            }
        }
    }
}
