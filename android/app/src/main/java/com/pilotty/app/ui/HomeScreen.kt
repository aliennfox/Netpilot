package com.pilotty.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
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
import com.pilotty.app.data.PilottyRepository
import com.pilotty.app.data.StatusDto
import com.pilotty.app.ui.agent.AgentQueryBus
import com.pilotty.app.ui.components.BarMini
import com.pilotty.app.ui.components.CardGroup
import com.pilotty.app.ui.components.FlagChip
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.LatencyRing
import com.pilotty.app.ui.components.MissionStrip
import com.pilotty.app.ui.components.PilottyButton
import com.pilotty.app.ui.components.PilottyButtonVariant
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.components.PilottyChip
import com.pilotty.app.ui.components.ProtoBadge
import com.pilotty.app.ui.components.RowDivider
import com.pilotty.app.ui.components.SectionHead
import com.pilotty.app.ui.components.Sparkline
import com.pilotty.app.ui.components.SparklineDual
import com.pilotty.app.ui.components.StatusDot
import com.pilotty.app.ui.components.TelemetryTile
import com.pilotty.app.ui.components.TraceDot
import com.pilotty.app.ui.theme.LocalPilottyColors
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/* ---------- Dashboard 数据状态 (保留原业务) ---------- */

data class HomeUi(
    val loading: Boolean = false,
    val status: StatusDto? = null,
    val tunRunning: Boolean = false,
    val isConnecting: Boolean = false,
    val snapshots: List<com.pilotty.app.data.SnapshotDto> = emptyList(),
    val error: String? = null,
    val toast: String? = null,
    val uptimeSec: Long = 0L,
)

class HomeViewModel : ViewModel() {
    private val _state = MutableStateFlow(HomeUi())
    val state: StateFlow<HomeUi> = _state.asStateFlow()

    init {
        refresh()
        viewModelScope.launch {
            com.pilotty.app.PilottyCore.tunRunning.collect { running ->
                val wasRunning = _state.value.tunRunning
                _state.value = _state.value.copy(
                    tunRunning = running,
                    isConnecting = if (running) false else _state.value.isConnecting,
                    uptimeSec = if (running) _state.value.uptimeSec else 0L,
                )
                if (running != wasRunning) refresh()
            }
        }
        // uptime 秒计数器, VPN 跑才累加
        viewModelScope.launch {
            while (true) {
                delay(1000)
                if (_state.value.tunRunning) {
                    _state.value = _state.value.copy(uptimeSec = _state.value.uptimeSec + 1)
                }
            }
        }
    }

    fun beginConnecting() = viewModelScope.launch {
        _state.value = _state.value.copy(isConnecting = true)
        delay(8_000)
        if (!_state.value.tunRunning) {
            _state.value = _state.value.copy(isConnecting = false)
        }
    }

    fun refresh() = viewModelScope.launch {
        _state.value = _state.value.copy(loading = true, error = null)
        try {
            val s = PilottyRepository.status()
            val snaps = runCatching { PilottyRepository.snapshots() }.getOrDefault(emptyList())
            _state.value = _state.value.copy(
                loading = false,
                status = s,
                snapshots = snaps,
                tunRunning = com.pilotty.app.PilottyCore.tunRunning.value,
            )
        } catch (e: Throwable) {
            _state.value = _state.value.copy(loading = false, error = e.message)
        }
    }

    fun setMode(mode: String) = viewModelScope.launch {
        if (!com.pilotty.app.PilottyCore.tunRunning.value) {
            _state.value = _state.value.copy(toast = "请先启动 VPN 再切换代理模式")
            return@launch
        }
        try { PilottyRepository.setMode(mode); refresh() }
        catch (e: Throwable) { _state.value = _state.value.copy(error = e.message) }
    }

    fun rollbackLatest() = viewModelScope.launch {
        if (!com.pilotty.app.PilottyCore.tunRunning.value) {
            _state.value = _state.value.copy(toast = "请先启动 VPN 再回滚")
            return@launch
        }
        try {
            val r = PilottyRepository.rollback("")
            _state.value = _state.value.copy(toast = r.message.ifEmpty { "已回滚" })
            refresh()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(error = e.message)
        }
    }

    fun dismissToast() { _state.value = _state.value.copy(toast = null) }
    fun dismissError() { _state.value = _state.value.copy(error = null) }
}

