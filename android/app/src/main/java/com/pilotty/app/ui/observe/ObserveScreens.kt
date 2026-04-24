package com.pilotty.app.ui.observe

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.clickable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.compose.viewModel
import com.pilotty.app.data.ConnectionDto
import com.pilotty.app.data.LogEntryDto
import com.pilotty.app.data.PilottyRepository
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.theme.LocalPilottyColors
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/* ============================= Shared utils ============================= */

private val hhmmss = SimpleDateFormat("HH:mm:ss", Locale.US)

private fun formatBytes(n: Long): String = when {
    n < 1024 -> "${n} B"
    n < 1024 * 1024 -> String.format(Locale.US, "%.1f KB", n / 1024.0)
    n < 1024L * 1024L * 1024L -> String.format(Locale.US, "%.2f MB", n / (1024.0 * 1024.0))
    else -> String.format(Locale.US, "%.2f GB", n / (1024.0 * 1024.0 * 1024.0))
}

private fun formatDuration(ms: Long): String {
    val s = ms / 1000
    return when {
        s < 60 -> "${s}s"
        s < 3600 -> "${s / 60}m${s % 60}s"
        else -> "${s / 3600}h${(s % 3600) / 60}m"
    }
}

/* ============================= Logs ============================= */

/**
 * M7 · 导出日志到系统 Share chooser。
 * 拼纯文本 (timestamp + level + msg), 经 ACTION_SEND 让用户自选去向
 * (Save to Files / Gmail / Telegram / Drive 等)。 Android 10+ scoped storage
 * 下不用任何权限, 也不碰 MediaStore。
 */
private fun shareLogs(
    ctx: android.content.Context,
    logs: List<LogEntryDto>,
    chooserTitle: String,
) {
    val body = buildString {
        append("# Pilotty logs · exported ").append(java.util.Date()).append('\n')
        append("# ").append(logs.size).append(" entries\n\n")
        logs.forEach { e ->
            val ts = if (e.ts > 0) hhmmss.format(Date(e.ts)) else "--:--:--"
            append(ts).append(' ')
                .append(e.level.ifEmpty { "?" }).append(' ')
                .append(e.msg).append('\n')
        }
    }
    val send = android.content.Intent(android.content.Intent.ACTION_SEND).apply {
        type = "text/plain"
        putExtra(android.content.Intent.EXTRA_SUBJECT, "Pilotty logs")
        putExtra(android.content.Intent.EXTRA_TEXT, body)
    }
    val chooser = android.content.Intent.createChooser(send, chooserTitle).apply {
        addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK)
    }
    ctx.startActivity(chooser)
}


class LogsViewModel : ViewModel() {
    private val _logs = MutableStateFlow<List<LogEntryDto>>(emptyList())
    val logs: StateFlow<List<LogEntryDto>> = _logs.asStateFlow()
    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error.asStateFlow()

    init { startPolling() }

    private fun startPolling() = viewModelScope.launch {
        while (true) {
            try {
                _logs.value = PilottyRepository.recentLogs(0)
                _error.value = null
            } catch (e: Throwable) {
                _error.value = e.message
            }
            delay(2000L)
        }
    }
}

