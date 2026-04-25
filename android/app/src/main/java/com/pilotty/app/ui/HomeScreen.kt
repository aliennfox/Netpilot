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
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.res.stringResource
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
    val pendingRollback: com.pilotty.app.data.SnapshotDto? = null,
    val error: String? = null,
    val toast: String? = null,
    val uptimeSec: Long = 0L,
    // 实时 telemetry (替代原 mock 常量 SERIES)
    // trafficSamples: 最近 60 秒流量采样, 驱动 SparklineDual / Throughput BarMini / 数字
    val trafficSamples: List<com.pilotty.app.data.TrafficSampleDto> = emptyList(),
    // connectionsHistory: 最近 60 个数据点, connections count 滑动窗口, 驱动 Connections Sparkline
    val connectionsHistory: List<Int> = emptyList(),
    // latencyHistory: 最近 60 个延迟采样, 驱动 Latency Sparkline
    val latencyHistory: List<Int> = emptyList(),
    // activeProto: 当前 active 节点的协议 (vless / trojan / ...), ActiveNodeCard ProtoBadge 用
    val activeProto: String = "",
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
        // Telemetry 真数据接入 (替代 Mission Control 重写时引入的 5 个 mock SERIES 常量)
        // 1Hz traffic 采样 — 驱动 ActiveNode SparklineDual + Throughput tile 数字&BarMini
        viewModelScope.launch {
            while (true) {
                delay(1000)
                if (!_state.value.tunRunning) continue
                runCatching { PilottyRepository.trafficHistory(60) }
                    .onSuccess { _state.value = _state.value.copy(trafficSamples = it) }
            }
        }
        // 2s connections 采样 — 驱动 Connections tile Sparkline + status 字段刷新
        // (Connections tile 数字读 ui.status?.connections, status 只在 refresh() 里
        //  拉一次永远冻结, 所以同一个协程里顺手刷下 status)
        viewModelScope.launch {
            while (true) {
                delay(2000)
                if (!_state.value.tunRunning) continue
                runCatching { PilottyRepository.connections() }
                    .onSuccess { conns ->
                        val hist = (_state.value.connectionsHistory + conns.size).takeLast(60)
                        _state.value = _state.value.copy(connectionsHistory = hist)
                    }
                // 顺带刷 status (current_node / mode / connections count / upload/download 累计)
                // 不用独立协程避免两个 poll 打架, 2s 频率对 Telemetry 够了
                runCatching { PilottyRepository.status() }
                    .onSuccess { _state.value = _state.value.copy(status = it) }
            }
        }
        // 5s latency + active proto 采样 — 驱动 Latency Sparkline + ActiveNode ProtoBadge
        viewModelScope.launch {
            while (true) {
                delay(5000)
                if (!_state.value.tunRunning) continue
                runCatching { PilottyRepository.nodes() }
                    .onSuccess { nodes ->
                        val active = nodes.firstOrNull { it.active }
                        if (active != null) {
                            val lat = active.latency.coerceAtLeast(0)
                            val hist = (_state.value.latencyHistory + lat).takeLast(60)
                            _state.value = _state.value.copy(
                                latencyHistory = hist,
                                activeProto = active.type,
                            )
                        }
                    }
            }
        }
        // 30s 主动测速 current node — sing-box 不会自动持续测延迟, /proxies/<tag>/delay
        // 必须显式调用才会发真实 HTTP 请求测, 结果写入 Clash API 缓存供 nodes() 下次拉取
        // 频率 30s 权衡: 太频 (< 10s) 会白烧流量; 太疏 (> 60s) Latency Sparkline 平线感明显
        viewModelScope.launch {
            while (true) {
                delay(30_000)
                if (!_state.value.tunRunning) continue
                val currentTag = _state.value.status?.currentNode ?: continue
                if (currentTag.isEmpty()) continue
                runCatching { PilottyRepository.testLatency(currentTag) }
                // 结果异步, 下一次 5s nodes() poll 会捞到新 latency 值塞进 latencyHistory
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
            _state.value = _state.value.copy(toast = r.message.ifEmpty { "已回滚到最新快照" })
            refresh()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(error = e.message)
        }
    }

    /** 用户在 Safety card 点某条快照 → 弹 confirm Dialog (pendingRollback 非空时显示) */
    fun requestRollback(snap: com.pilotty.app.data.SnapshotDto) {
        _state.value = _state.value.copy(pendingRollback = snap)
    }

    fun cancelRollback() { _state.value = _state.value.copy(pendingRollback = null) }

    // needVpnMsg / successFmt 由 Compose 层从 stringResource 透传, 因为 VM 里没有 Context
    fun confirmRollback(needVpnMsg: String, successFmt: (String) -> String) = viewModelScope.launch {
        val snap = _state.value.pendingRollback ?: return@launch
        _state.value = _state.value.copy(pendingRollback = null)
        if (!com.pilotty.app.PilottyCore.tunRunning.value) {
            _state.value = _state.value.copy(toast = needVpnMsg)
            return@launch
        }
        try {
            val r = PilottyRepository.rollback(snap.id)
            _state.value = _state.value.copy(toast = r.message.ifEmpty { successFmt(snap.id) })
            refresh()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(error = e.message)
        }
    }

    fun dismissToast() { _state.value = _state.value.copy(toast = null) }
    fun dismissError() { _state.value = _state.value.copy(error = null) }
}