/* ---------- Mock telemetry series (真实数据源待接, 先给视觉) ---------- */
private val UP_SERIES = listOf(0.8f, 1.2f, 0.9f, 1.8f, 2.1f, 1.7f, 2.4f, 2.8f, 2.3f, 2.6f, 3.1f, 2.9f, 2.4f, 2.7f, 3.0f, 2.6f, 2.4f)
private val DOWN_SERIES = listOf(0.3f, 0.4f, 0.3f, 0.6f, 0.7f, 0.5f, 0.8f, 0.9f, 0.7f, 0.8f, 1.0f, 0.9f, 0.7f, 0.8f, 0.9f, 0.8f, 0.7f)
private val LAT_SERIES = listOf(44f, 42f, 45f, 41f, 43f, 40f, 42f, 45f, 43f, 41f, 42f, 44f, 43f, 41f, 42f, 43f, 42f)
private val CONN_SERIES = listOf(12f, 18f, 22f, 16f, 24f, 28f, 26f, 32f, 30f, 28f, 34f, 36f, 32f, 30f, 34f, 38f, 36f)
private val BAR_SERIES = listOf(3f, 5f, 4f, 6f, 8f, 7f, 9f, 11f, 10f, 12f, 14f, 12f, 10f, 11f, 13f, 12f, 10f)

private val QUICK_COMMANDS = listOf(
    "切到最快的日本节点" to "切换节点 找个快的",
    "让 Netflix 走代理" to "netflix 分流",
    "诊断当前连接" to "当前状态",
    "Route GitHub 走新加坡" to "把 github 走代理",
    "关闭自动切换" to "停止自动切换",
)

/**
 * Mission Control Home —— 设计稿 01/02 屏主布局。
 *   MissionStrip · Active Node hero · 2x2 Telemetry · Agent Log · Quick Commands · 粘性输入
 */