@Composable
fun LogsScreen(onBack: () -> Unit, vm: LogsViewModel = viewModel()) {
    val pc = LocalPilottyColors.current
    val ctx = LocalContext.current
    val logs by vm.logs.collectAsStateWithLifecycle()
    val error by vm.error.collectAsStateWithLifecycle()
    val listState = rememberLazyListState()
    val chooserTitle = stringResource(com.pilotty.app.R.string.logs_export_chooser_title)
    val emptyToast = stringResource(com.pilotty.app.R.string.logs_export_empty_toast)

    // 当日志新增时自动滚到底 (用户不在手动回看历史时)
    LaunchedEffect(logs.size) {
        if (logs.isNotEmpty()) {
            val lastVisible = listState.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0
            val total = listState.layoutInfo.totalItemsCount
            // 只在用户已经滚到底部 ±3 条范围时自动贴底, 否则别抢用户的视角
            if (lastVisible >= total - 4) {
                listState.animateScrollToItem(logs.size - 1)
            }
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
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                "←",
                color = pc.ink,
                fontSize = 20.sp,
                fontWeight = FontWeight.Bold,
                modifier = Modifier
                    .clickable { onBack() }
                    .padding(end = 12.dp),
            )
            Column(Modifier.weight(1f)) {
                Text(stringResource(com.pilotty.app.R.string.logs_title), color = pc.ink, fontSize = 20.sp, fontWeight = FontWeight.SemiBold)
                Text(
                    stringResource(com.pilotty.app.R.string.logs_subtitle_format, logs.size),
                    color = pc.ink3,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                )
            }
            // M7: 导出按钮 — 把所有日志 share 出去 (用户自选 Save to Files / Gmail / Telegram / 云盘 等),
            // 不需要 scoped storage 权限, 兼容 Android 10+ 最干净路径
            Text(
                stringResource(com.pilotty.app.R.string.logs_export),
                color = pc.accentInk,
                fontSize = 13.sp,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier
                    .clickable {
                        if (logs.isEmpty()) {
                            android.widget.Toast.makeText(ctx, emptyToast, android.widget.Toast.LENGTH_SHORT).show()
                        } else {
                            shareLogs(ctx, logs, chooserTitle)
                        }
                    }
                    .padding(horizontal = 8.dp, vertical = 4.dp),
            )
        }
        error?.let {
            Text(
                stringResource(com.pilotty.app.R.string.logs_load_error_format, it),
                color = pc.error,
                fontSize = 12.sp,
                modifier = Modifier.padding(horizontal = 20.dp),
            )
        }
        if (logs.isEmpty() && error == null) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text(stringResource(com.pilotty.app.R.string.logs_empty), color = pc.ink4, fontSize = 12.sp)
            }
        } else {
            LazyColumn(
                state = listState,
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = 12.dp),
                contentPadding = PaddingValues(bottom = 96.dp), // floating nav 留白
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                items(logs) { entry ->
                    LogRow(entry)
                }
            }
        }
    }
}

@Composable
private fun LogRow(e: LogEntryDto) {
    val pc = LocalPilottyColors.current
    val levelColor = when (e.level.uppercase()) {
        "E" -> pc.error
        "W" -> pc.accentInk
        "D" -> pc.ink4
        else -> pc.ink2 // "I"
    }
    val time = if (e.ts > 0) hhmmss.format(Date(e.ts)) else "--:--:--"
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 8.dp, vertical = 2.dp),
    ) {
        Text(
            time,
            color = pc.ink4,
            fontSize = 10.5.sp,
            fontFamily = FontFamily.Monospace,
            modifier = Modifier.width(64.dp),
        )
        Text(
            e.level.ifEmpty { "?" },
            color = levelColor,
            fontSize = 10.5.sp,
            fontFamily = FontFamily.Monospace,
            fontWeight = FontWeight.Bold,
            modifier = Modifier.width(20.dp),
        )
        Text(
            e.msg,
            color = pc.ink,
            fontSize = 11.5.sp,
            fontFamily = FontFamily.Monospace,
            modifier = Modifier.weight(1f),
        )
    }
}

/* ============================= Connections ============================= */

class ConnectionsViewModel : ViewModel() {
    private val _conns = MutableStateFlow<List<ConnectionDto>>(emptyList())
    val conns: StateFlow<List<ConnectionDto>> = _conns.asStateFlow()
    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error.asStateFlow()
    /** A3 刚被 kill 的 id, UI 短暂显示 "已掐断" 态 (1.5s) 然后随下一轮 poll 自然消失。 */
    private val _killingIds = MutableStateFlow<Set<String>>(emptySet())
    val killingIds: StateFlow<Set<String>> = _killingIds.asStateFlow()

    init { startPolling() }

    private fun startPolling() = viewModelScope.launch {
        while (true) {
            try {
                // 按下载流量倒排, 大流量连接更优先被看到
                _conns.value = PilottyRepository.connections().sortedByDescending { it.download }
                _error.value = null
            } catch (e: Throwable) {
                _error.value = e.message
            }
            delay(2000L)
        }
    }

    fun killConnection(id: String) = viewModelScope.launch {
        _killingIds.value = _killingIds.value + id
        try {
            PilottyRepository.closeConnection(id)
            // 掐完立即从本地列表移除, 不等下一轮 poll, UI 看起来更即时
            _conns.value = _conns.value.filterNot { it.id == id }
        } catch (e: Throwable) {
            _error.value = "掐断失败: ${e.message}"
        } finally {
            _killingIds.value = _killingIds.value - id
        }
    }
}

