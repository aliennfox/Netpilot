package com.pilotty.app.ui.settings

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.R
import com.pilotty.app.data.PilottyRepository
import com.pilotty.app.data.TelemetryEntryDto
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.components.SectionHead
import com.pilotty.app.ui.friendlyToolName
import com.pilotty.app.ui.theme.LocalPilottyColors

// D3 Settings · 最近 Agent 活动. 数据源: telemetry.jsonl tail (跨重启可见).
// 卡片显示前 5 条预览, 点 "查看全部" 弹 Dialog 显示最近 50 条.
@Composable
fun AgentActivitySection() {
    val pc = LocalPilottyColors.current
    var entries by remember { mutableStateOf<List<TelemetryEntryDto>>(emptyList()) }
    var loaded by remember { mutableStateOf(false) }
    var dialogOpen by remember { mutableStateOf(false) }
    var refreshTick by remember { mutableStateOf(0) }

    LaunchedEffect(refreshTick) {
        runCatching { PilottyRepository.recentTelemetry(50) }
            .onSuccess { entries = it.asReversed() } // 最新在前
            .onFailure { entries = emptyList() }
        loaded = true
    }

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        SectionHead(stringResource(R.string.settings_agent_activity))
        PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
            Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(
                    stringResource(R.string.settings_agent_activity_subtitle),
                    color = pc.ink3,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                )
                if (loaded && entries.isEmpty()) {
                    Text(
                        stringResource(R.string.settings_agent_activity_empty),
                        color = pc.ink3,
                        fontSize = 12.sp,
                    )
                } else {
                    val preview = entries.take(5)
                    preview.forEach { e ->
                        TelemetryRow(e)
                    }
                    if (entries.size > 5) {
                        Surface(
                            color = pc.surface2,
                            shape = RoundedCornerShape(8.dp),
                            onClick = {
                                refreshTick++
                                dialogOpen = true
                            },
                        ) {
                            Text(
                                stringResource(R.string.settings_agent_activity_view_all)
                                    + " · "
                                    + stringResource(
                                        R.string.settings_agent_activity_more_format,
                                        entries.size - 5,
                                    ),
                                modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
                                color = pc.ink,
                                fontSize = 11.5.sp,
                                fontWeight = FontWeight.Medium,
                            )
                        }
                    }
                }
            }
        }
    }

    if (dialogOpen) {
        AlertDialog(
            onDismissRequest = { dialogOpen = false },
            confirmButton = {
                TextButton(onClick = { dialogOpen = false }) {
                    Text(stringResource(R.string.settings_agent_activity_close))
                }
            },
            title = {
                Text(
                    stringResource(R.string.settings_agent_activity_dialog_title),
                    fontSize = 15.sp,
                    fontWeight = FontWeight.SemiBold,
                )
            },
            text = {
                LazyColumn(
                    modifier = Modifier
                        .fillMaxWidth()
                        .heightIn(max = 480.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    items(entries) { e -> TelemetryRow(e, expanded = true) }
                }
            },
        )
    }
}

@Composable
private fun TelemetryRow(e: TelemetryEntryDto, expanded: Boolean = false) {
    val pc = LocalPilottyColors.current
    val ok = e.success
    val rolled = e.rolledBack
    val badge = when {
        rolled -> "↶"
        ok -> "✓"
        else -> "✗"
    }
    val badgeColor = when {
        rolled -> pc.ink3
        ok -> pc.accentInk
        else -> pc.error
    }
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.Top,
    ) {
        Text(
            badge,
            color = badgeColor,
            fontSize = 12.sp,
            fontFamily = FontFamily.Monospace,
        )
        Column(modifier = Modifier.weight(1f)) {
            Row(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(
                    friendlyToolName(e.tool),
                    color = pc.ink,
                    fontSize = 12.5.sp,
                    fontWeight = FontWeight.Medium,
                )
                if (rolled) {
                    Text(
                        "· " + stringResource(R.string.settings_agent_activity_rolledback),
                        color = pc.ink3,
                        fontSize = 10.5.sp,
                    )
                }
            }
            Text(
                buildString {
                    append(formatTelemetryTime(e.timestamp))
                    append(" · ")
                    append(formatDurationMsCompact(e.durationMs))
                    if (e.snapshotId.isNotEmpty()) {
                        append(" · ")
                        append(e.snapshotId)
                    }
                },
                color = pc.ink3,
                fontSize = 10.5.sp,
                fontFamily = FontFamily.Monospace,
            )
            if (expanded && !ok && e.error.isNotEmpty()) {
                Text(
                    e.error.take(180),
                    color = pc.error,
                    fontSize = 10.5.sp,
                    fontFamily = FontFamily.Monospace,
                )
            }
        }
    }
}

// Go telemetry timestamp 是 RFC3339 (e.g. "2026-04-25T04:14:34.540428+08:00").
// 只取 HH:mm:ss 显示, 失败回退原串前 19 字符.
private fun formatTelemetryTime(ts: String): String {
    if (ts.length < 19) return ts
    val tIdx = ts.indexOf('T')
    if (tIdx < 0 || tIdx + 9 > ts.length) return ts.take(19)
    return ts.substring(tIdx + 1, tIdx + 9) // HH:mm:ss
}

private fun formatDurationMsCompact(ms: Long): String = when {
    ms <= 0 -> "0ms"
    ms < 1000 -> "${ms}ms"
    ms < 10_000 -> "%.1fs".format(ms / 1000.0)
    else -> "${ms / 1000}s"
}