@Composable
fun HomeScreen(
    onStartVpn: () -> Unit,
    onStopVpn: () -> Unit,
    onNavigateChat: () -> Unit = {},
    vm: HomeViewModel = viewModel(),
) {
    val pc = LocalPilottyColors.current
    val ui by vm.state.collectAsStateWithLifecycle()
    var input by remember { mutableStateOf("") }
    val missionState = when {
        !ui.tunRunning -> "warn"
        else -> "nominal"
    }

    Column(modifier = Modifier.fillMaxSize().background(pc.bg)) {
        // 可滚动主区
        Column(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
                .verticalScroll(rememberScrollState()),
        ) {
            MissionStrip(
                state = missionState,
                vpn = ui.tunRunning,
                uptime = formatUptime(ui.uptimeSec),
            )

            // Hero — Active Node card
            Box(modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 16.dp)) {
                ActiveNodeCard(
                    nodeName = ui.status?.currentNode?.ifEmpty { "未连接" } ?: "未连接",
                    latency = parseLatencyMs(ui.status),
                    running = ui.tunRunning,
                    isConnecting = ui.isConnecting,
                    mode = (ui.status?.mode ?: "rule").uppercase(),
                    onPower = {
                        if (ui.tunRunning) onStopVpn() else { vm.beginConnecting(); onStartVpn() }
                    },
                    onSwitch = onNavigateChat,
                )
            }

            // Telemetry grid
            SectionHead(
                text = "Telemetry",
                modifier = Modifier.padding(start = 20.dp, end = 20.dp, top = 4.dp, bottom = 12.dp),
            )
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                TelemetryTile(
                    label = "Latency",
                    modifier = Modifier.weight(1f),
                ) {
                    Row(verticalAlignment = Alignment.Bottom) {
                        Text(
                            text = parseLatencyMs(ui.status).toString(),
                            color = pc.ink,
                            fontSize = 24.sp,
                            fontWeight = FontWeight.Bold,
                            letterSpacing = (-0.48).sp,
                            fontFamily = FontFamily.Monospace,
                        )
                        Spacer(Modifier.width(4.dp))
                        Text(
                            "ms",
                            color = pc.ink3,
                            fontSize = 11.sp,
                            fontWeight = FontWeight.Medium,
                        )
                        Spacer(Modifier.weight(1f))
                        Text(
                            "p95 48",
                            color = pc.ink3,
                            fontSize = 11.sp,
                            fontFamily = FontFamily.Monospace,
                        )
                    }
                    Spacer(Modifier.height(6.dp))
                    Sparkline(
                        data = LAT_SERIES,
                        modifier = Modifier.fillMaxWidth().height(22.dp),
                        color = pc.accent,
                    )
                }
                TelemetryTile(
                    label = "Throughput",
                    modifier = Modifier.weight(1f),
                ) {
                    Row(verticalAlignment = Alignment.Bottom) {
                        Text(
                            "3.2",
                            color = pc.ink,
                            fontSize = 24.sp,
                            fontWeight = FontWeight.Bold,
                            letterSpacing = (-0.48).sp,
                            fontFamily = FontFamily.Monospace,
                        )
                        Spacer(Modifier.width(4.dp))
                        Text(
                            "MB/s",
                            color = pc.ink3,
                            fontSize = 11.sp,
                            fontWeight = FontWeight.Medium,
                        )
                    }
                    Spacer(Modifier.height(6.dp))
                    BarMini(
                        data = BAR_SERIES,
                        modifier = Modifier.fillMaxWidth().height(22.dp),
                        color = pc.ink,
                    )
                }
            }
            Spacer(Modifier.height(10.dp))
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                TelemetryTile(
                    label = "Connections",
                    modifier = Modifier.weight(1f),
                ) {
                    Row(verticalAlignment = Alignment.Bottom) {
                        Text(
                            text = (ui.status?.connections ?: 0).toString(),
                            color = pc.ink,
                            fontSize = 24.sp,
                            fontWeight = FontWeight.Bold,
                            letterSpacing = (-0.48).sp,
                            fontFamily = FontFamily.Monospace,
                        )
                        Spacer(Modifier.width(4.dp))
                        Text(
                            "active",
                            color = pc.ink3,
                            fontSize = 11.sp,
                            fontWeight = FontWeight.Medium,
                        )
                    }
                    Spacer(Modifier.height(6.dp))
                    Sparkline(
                        data = CONN_SERIES,
                        modifier = Modifier.fillMaxWidth().height(22.dp),
                        color = pc.ink2,
                    )
                }
                TelemetryTile(
                    label = "Agent",
                    modifier = Modifier.weight(1f),
                ) {
                    Row(verticalAlignment = Alignment.Bottom) {
                        Text(
                            "24",
                            color = pc.ink,
                            fontSize = 24.sp,
                            fontWeight = FontWeight.Bold,
                            letterSpacing = (-0.48).sp,
                            fontFamily = FontFamily.Monospace,
                        )
                        Spacer(Modifier.width(4.dp))
                        Text(
                            "tool calls / hr",
                            color = pc.ink3,
                            fontSize = 11.sp,
                            fontWeight = FontWeight.Medium,
                        )
                    }
                    Spacer(Modifier.height(6.dp))
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(5.dp),
                    ) {
                        StatusDot(state = if (ui.status?.agentReady == true) "nominal" else "warn")
                        Text(
                            text = if (ui.status?.agentReady == true) "deepseek-chat · 64K ctx" else "未配置 apiKey",
                            color = pc.ink3,
                            fontSize = 11.sp,
                            fontFamily = FontFamily.Monospace,
                        )
                    }
                }
            }

            // Agent Log
            SectionHead(
                text = "Agent Log · Last 3",
                modifier = Modifier.padding(start = 20.dp, end = 20.dp, top = 20.dp, bottom = 12.dp),
            )
            Box(modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp)) {
                AgentLogCard(
                    snapCount = ui.snapshots.size,
                    snapTime = formatRelativeTime(ui.snapshots.firstOrNull()?.timestamp),
                    onRollback = { vm.rollbackLatest() },
                )
            }

            // Quick Commands — 横滑 chips
            SectionHead(
                text = "Quick Commands",
                modifier = Modifier.padding(start = 20.dp, end = 20.dp, top = 20.dp, bottom = 12.dp),
            )
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState())
                    .padding(horizontal = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                QUICK_COMMANDS.forEach { (label, query) ->
                    PilottyChip(
                        text = label,
                        selected = false,
                        onClick = {
                            AgentQueryBus.post(query)
                            onNavigateChat()
                        },
                    )
                }
            }
            Spacer(Modifier.height(16.dp))

            // Mode chips (保留原业务)
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                listOf("rule" to "规则", "global" to "全局", "direct" to "直连").forEach { (id, label) ->
                    PilottyChip(
                        text = label,
                        selected = (ui.status?.mode ?: "").equals(id, ignoreCase = true),
                        onClick = { vm.setMode(id) },
                    )
                }
            }

            ui.error?.let {
                Text(
                    "✗ $it",
                    color = pc.error,
                    fontSize = 12.sp,
                    modifier = Modifier
                        .padding(16.dp)
                        .clickable { vm.dismissError() },
                )
            }
            ui.toast?.let {
                Text(
                    it,
                    color = pc.ink2,
                    fontSize = 12.sp,
                    modifier = Modifier.padding(16.dp),
                )
                LaunchedEffect(it) {
                    delay(2500)
                    vm.dismissToast()
                }
            }

            Spacer(Modifier.height(80.dp))
        }

        // 粘性自然语言输入栏 —— 挂在 bottom, 位于 BottomNav 之上
        Surface(
            color = pc.bg,
            shadowElevation = 0.dp,
            modifier = Modifier.fillMaxWidth(),
        ) {
            Column {
                RowDivider()
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 16.dp, vertical = 10.dp),
                ) {
                    Surface(
                        color = pc.surface,
                        shape = RoundedCornerShape(16.dp),
                        shadowElevation = if (pc.isDark) 0.dp else 4.dp,
                        border = if (pc.isDark) androidx.compose.foundation.BorderStroke(1.dp, pc.hairline) else null,
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Row(
                            modifier = Modifier.padding(start = 14.dp, end = 10.dp, top = 10.dp, bottom = 10.dp),
                            verticalAlignment = Alignment.Bottom,
                        ) {
                            Box(modifier = Modifier.weight(1f).padding(top = 2.dp)) {
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
                                )
                                if (input.isEmpty()) {
                                    Text(
                                        "告诉 Agent 你想做什么…",
                                        color = pc.ink3,
                                        fontSize = 14.5.sp,
                                    )
                                }
                            }
                            Spacer(Modifier.width(8.dp))
                            Box(
                                modifier = Modifier
                                    .size(36.dp)
                                    .clip(RoundedCornerShape(12.dp))
                                    .background(if (input.isNotEmpty()) pc.accent else pc.surface2)
                                    .clickable(enabled = input.isNotEmpty()) {
                                        AgentQueryBus.post(input)
                                        input = ""
                                        onNavigateChat()
                                    },
                                contentAlignment = Alignment.Center,
                            ) {
                                Text(
                                    "↑",
                                    color = if (input.isNotEmpty()) pc.accentOnBg else pc.ink3,
                                    fontSize = 16.sp,
                                    fontWeight = FontWeight.Bold,
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

/* ---------- Hero Active Node card ---------- */

@Composable
private fun ActiveNodeCard(
    nodeName: String,
    latency: Int,
    running: Boolean,
    isConnecting: Boolean,
    mode: String,
    onPower: () -> Unit,
    onSwitch: () -> Unit,
) {
    val pc = LocalPilottyColors.current
    PilottyCard(modifier = Modifier.fillMaxWidth()) {
        Column(Modifier.padding(20.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Kicker("Active Node")
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    StatusDot(state = if (running) "nominal" else "warn")
                    Box(
                        modifier = Modifier
                            .clip(RoundedCornerShape(4.dp))
                            .background(pc.accent)
                            .padding(horizontal = 8.dp, vertical = 3.dp),
                    ) {
                        Text(
                            mode,
                            color = pc.accentOnBg,
                            fontSize = 11.sp,
                            fontWeight = FontWeight.Bold,
                            letterSpacing = 1.32.sp,
                        )
                    }
                }
            }
            Spacer(Modifier.height(14.dp))
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                LatencyRing(ms = if (running) latency else 0, size = 72)
                Column(
                    modifier = Modifier.weight(1f),
                    verticalArrangement = Arrangement.spacedBy(4.dp),
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        FlagChip(code = guessCountryCode(nodeName))
                        Text(
                            nodeName,
                            color = pc.ink,
                            fontSize = 18.sp,
                            fontWeight = FontWeight.SemiBold,
                            letterSpacing = (-0.27).sp,
                            maxLines = 1,
                        )
                    }
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        ProtoBadge(name = "VLESS")
                        Text(
                            "jp-01.net:443",
                            color = pc.ink3,
                            fontSize = 11.sp,
                            fontFamily = FontFamily.Monospace,
                            maxLines = 1,
                        )
                    }
                }
            }
            // Traffic row
            Spacer(Modifier.height(16.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.Bottom,
            ) {
                Column {
                    Kicker("Traffic · 60s", modifier = Modifier.padding(bottom = 4.dp))
                    Row(verticalAlignment = Alignment.Bottom) {
                        Text("2.4", color = pc.ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, fontFamily = FontFamily.Monospace)
                        Text(" MB/s ↑  ", color = pc.ink3, fontSize = 11.sp)
                        Text("0.8", color = pc.ink2, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, fontFamily = FontFamily.Monospace)
                        Text(" MB/s ↓", color = pc.ink3, fontSize = 11.sp)
                    }
                }
                SparklineDual(
                    up = UP_SERIES,
                    down = DOWN_SERIES,
                    modifier = Modifier.size(width = 110.dp, height = 32.dp),
                )
            }
            Spacer(Modifier.height(16.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                PilottyButton(
                    text = if (isConnecting) "连接中…" else if (running) "断开" else "启动",
                    onClick = onPower,
                    variant = PilottyButtonVariant.Power,
                    enabled = !isConnecting,
                    modifier = Modifier.weight(1f),
                )
                PilottyButton(
                    text = "切换节点",
                    onClick = onSwitch,
                    variant = PilottyButtonVariant.Ghost,
                    modifier = Modifier.weight(1f),
                )
            }
        }
    }
}

/* ---------- Agent Log card ---------- */

@Composable
private fun AgentLogCard(
    snapCount: Int,
    snapTime: String,
    onRollback: () -> Unit,
) {
    val pc = LocalPilottyColors.current
    val logs = listOf(
        Triple("09:38:12", "switch_node", "target=jp-01") to ("ok" to "440ms"),
        Triple("09:38:09", "test_latency", "candidates=3") to ("ok" to "1.2s"),
        Triple("09:38:07", "list_nodes", "region=JP sort=latency") to ("ok" to "180ms"),
    )
    CardGroup(modifier = Modifier.fillMaxWidth()) {
        logs.forEachIndexed { i, (meta, result) ->
            val (ts, tool, args) = meta
            val (status, dur) = result
            val bg = if (i % 2 == 1) pc.stripe else androidx.compose.ui.graphics.Color.Transparent
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(bg)
                    .padding(horizontal = 16.dp, vertical = 10.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                TraceDot(state = status)
                Text(
                    ts,
                    color = pc.ink3,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                    modifier = Modifier.width(56.dp),
                )
                Text(
                    tool,
                    color = pc.ink,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.SemiBold,
                    fontFamily = FontFamily.Monospace,
                )
                Text(
                    "($args)",
                    color = pc.ink2,
                    fontSize = 11.5.sp,
                    fontFamily = FontFamily.Monospace,
                    modifier = Modifier.weight(1f),
                    maxLines = 1,
                )
                Text(
                    dur,
                    color = pc.ink3,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                )
            }
        }
        RowDivider()
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 10.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                if (snapCount > 0) "最近快照 · $snapTime" else "暂无快照",
                color = pc.ink3,
                fontSize = 11.5.sp,
            )
            PilottyButton(
                text = "回滚",
                onClick = onRollback,
                variant = PilottyButtonVariant.Mono,
                enabled = snapCount > 0,
            )
        }
    }
}