@Composable
fun ConnectionsScreen(onBack: () -> Unit, vm: ConnectionsViewModel = viewModel()) {
    val pc = LocalPilottyColors.current
    val conns by vm.conns.collectAsStateWithLifecycle()
    val error by vm.error.collectAsStateWithLifecycle()
    val killing by vm.killingIds.collectAsStateWithLifecycle()

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(pc.bg),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                "←",
                color = pc.ink,
                fontSize = 20.sp,
                fontWeight = FontWeight.Bold,
                modifier = Modifier
                    .clickable { onBack() }
                    .padding(end = 12.dp),
            )
            Column(Modifier.weight(1f)) {
                Text("活跃连接", color = pc.ink, fontSize = 20.sp, fontWeight = FontWeight.SemiBold)
                Text(
                    "${conns.size} 条 · 按下载流量倒排 · 每 2 秒刷新",
                    color = pc.ink3,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                )
            }
        }
        error?.let {
            Text(
                "加载错误: $it",
                color = pc.error,
                fontSize = 12.sp,
                modifier = Modifier.padding(horizontal = 20.dp),
            )
        }
        if (conns.isEmpty() && error == null) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text(
                    "无活跃连接 — VPN 未启动或当前无流量",
                    color = pc.ink4,
                    fontSize = 12.sp,
                )
            }
        } else {
            LazyColumn(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = 12.dp),
                contentPadding = PaddingValues(vertical = 4.dp, horizontal = 4.dp).let {
                    PaddingValues(bottom = 96.dp)
                },
                verticalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                items(conns, key = { it.id }) { c ->
                    ConnectionRow(
                        c = c,
                        isKilling = c.id in killing,
                        onKill = { vm.killConnection(c.id) },
                    )
                }
            }
        }
    }
}

@Composable
private fun ConnectionRow(
    c: ConnectionDto,
    isKilling: Boolean = false,
    onKill: () -> Unit = {},
) {
    val pc = LocalPilottyColors.current
    var confirm by remember { mutableStateOf(false) }
    if (confirm) {
        AlertDialog(
            onDismissRequest = { confirm = false },
            title = { Text("掐断此连接?") },
            text = {
                Text(
                    "${c.destination}\n目标进程: ${c.processName.ifEmpty { "?" }}",
                    fontSize = 13.sp,
                    fontFamily = FontFamily.Monospace,
                    lineHeight = 18.sp,
                )
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        confirm = false
                        onKill()
                    },
                ) { Text("掐断", color = pc.error) }
            },
            dismissButton = { TextButton(onClick = { confirm = false }) { Text("取消") } },
        )
    }
    PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
        Column(Modifier.padding(horizontal = 12.dp, vertical = 10.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    c.destination,
                    color = pc.ink,
                    fontSize = 13.sp,
                    fontWeight = FontWeight.Medium,
                    fontFamily = FontFamily.Monospace,
                    modifier = Modifier.weight(1f),
                )
                Text(
                    formatDuration(c.durationMs),
                    color = pc.ink3,
                    fontSize = 10.sp,
                    fontFamily = FontFamily.Monospace,
                )
                Spacer(Modifier.width(8.dp))
                // Kill 按钮: 小 × 图标, 带确认 dialog 防误操作
                Surface(
                    color = if (isKilling) pc.surface3 else pc.surface2,
                    shape = RoundedCornerShape(6.dp),
                    onClick = { if (!isKilling) confirm = true },
                ) {
                    Text(
                        if (isKilling) "···" else "×",
                        color = if (isKilling) pc.ink3 else pc.error,
                        fontSize = 14.sp,
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp),
                    )
                }
            }
            Spacer(Modifier.height(3.dp))
            val proto = c.protocol.takeIf { it.isNotEmpty() } ?: "-"
            val proc = c.processName.takeIf { it.isNotEmpty() } ?: "?"
            Text(
                "$proto · $proc",
                color = pc.ink3,
                fontSize = 10.5.sp,
                fontFamily = FontFamily.Monospace,
            )
            Spacer(Modifier.height(3.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "↑ ${formatBytes(c.upload)}",
                    color = pc.ink2,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                )
                Spacer(Modifier.width(10.dp))
                Text(
                    "↓ ${formatBytes(c.download)}",
                    color = pc.accentInk,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                    fontWeight = FontWeight.SemiBold,
                )
                Spacer(Modifier.width(10.dp))
                if (c.chain.isNotEmpty()) {
                    Text(
                        c.chain.joinToString(" → "),
                        color = pc.ink4,
                        fontSize = 10.sp,
                        fontFamily = FontFamily.Monospace,
                    )
                }
            }
            if (c.rule.isNotEmpty()) {
                Text(
                    "rule: ${c.rule}",
                    color = pc.ink4,
                    fontSize = 10.sp,
                    fontFamily = FontFamily.Monospace,
                    modifier = Modifier.padding(top = 2.dp),
                )
            }
        }
    }
}
