package com.pilotty.app.ui

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.drawscope.Stroke
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
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.LiveDot
import com.pilotty.app.ui.components.PilottyButton
import com.pilotty.app.ui.components.PilottyButtonVariant
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.components.PilottyChip
import com.pilotty.app.ui.components.StaticDot
import com.pilotty.app.ui.theme.LocalPilottyColors
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/* ---------- DashboardViewModel (保留原业务逻辑) ---------- */

data class HomeUi(
    val loading: Boolean = false,
    val status: StatusDto? = null,
    val tunRunning: Boolean = false,
    val isConnecting: Boolean = false,
    val snapshots: List<com.pilotty.app.data.SnapshotDto> = emptyList(),
    val error: String? = null,
    val toast: String? = null,
)

class HomeViewModel : ViewModel() {
    private val _state = MutableStateFlow(HomeUi())
    val state: StateFlow<HomeUi> = _state.asStateFlow()

    init {
        refresh()
        viewModelScope.launch {
            com.pilotty.app.PilottyCore.tunRunning.collect { running ->
                // tunRunning 变 true → 清掉启动中 spinner。 变 false 时不清, 由超时兜底 / 用户再次点击。
                val wasRunning = _state.value.tunRunning
                _state.value = _state.value.copy(
                    tunRunning = running,
                    isConnecting = if (running) false else _state.value.isConnecting,
                )
                // running 翻转 (false→true 或 true→false) 触发 refresh, 拉 Clash API 的 currentNode /
                // mode / nodeCount / connections。 不 poll 的代价: 连接数 / mode 只在 running 切换 + 手动
                // 触发 setMode 时刷新, 自用场景够用, 避免 1Hz 轮询 Clash API 浪费电。
                if (running != wasRunning) refresh()
            }
        }
    }

    /**
     * 启动 VPN 并进入 "启动中" 视觉态。 实际 Intent 启动由上层 onStartVpn 处理 (需要 Activity
     * 手持 ActivityResultLauncher 请求 VpnService.prepare),ViewModel 这里只负责 UI state。
     * 8s 超时兜底: libbox 首次冷启动实测 5-10s, 超时只清 spinner 不当 error (真启动成功后
     * tunRunning collect 会纠正按钮文案为 "停止")。
     */
    fun beginConnecting() = viewModelScope.launch {
        _state.value = _state.value.copy(isConnecting = true)
        kotlinx.coroutines.delay(8_000)
        if (!_state.value.tunRunning) {
            _state.value = _state.value.copy(isConnecting = false)
        }
    }