/* ---------- 小工具 ---------- */

private fun parseLatencyMs(s: StatusDto?): Int {
    // StatusDto 的 currentNode 格式 "TAG · Xms" — 只提数字
    val raw = s?.currentNode ?: return 0
    val m = Regex("(\\d+)\\s*ms").find(raw) ?: return 42
    return m.groupValues[1].toIntOrNull() ?: 42
}

private fun guessCountryCode(name: String): String {
    val lower = name.lowercase()
    return when {
        "东京" in name || "大阪" in name || "日本" in name || "japan" in lower || "tokyo" in lower -> "JP"
        "香港" in name || "hong kong" in lower || "hk-" in lower -> "HK"
        "新加坡" in name || "singapore" in lower || "sg-" in lower -> "SG"
        "美国" in name || "san" in lower || "us-" in lower || "los angeles" in lower -> "US"
        "伦敦" in name || "london" in lower || "uk-" in lower -> "UK"
        "德国" in name || "frankfurt" in lower || "de-" in lower -> "DE"
        else -> "••"
    }
}

private fun formatUptime(sec: Long): String {
    if (sec <= 0) return "00:00:00"
    val h = sec / 3600
    val m = (sec % 3600) / 60
    val s = sec % 60
    return "%02d:%02d:%02d".format(h, m, s)
}

private fun formatRelativeTime(raw: String?): String {
    if (raw.isNullOrEmpty()) return "—"
    return runCatching {
        val cleaned = raw.take(19).replace('T', ' ')
        cleaned.substring(5, 16)
    }.getOrDefault("—")
}
