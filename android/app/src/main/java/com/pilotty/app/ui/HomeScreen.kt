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
    val error: String? = null,
)

class HomeViewModel : ViewModel() {
    private val _state = MutableStateFlow(HomeUi())
    val state: StateFlow<HomeUi> = _state.asStateFlow()

    init {
        refresh()
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

            // Quick action chips
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 12.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                listOf("最快节点", "Netflix 模式", "诊断卡顿").forEach { label ->
                    PilottyChip(
                        text = label,
                        selected = false,
                        onClick = onNavigateChat,
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

        // VPN 控制
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            if (ui.tunRunning) {
                PilottyButton(
                    text = "停止",
                    onClick = onStopVpn,
                    variant = PilottyButtonVariant.Outline,
                    modifier = Modifier.weight(1f),
                )
            } else {
                PilottyButton(
                    text = "启动 VPN",
                    onClick = onStartVpn,
                    variant = PilottyButtonVariant.Accent,
                    modifier = Modifier.weight(1f),
                )
            }
            PilottyButton(
                text = "刷新",
                onClick = { vm.refresh() },
                variant = PilottyButtonVariant.Outline,
            )
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

        // Safety card (stub 占位, 待后端 snapshot API)
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
                    }
                    PilottyButton(
                        text = "Rollback",
                        onClick = { /* TODO: ManualRollback */ },
                        variant = PilottyButtonVariant.Accent,
                        small = true,
                    )
                }
                Spacer(Modifier.height(8.dp))
                Text(
                    "自动快照在每次 Agent 写操作前生成。TODO: snapshot API 暴露后填充最近一次快照时间戳。",
                    color = pc.ink3,
                    fontSize = 11.5.sp,
                )
            }
        }

        ui.error?.let {
            Text("错误: $it", color = pc.error, fontSize = 12.sp)
        }
        Spacer(Modifier.height(96.dp)) // floating nav 留白
    }
}