    fun refresh() = viewModelScope.launch {
        _state.value = _state.value.copy(loading = true, error = null)
        try {
            val s = PilottyRepository.status()
            // snapshot 拉取失败不阻塞 status, 用空列表 fallback
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
        // Clash API 只在 VPN 运行时在 127.0.0.1:9090 可达, 否则 setMode 会撞 connection refused。
        // 与其让用户看到 Go 原始错误, 不如前置兜底提示。
        if (!com.pilotty.app.PilottyCore.tunRunning.value) {
            _state.value = _state.value.copy(toast = "请先启动 VPN 再切换代理模式")
            return@launch
        }
        try { PilottyRepository.setMode(mode); refresh() }
        catch (e: Throwable) { _state.value = _state.value.copy(error = e.message) }
    }

    /** D2 Safety card: 一键回滚到最新快照。 */
    fun rollbackLatest() = viewModelScope.launch {
        // Rollback 会通过 Clash API 还原 proxy-group 状态, VPN 未运行时必失败 → 友好提示。
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

/* ---------- Home Screen ---------- */

private val HOME_PLACEHOLDERS = listOf(
    "切到最快的日本节点",
    "让 Netflix 走代理",
    "为什么 YouTube 卡?",
    "诊断当前连接",
    "Route GitHub through Singapore",
)

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
    var phIdx by remember { mutableStateOf(0) }

    LaunchedEffect(Unit) {
        while (true) {
            delay(2600)
            if (input.isEmpty()) phIdx = (phIdx + 1) % HOME_PLACEHOLDERS.size
        }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(pc.bg)
            .padding(horizontal = 20.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Spacer(Modifier.height(4.dp))

        // Brand row + live status
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Box(
                    modifier = Modifier
                        .size(22.dp)
                        .clip(RoundedCornerShape(6.dp))
                        .background(pc.ink),
                    contentAlignment = Alignment.Center,
                ) {
                    // 品牌标志 —— 线条小上升图
                    Canvas(modifier = Modifier.size(12.dp)) {
                        val c = pc.bg
                        drawLine(c, Offset(0f, size.height * 0.67f), Offset(size.width * 0.25f, size.height * 0.33f), strokeWidth = 2.2f)
                        drawLine(c, Offset(size.width * 0.25f, size.height * 0.33f), Offset(size.width * 0.40f, size.height * 0.5f), strokeWidth = 2.2f)
                        drawLine(c, Offset(size.width * 0.40f, size.height * 0.5f), Offset(size.width * 0.6f, size.height * 0.3f), strokeWidth = 2.2f)
                        drawLine(c, Offset(size.width * 0.6f, size.height * 0.3f), Offset(size.width, size.height * 0.5f), strokeWidth = 2.2f)
                    }
                }
                Text("Pilotty", color = pc.ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, letterSpacing = (-0.15).sp)
            }
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                if (ui.tunRunning) LiveDot() else StaticDot(pc.ink4)
                Text(
                    text = ui.status?.currentNode?.ifEmpty { "未连接" } ?: "未连接",
                    color = pc.ink2,
                    fontSize = 11.5.sp,
                    fontWeight = FontWeight.Medium,
                    fontFamily = FontFamily.Monospace,
                )
            }
        }

        // Hero input
        Column {
            Kicker("Ask the agent", modifier = Modifier.padding(bottom = 8.dp))
            val focusedBorderColor = if (input.isNotEmpty()) pc.ink else pc.hairlineStrong
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(14.dp))
                    .background(pc.bg)
                    .border(1.dp, focusedBorderColor, RoundedCornerShape(14.dp))
                    .padding(horizontal = 14.dp, vertical = 12.dp),
            ) {
                Box(modifier = Modifier.fillMaxWidth()) {
                    BasicTextField(
                        value = input,
                        onValueChange = { input = it },
                        modifier = Modifier.fillMaxWidth(),
                        textStyle = TextStyle(
                            color = pc.ink,
                            fontSize = 18.sp,
                            fontWeight = FontWeight.Normal,
                            letterSpacing = (-0.2).sp,
                            lineHeight = 24.sp,
                        ),
                        cursorBrush = SolidColor(pc.ink),
                        maxLines = 3,
                        keyboardOptions = KeyboardOptions.Default,
                    )
                    if (input.isEmpty()) {
                        AnimatedContent(
                            targetState = phIdx,
                            transitionSpec = { fadeIn() togetherWith fadeOut() },
                            label = "placeholder-cycle",
                        ) { idx ->
                            Text(
                                HOME_PLACEHOLDERS[idx],
                                color = pc.ink4,
                                fontSize = 18.sp,
                                fontWeight = FontWeight.Normal,
                                letterSpacing = (-0.2).sp,
                            )
                        }
                    }
                }
                Spacer(Modifier.height(8.dp))
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        "↵ to send",
                        color = pc.ink3,
                        fontSize = 10.5.sp,
                        fontFamily = FontFamily.Monospace,
                        letterSpacing = 0.2.sp,
                    )
                    Box(
                        modifier = Modifier
                            .size(32.dp)
                            .clip(RoundedCornerShape(8.dp))
                            .background(if (input.isNotEmpty()) pc.accent else pc.surface3)
                            .clickable(enabled = input.isNotEmpty()) {
                                AgentQueryBus.post(input)
                                input = ""
                                onNavigateChat()
                            },
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(
                            "↑",
                            color = if (input.isNotEmpty()) pc.accentOnBg else pc.ink4,
                            fontSize = 16.sp,
                            fontWeight = FontWeight.Bold,
                        )
                    }
                }
            }

            // Quick action chips — 每个 chip 对应一个真能扔给 Agent 的自然语言 query。
            // query 字符串故意选了 internal/router/keywords.go 的 substring match 能命中的短语,
            // 保证在 LLM 未配置 apiKey 时本地 IntentRouter 也能直接执行, 形成 D1 差异化闭环。
            val chipQueries = listOf(
                "最快节点" to "切换节点 找个快的",       // → switch_best_node
                "Netflix 模式" to "netflix 分流",       // → apply_template
                "诊断卡顿" to "当前状态",              // → show_status
            )
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 12.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                chipQueries.forEach { (label, query) ->
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
        }

        // Status card (real data)
        val s = ui.status
        PilottyCard(modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.Top,
                ) {
                    Column(modifier = Modifier.weight(1f)) {
                        Kicker("Active · ${s?.mode ?: "—"} mode", modifier = Modifier.padding(bottom = 4.dp))
                        Text(
                            s?.currentNode?.ifEmpty { "未连接" } ?: "未连接",
                            color = pc.ink,
                            fontSize = 17.sp,
                            fontWeight = FontWeight.SemiBold,
                            letterSpacing = (-0.25).sp,
                        )
                        Spacer(Modifier.height(6.dp))
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            Text(
                                "节点 ${s?.nodeCount ?: 0}",
                                color = pc.ink3,
                                fontSize = 11.sp,
                                fontFamily = FontFamily.Monospace,
                            )
                            StaticDot(pc.ink4, size = 3)
                            Text(
                                "连接 ${s?.connections ?: 0}",
                                color = pc.ink3,
                                fontSize = 11.sp,
                                fontFamily = FontFamily.Monospace,
                            )
                            StaticDot(pc.ink4, size = 3)
                            Text(
                                "Agent ${if (s?.agentReady == true) "就绪" else "未启用"}",
                                color = if (s?.agentReady == true) pc.accentInk else pc.ink3,
                                fontSize = 11.sp,
                                fontFamily = FontFamily.Monospace,
                                fontWeight = if (s?.agentReady == true) FontWeight.Bold else FontWeight.Normal,
                            )
                        }
                    }
                }
            }
        }

        // VPN 控制 —— 单按钮占满一行, tunRunning/isConnecting 驱动文案 & 态切换。
        // 刷新按钮已删除: PilottyCore.tunRunning StateFlow 实时推到 ui.tunRunning, 无需手动刷。
        when {
            ui.tunRunning -> {
                PilottyButton(
                    text = "停止",
                    onClick = onStopVpn,
                    variant = PilottyButtonVariant.Outline,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            ui.isConnecting -> {
                PilottyButton(
                    text = "启动中...",
                    onClick = {},
                    variant = PilottyButtonVariant.Accent,
                    enabled = false,
                    modifier = Modifier.fillMaxWidth(),
                    leading = {
                        CircularProgressIndicator(
                            modifier = Modifier.size(14.dp),
                            strokeWidth = 2.dp,
                            color = pc.ink,
                        )
                    },
                )
            }
            else -> {
                PilottyButton(
                    text = "启动",
                    onClick = {
                        vm.beginConnecting()
                        onStartVpn()
                    },
                    variant = PilottyButtonVariant.Accent,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }

        // 模式切换
        Column {
            Kicker("Mode", modifier = Modifier.padding(bottom = 8.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                listOf("rule" to "规则", "global" to "全局", "direct" to "直连").forEach { (id, label) ->
                    PilottyChip(
                        text = label,
                        selected = s?.mode.equals(id, true),
                        onClick = { vm.setMode(id) },
                    )
                }
            }
        }

        // Safety card (D2) — 自动快照 + 一键回滚
        val latestSnap = ui.snapshots.firstOrNull()
        PilottyCard(modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(14.dp)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        Text("🛡", fontSize = 14.sp)
                        Text("Safety", color = pc.ink, fontSize = 13.5.sp, fontWeight = FontWeight.SemiBold)
                        Text(
                            "· ${ui.snapshots.size} snapshots",
                            color = pc.ink3,
                            fontSize = 11.sp,
                            fontFamily = FontFamily.Monospace,
                        )
                    }
                    PilottyButton(
                        text = "Rollback",
                        onClick = { vm.rollbackLatest() },
                        variant = PilottyButtonVariant.Accent,
                        small = true,
                        // 只有当存在快照 *且* VPN 运行时才启用: 回滚必须经 Clash API 重设 selector。
                        enabled = latestSnap != null && ui.tunRunning,
                    )
                }
                Spacer(Modifier.height(8.dp))
                if (latestSnap != null) {
                    Text(
                        "最近快照: ${latestSnap.id} · ${formatSnapTs(latestSnap.timestamp)}",
                        color = pc.ink2,
                        fontSize = 12.sp,
                        fontFamily = FontFamily.Monospace,
                    )
                    if (latestSnap.activeProxies.isNotEmpty()) {
                        Text(
                            latestSnap.activeProxies.entries.joinToString(" · ") { "${it.key}=${it.value}" },
                            color = pc.ink3,
                            fontSize = 11.sp,
                            fontFamily = FontFamily.Monospace,
                        )
                    }
                } else {
                    Text(
                        "还没有快照。自动快照在每次 Agent 或订阅写操作前生成。",
                        color = pc.ink3,
                        fontSize = 11.5.sp,
                    )
                }
            }
        }

        // 错误横幅: 带红色描边的卡片, 不再是裸文本, 避免跟随内容区渗在下方
        ui.error?.let { err ->
            Surface(
                color = pc.surface2,
                shape = RoundedCornerShape(12.dp),
                border = androidx.compose.foundation.BorderStroke(1.dp, pc.error),
                modifier = Modifier.fillMaxWidth(),
            ) {
                Row(
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        "✗ $err",
                        color = pc.error,
                        fontSize = 12.sp,
                        modifier = Modifier.weight(1f).padding(end = 8.dp),
                    )
                    Text(
                        "×",
                        color = pc.error,
                        fontSize = 16.sp,
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier.clickable { vm.dismissError() }.padding(horizontal = 6.dp),
                    )
                }
            }
        }
        // Toast: 中性色, 2.5s 自动消失
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
            LaunchedEffect(it) {
                kotlinx.coroutines.delay(2500)
                vm.dismissToast()
            }
        }
        Spacer(Modifier.height(96.dp)) // floating nav 留白
    }
}

/** Go 传回的 RFC3339 时间戳 → "04-22 18:35" 紧凑显示, 解析失败 fallback 原串。 */
private fun formatSnapTs(raw: String): String {
    if (raw.isEmpty()) return "—"
    return runCatching {
        val cleaned = raw.take(19).replace('T', ' ')
        cleaned.substring(5, 16)
    }.getOrDefault(raw.take(16))
}