// @VisualOnly: Quick Commands 是静态入口 chip, 不绑后端数据 (点后 post 到 AgentQueryBus 驱动 Chat)
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
    onNavigateLogs: () -> Unit = {},
    vm: HomeViewModel = viewModel(),
) {
    val pc = LocalPilottyColors.current
    val ui by vm.state.collectAsStateWithLifecycle()
    // tab 切回 Home 时 NavHost 重新 composition, 触发一次 refresh 拉最新 status / snapshots.
    // 不依赖 ViewModel.init 单次跑 — 用户在 Chat 触发的新 snapshot 切回 Home 才能立即看到.
    LaunchedEffect(Unit) { vm.refresh() }
    var input by remember { mutableStateOf("") }
    // 点发送 ↑ 跳 Chat 时需要主动收 IME — 否则 MainActivity 里 imeVisible=true 会隐藏
    // 底部 nav, 用户在 Chat 页看不到 Home tab, 回不来 (2026-04-24 用户反馈的 bug)
    val focusManager = LocalFocusManager.current
    // i18n: 在 Composable scope 提前捕获, 下方 lambda / 嵌套 Composable 共用
    val notConnectedText = stringResource(com.pilotty.app.R.string.home_not_connected)
    val chatPlaceholder = stringResource(com.pilotty.app.R.string.home_chat_placeholder)
    // M9: VPN 异常退出 banner (仅非 USER 停 + 未 ack 时显示)
    val ctx = androidx.compose.ui.platform.LocalContext.current
    val stopReasonPrefs = remember { com.pilotty.app.vpn.VpnStopReasonPrefs.get(ctx) }
    val stopSnap by stopReasonPrefs.state.collectAsStateWithLifecycle()
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

            // M9: VPN 异常退出 banner (只在非 USER 停 + 未 ack 时显示)
            if (!stopSnap.acked && stopSnap.reason != com.pilotty.app.vpn.VpnStopReasonPrefs.Reason.NONE
                && stopSnap.reason != com.pilotty.app.vpn.VpnStopReasonPrefs.Reason.USER) {
                VpnStopBanner(
                    snap = stopSnap,
                    onAck = { stopReasonPrefs.ack() },
                    onViewLogs = {
                        stopReasonPrefs.ack()
                        onNavigateLogs()
                    },
                )
            }

            // Hero — Active Node card
            Box(modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 16.dp)) {
                ActiveNodeCard(
                    nodeName = ui.status?.currentNode?.ifEmpty { notConnectedText } ?: notConnectedText,
                    latency = ui.latencyHistory.lastOrNull() ?: 0,
                    running = ui.tunRunning,
                    isConnecting = ui.isConnecting,
                    mode = (ui.status?.mode ?: "rule").uppercase(),
                    proto = ui.activeProto,
                    trafficSamples = ui.trafficSamples,
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
                    val latSeries = ui.latencyHistory.map { it.toFloat() }
                    val p95 = computeP95(ui.latencyHistory)
                    val lastLat = ui.latencyHistory.lastOrNull() ?: 0
                    Row(verticalAlignment = Alignment.Bottom) {
                        Text(
                            text = lastLat.toString(),
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
                        if (p95 > 0) {
                            Text(
                                "p95 $p95",
                                color = pc.ink3,
                                fontSize = 11.sp,
                                fontFamily = FontFamily.Monospace,
                            )
                        }
                    }
                    Spacer(Modifier.height(6.dp))
                    Sparkline(
                        data = latSeries,
                        modifier = Modifier.fillMaxWidth().height(22.dp),
                        color = pc.accent,
                    )
                }
                TelemetryTile(
                    label = "Throughput",
                    modifier = Modifier.weight(1f),
                ) {
                    val lastSample = ui.trafficSamples.lastOrNull()
                    val totalRate = (lastSample?.downRate ?: 0) + (lastSample?.upRate ?: 0)
                    val (rateNum, rateUnit) = formatRate(totalRate)
                    val downSeries = ui.trafficSamples.map { it.downRate.toFloat() }
                    Row(verticalAlignment = Alignment.Bottom) {
                        Text(
                            rateNum,
                            color = pc.ink,
                            fontSize = 24.sp,
                            fontWeight = FontWeight.Bold,
                            letterSpacing = (-0.48).sp,
                            fontFamily = FontFamily.Monospace,
                        )
                        Spacer(Modifier.width(4.dp))
                        Text(
                            rateUnit,
                            color = pc.ink3,
                            fontSize = 11.sp,
                            fontWeight = FontWeight.Medium,
                        )
                    }
                    Spacer(Modifier.height(6.dp))
                    BarMini(
                        data = downSeries,
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
                    val connSeries = ui.connectionsHistory.map { it.toFloat() }
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
                        data = connSeries,
                        modifier = Modifier.fillMaxWidth().height(22.dp),
                        color = pc.ink2,
                    )
                }
                TelemetryTile(
                    label = "Agent",
                    modifier = Modifier.weight(1f),
                ) {
                    val agentReady = ui.status?.agentReady == true
                    Row(verticalAlignment = Alignment.Bottom) {
                        Text(
                            if (agentReady) "ON" else "OFF",
                            color = if (agentReady) pc.ink else pc.ink3,
                            fontSize = 24.sp,
                            fontWeight = FontWeight.Bold,
                            letterSpacing = (-0.48).sp,
                            fontFamily = FontFamily.Monospace,
                        )
                        Spacer(Modifier.width(4.dp))
                        Text(
                            if (agentReady) "LLM ready" else stringResource(com.pilotty.app.R.string.home_agent_local_route),
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
                        StatusDot(state = if (agentReady) "nominal" else "warn")
                        Text(
                            text = if (agentReady) "deepseek-chat" else stringResource(com.pilotty.app.R.string.home_agent_no_apikey),
                            color = pc.ink3,
                            fontSize = 11.sp,
                            fontFamily = FontFamily.Monospace,
                        )
                    }
                }
            }

            // Safety · 最近变更 (D2)
            SectionHead(
                text = stringResource(com.pilotty.app.R.string.home_safety_title),
                modifier = Modifier.padding(start = 20.dp, end = 20.dp, top = 20.dp, bottom = 12.dp),
            )
            Box(modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp)) {
                AgentLogCard(
                    snapshots = ui.snapshots,
                    onRollbackAt = { vm.requestRollback(it) },
                    onRollbackLatest = { vm.rollbackLatest() },
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
                val modeItems = listOf(
                    "rule" to stringResource(com.pilotty.app.R.string.mode_rule),
                    "global" to stringResource(com.pilotty.app.R.string.mode_global),
                    "direct" to stringResource(com.pilotty.app.R.string.mode_direct),
                )
                modeItems.forEach { (id, label) ->
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

        // D2 Safety: 回滚确认 Dialog, 点某条 snapshot 触发 vm.requestRollback()
        ui.pendingRollback?.let { snap ->
            val primary = snap.activeProxies["proxy-group"] ?: snap.activeProxies.values.firstOrNull().orEmpty()
            androidx.compose.material3.AlertDialog(
                onDismissRequest = { vm.cancelRollback() },
                containerColor = pc.surface,
                title = { Text(stringResource(com.pilotty.app.R.string.home_rollback_confirm_title), color = pc.ink) },
                text = {
                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(
                            stringResource(
                                com.pilotty.app.R.string.home_rollback_time_format,
                                formatClockTime(snap.timestamp),
                                snap.id.takeLast(8),
                            ),
                            color = pc.ink2,
                            fontSize = 12.sp,
                            fontFamily = FontFamily.Monospace,
                        )
                        if (primary.isNotEmpty()) {
                            Text(
                                stringResource(com.pilotty.app.R.string.home_rollback_active_format, primary),
                                color = pc.ink2,
                                fontSize = 12.sp,
                                fontFamily = FontFamily.Monospace,
                            )
                        }
                        Text(
                            stringResource(com.pilotty.app.R.string.home_rollback_warning),
                            color = pc.ink3,
                            fontSize = 11.5.sp,
                        )
                    }
                },
                confirmButton = {
                    val needVpnMsg = stringResource(com.pilotty.app.R.string.home_need_vpn_to_rollback)
                    val successFmtBase = stringResource(com.pilotty.app.R.string.home_rollback_success_format)
                    androidx.compose.material3.TextButton(onClick = {
                        vm.confirmRollback(needVpnMsg) { id -> successFmtBase.format(id) }
                    }) {
                        Text(stringResource(com.pilotty.app.R.string.action_rollback), color = pc.accentInk, fontWeight = FontWeight.SemiBold)
                    }
                },
                dismissButton = {
                    androidx.compose.material3.TextButton(onClick = { vm.cancelRollback() }) {
                        Text(stringResource(com.pilotty.app.R.string.action_cancel), color = pc.ink2)
                    }
                },
            )
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
                                        chatPlaceholder,
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
                                        focusManager.clearFocus() // 关 IME, 让底部 nav 在 Chat 屏里露出
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

/* ---------- M9 VPN 异常退出 banner ---------- */

@Composable
private fun VpnStopBanner(
    snap: com.pilotty.app.vpn.VpnStopReasonPrefs.Snapshot,
    onAck: () -> Unit,
    onViewLogs: () -> Unit,
) {
    val pc = LocalPilottyColors.current
    val reasonText = when (snap.reason) {
        com.pilotty.app.vpn.VpnStopReasonPrefs.Reason.REVOKE ->
            stringResource(com.pilotty.app.R.string.vpn_stop_reason_revoke)
        com.pilotty.app.vpn.VpnStopReasonPrefs.Reason.CRASH ->
            stringResource(com.pilotty.app.R.string.vpn_stop_reason_crash)
        com.pilotty.app.vpn.VpnStopReasonPrefs.Reason.CONFIG_FAIL ->
            stringResource(com.pilotty.app.R.string.vpn_stop_reason_config)
        else -> ""
    }
    val timeText = if (snap.ts > 0) {
        java.text.SimpleDateFormat("MM-dd HH:mm", java.util.Locale.getDefault()).format(java.util.Date(snap.ts))
    } else "—"
    Box(modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
        Surface(
            color = pc.surface,
            shape = androidx.compose.foundation.shape.RoundedCornerShape(12.dp),
            border = androidx.compose.foundation.BorderStroke(1.dp, pc.warn),
            modifier = Modifier.fillMaxWidth(),
        ) {
            Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text("⚠", color = pc.warn, fontSize = 16.sp, fontWeight = FontWeight.Bold)
                    Text(
                        stringResource(com.pilotty.app.R.string.vpn_stop_banner_title),
                        color = pc.ink,
                        fontSize = 14.sp,
                        fontWeight = FontWeight.SemiBold,
                    )
                }
                Text(
                    stringResource(com.pilotty.app.R.string.vpn_stop_banner_time_format, timeText, reasonText),
                    color = pc.ink2,
                    fontSize = 12.sp,
                    fontFamily = FontFamily.Monospace,
                    lineHeight = 16.sp,
                )
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Text(
                        stringResource(com.pilotty.app.R.string.vpn_stop_banner_view_logs),
                        color = pc.accentInk,
                        fontSize = 13.sp,
                        fontWeight = FontWeight.SemiBold,
                        modifier = Modifier.clickable { onViewLogs() }.padding(vertical = 4.dp),
                    )
                    Spacer(Modifier.weight(1f))
                    Text(
                        stringResource(com.pilotty.app.R.string.vpn_stop_banner_ack),
                        color = pc.ink3,
                        fontSize = 13.sp,
                        modifier = Modifier.clickable { onAck() }.padding(vertical = 4.dp),
                    )
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
    proto: String,
    trafficSamples: List<com.pilotty.app.data.TrafficSampleDto>,
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
                        ProtoBadge(name = proto.uppercase().ifEmpty { "—" })
                        val stateText = when {
                            running && latency > 0 -> "$latency ms"
                            running -> stringResource(com.pilotty.app.R.string.home_active_measuring)
                            else -> stringResource(com.pilotty.app.R.string.home_active_waiting)
                        }
                        Text(
                            stateText,
                            color = pc.ink3,
                            fontSize = 11.sp,
                            fontFamily = FontFamily.Monospace,
                            maxLines = 1,
                        )
                    }
                }
            }
            // Traffic row — 真实数据来自 trafficSamples; 取最后一秒 up/down rate + 60 点曲线
            Spacer(Modifier.height(16.dp))
            val lastSample = trafficSamples.lastOrNull()
            val (upNum, upUnit) = formatRate(lastSample?.upRate ?: 0)
            val (downNum, downUnit) = formatRate(lastSample?.downRate ?: 0)
            val upSeries = trafficSamples.map { it.upRate.toFloat() }
            val downSeries = trafficSamples.map { it.downRate.toFloat() }
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.Bottom,
            ) {
                Column {
                    Kicker("Traffic · 60s", modifier = Modifier.padding(bottom = 4.dp))
                    Row(verticalAlignment = Alignment.Bottom) {
                        Text(upNum, color = pc.ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, fontFamily = FontFamily.Monospace)
                        Text(" $upUnit ↑  ", color = pc.ink3, fontSize = 11.sp)
                        Text(downNum, color = pc.ink2, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, fontFamily = FontFamily.Monospace)
                        Text(" $downUnit ↓", color = pc.ink3, fontSize = 11.sp)
                    }
                }
                SparklineDual(
                    up = upSeries,
                    down = downSeries,
                    modifier = Modifier.size(width = 110.dp, height = 32.dp),
                )
            }
            Spacer(Modifier.height(16.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                val powerText = when {
                    isConnecting -> stringResource(com.pilotty.app.R.string.home_btn_connecting)
                    running -> stringResource(com.pilotty.app.R.string.home_btn_disconnect)
                    else -> stringResource(com.pilotty.app.R.string.home_btn_start_short)
                }
                PilottyButton(
                    text = powerText,
                    onClick = onPower,
                    variant = PilottyButtonVariant.Power,
                    enabled = !isConnecting,
                    modifier = Modifier.weight(1f),
                )
                PilottyButton(
                    text = stringResource(com.pilotty.app.R.string.home_btn_switch_node),
                    onClick = onSwitch,
                    variant = PilottyButtonVariant.Ghost,
                    modifier = Modifier.weight(1f),
                )
            }
        }
    }
}

