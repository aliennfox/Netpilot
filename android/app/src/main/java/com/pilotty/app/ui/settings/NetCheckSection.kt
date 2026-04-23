package com.pilotty.app.ui.settings

import android.util.Log
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.data.NetCheckLineDto
import com.pilotty.app.data.NetCheckReportDto
import com.pilotty.app.data.PilottyRepository
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.PilottyButton
import com.pilotty.app.ui.components.PilottyButtonVariant
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.theme.LocalPilottyColors
import kotlinx.coroutines.launch

/**
 * Phase P1-B: 网络自检。 用户 "连不上" 时一键跑 5 段探测 (VPN / 节点 / 规则 / DNS / HTTP),
 * 每行 level 染色, detail 点击展开。
 *
 * 不含 socket/route table 原始信息 —— 那是开发者工具, 消费级用户看不懂也不需要。
 * 重点在 "能不能走通" + "哪段卡住"。
 */
@Composable
fun NetCheckSection() {
    val pc = LocalPilottyColors.current
    val scope = rememberCoroutineScope()
    var report by remember { mutableStateOf<NetCheckReportDto?>(null) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var expandedIdx by remember { mutableStateOf<Int?>(null) }

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Kicker("网络自检")
        PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
            Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text(
                            "一键诊断",
                            color = pc.ink,
                            fontSize = 14.sp,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Spacer(Modifier.height(2.dp))
                        Text(
                            "跑 5 段探测: VPN 状态 · 当前节点 · 规则/节点 · DNS 解析 · HTTP 连通。 约 3 秒。",
                            color = pc.ink3,
                            fontSize = 11.sp,
                        )
                    }
                    PilottyButton(
                        text = if (busy) "检测中…" else "开始",
                        onClick = {
                            if (busy) return@PilottyButton
                            busy = true
                            error = null
                            scope.launch {
                                try {
                                    report = PilottyRepository.netCheck()
                                } catch (t: Throwable) {
                                    Log.w(TAG, "netCheck failed", t)
                                    error = t.message
                                } finally {
                                    busy = false
                                }
                            }
                        },
                        variant = PilottyButtonVariant.Primary,
                        small = true,
                        enabled = !busy,
                    )
                }

                error?.let {
                    Text("错误: $it", color = pc.error, fontSize = 11.sp)
                }

                report?.let { r ->
                    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        r.lines.forEachIndexed { idx, line ->
                            NetCheckRow(
                                line = line,
                                expanded = expandedIdx == idx,
                                onToggle = { expandedIdx = if (expandedIdx == idx) null else idx },
                            )
                        }
                        Spacer(Modifier.height(2.dp))
                        Text(
                            "用时 ${r.elapsedMs} ms",
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

@Composable
private fun NetCheckRow(
    line: NetCheckLineDto,
    expanded: Boolean,
    onToggle: () -> Unit,
) {
    val pc = LocalPilottyColors.current
    val dotColor = when (line.level) {
        "ok" -> pc.accent
        "warn" -> Color(0xFFE8A33D)
        "fail" -> pc.error
        else -> pc.ink4
    }
    val clickable = line.detail.isNotEmpty()

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .then(if (clickable) Modifier.clickable(onClick = onToggle) else Modifier),
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Box(
                modifier = Modifier
                    .size(8.dp)
                    .clip(CircleShape)
                    .background(dotColor),
            )
            Text(
                line.section,
                color = pc.ink4,
                fontSize = 10.sp,
                fontFamily = FontFamily.Monospace,
                modifier = Modifier.width(38.dp),
            )
            Text(
                line.msg,
                color = pc.ink,
                fontSize = 12.sp,
                modifier = Modifier.weight(1f),
            )
            if (clickable) {
                Text(
                    if (expanded) "−" else "＋",
                    color = pc.ink4,
                    fontSize = 13.sp,
                    fontWeight = FontWeight.Bold,
                )
            }
        }
        if (expanded && line.detail.isNotEmpty()) {
            Spacer(Modifier.height(2.dp))
            Text(
                line.detail,
                color = pc.ink3,
                fontSize = 10.sp,
                fontFamily = FontFamily.Monospace,
                modifier = Modifier.padding(start = 16.dp, end = 4.dp),
            )
        }
    }
}

private const val TAG = "NetCheckSection"