/* ---------- Safety card (D2) ----------
 * 渲染真实 snapshots (由 Tool Pipeline 在每次写操作前保存)。
 * 每条一行: TraceDot · 时间 · 主节点 tag · "↩" 单条回滚。
 * 底部状态行 + "回滚最新" 大按钮。
 * 空态提示 "暂无快照", 按钮禁用。
 */
@Composable
private fun AgentLogCard(
    snapshots: List<com.pilotty.app.data.SnapshotDto>,
    onRollbackAt: (com.pilotty.app.data.SnapshotDto) -> Unit,
    onRollbackLatest: () -> Unit,
) {
    val pc = LocalPilottyColors.current
    val recent = snapshots.take(3)
    CardGroup(modifier = Modifier.fillMaxWidth()) {
        if (recent.isEmpty()) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 18.dp),
                horizontalArrangement = Arrangement.Center,
            ) {
                Text(
                    stringResource(com.pilotty.app.R.string.home_safety_empty),
                    color = pc.ink3,
                    fontSize = 11.5.sp,
                )
            }
        } else {
            recent.forEachIndexed { i, snap ->
                val bg = if (i % 2 == 1) pc.stripe else androidx.compose.ui.graphics.Color.Transparent
                // D2 升级: snap.tool 非空时直接显示友好名, 否则回落差分推断 (老 snapshot 无 tool 字段)
                val (tool, target) = if (snap.tool.isNotEmpty()) {
                    val friendlyName = com.pilotty.app.ui.friendlyToolName(snap.tool)
                    val primary = snap.activeProxies["proxy-group"]
                        ?: snap.activeProxies.values.firstOrNull().orEmpty()
                    friendlyName to primary.ifEmpty { "—" }
                } else {
                    inferActionFromSnapshot(snap, recent.getOrNull(i + 1))
                }
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .background(bg)
                        .clickable { onRollbackAt(snap) }
                        .padding(horizontal = 16.dp, vertical = 10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    TraceDot(state = "ok")
                    Text(
                        formatClockTime(snap.timestamp),
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
                        "($target)",
                        color = pc.ink2,
                        fontSize = 11.5.sp,
                        fontFamily = FontFamily.Monospace,
                        modifier = Modifier.weight(1f),
                        maxLines = 1,
                    )
                    Text(
                        "↩",
                        color = pc.ink3,
                        fontSize = 14.sp,
                        fontWeight = FontWeight.Bold,
                    )
                }
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
            val latestTime = formatRelativeTime(snapshots.firstOrNull()?.timestamp)
            val summary = if (snapshots.isNotEmpty())
                stringResource(com.pilotty.app.R.string.home_safety_summary_format, snapshots.size, latestTime)
            else
                stringResource(com.pilotty.app.R.string.home_safety_hint)
            Text(
                summary,
                color = pc.ink3,
                fontSize = 11.5.sp,
                maxLines = 1,
            )
            PilottyButton(
                text = stringResource(com.pilotty.app.R.string.home_rollback_latest),
                onClick = onRollbackLatest,
                variant = PilottyButtonVariant.Mono,
                enabled = snapshots.isNotEmpty(),
            )
        }
    }
}

/**
 * 从两个相邻 snapshot 对比 active_proxies, 推断用户做了什么:
 * - 发现 proxy-group 的值变了 → "switch_node(newTag)"
 * - 没变化或没 prev → "snapshot(当前 primary tag)"
 *
 * 实际 Go 侧 snapshot 只记 active_proxies, 不记 tool 名 ——
 * 这里做"差分推断"给用户更可读的标签。 未来 D3 通过 snapshot.tool 字段能直接拿到更准信息。
 */
private fun inferActionFromSnapshot(
    curr: com.pilotty.app.data.SnapshotDto,
    prev: com.pilotty.app.data.SnapshotDto?,
): Pair<String, String> {
    val currPrimary = curr.activeProxies["proxy-group"] ?: curr.activeProxies.values.firstOrNull().orEmpty()
    if (prev != null) {
        val prevPrimary = prev.activeProxies["proxy-group"] ?: prev.activeProxies.values.firstOrNull().orEmpty()
        if (prevPrimary.isNotEmpty() && currPrimary.isNotEmpty() && prevPrimary != currPrimary) {
            return "switch_node" to currPrimary
        }
    }
    return "snapshot" to currPrimary.ifEmpty { "initial" }
}

/** RFC3339 → "HH:mm:ss". 解析失败返回原串前 8 位, 避免空白。 */
private fun formatClockTime(ts: String): String {
    if (ts.isEmpty()) return "—"
    return try {
        val parser = java.text.SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss", java.util.Locale.getDefault()).apply {
            timeZone = java.util.TimeZone.getTimeZone("UTC")
            isLenient = true
        }
        val clean = ts.substringBefore('.').substringBefore('+').substringBefore('Z')
        val d = parser.parse(clean) ?: return ts.take(8)
        java.text.SimpleDateFormat("HH:mm:ss", java.util.Locale.getDefault()).format(d)
    } catch (_: Throwable) {
        ts.take(8)
    }
}

/* ---------- 小工具 ---------- */

// parseLatencyMs 删 — 历史 bug: Go 侧 status.currentNode 只是 tag 名 (如 "RN-San-Jose-VLESS"),
// 不含 "Xms" 后缀, regex 永远 miss → 永远返回默认 42。 UI 现在直接读
// ui.latencyHistory.lastOrNull() (由 5s nodes() poll 填充, 30s testLatency 真测速触发更新)

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

/** bytes/s → (数字文本, 单位文本); 自动在 B/KB/MB 档位间选, 避免小流量永远是 "0.0 MB/s" */
private fun formatRate(bytes: Long): Pair<String, String> {
    val b = bytes.coerceAtLeast(0)
    return when {
        b < 1024 -> b.toString() to "B/s"
        b < 1024 * 1024 -> "%.0f".format(b / 1024.0) to "KB/s"
        else -> "%.1f".format(b / (1024.0 * 1024.0)) to "MB/s"
    }
}

/** 近似 p95, 样本少 (< 20) 直接返回 max 更直观; 空返回 0 (调用方据此决定是否渲染) */
private fun computeP95(values: List<Int>): Int {
    if (values.isEmpty()) return 0
    if (values.size < 20) return values.max()
    val sorted = values.sorted()
    val idx = (sorted.size * 0.95).toInt().coerceAtMost(sorted.size - 1)
    return sorted[idx]
}

private fun formatRelativeTime(raw: String?): String {
    if (raw.isNullOrEmpty()) return "—"
    return runCatching {
        val cleaned = raw.take(19).replace('T', ' ')
        cleaned.substring(5, 16)
    }.getOrDefault("—")
}
