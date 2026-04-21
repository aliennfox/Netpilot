package com.pilotty.app.ui.settings

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.pilotty.app.data.SubscriptionDto
import com.pilotty.app.data.UserInfoDto

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(vm: SettingsViewModel = viewModel()) {
    val ui by vm.state.collectAsStateWithLifecycle()
    val snackbar = remember { SnackbarHostState() }
    var addDialogOpen by remember { mutableStateOf(false) }
    var removeTarget by remember { mutableStateOf<SubscriptionDto?>(null) }

    LaunchedEffect(ui.toast) {
        ui.toast?.let { snackbar.showSnackbar(it); vm.dismissToast() }
    }

    Scaffold(snackbarHost = { SnackbarHost(snackbar) }) { pad ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(pad)
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("订阅管理", style = MaterialTheme.typography.titleMedium)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    TextButton(
                        onClick = { vm.updateAll() },
                        enabled = !ui.loading && ui.subscriptions.isNotEmpty(),
                    ) { Text("全部更新") }
                    Button(
                        onClick = { addDialogOpen = true },
                        enabled = !ui.loading,
                    ) { Text("添加") }
                }
            }
            if (ui.loading) LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
            ui.error?.let {
                Text("错误: $it", color = MaterialTheme.colorScheme.error)
                TextButton(onClick = { vm.dismissError() }) { Text("清除") }
            }

            if (ui.subscriptions.isEmpty() && !ui.loading) {
                Text(
                    "还没有订阅。点右上「添加」粘贴 URL 导入。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            } else {
                LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    items(ui.subscriptions, key = { it.id }) { sub ->
                        SubscriptionCard(
                            sub = sub,
                            onRemove = { removeTarget = sub },
                        )
                    }
                }
            }

            Spacer(Modifier.height(8.dp))
            HorizontalDivider()
            Text(
                "更多:故障自动切换 / 备份导入导出 / API Token (TODO)",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }

    if (addDialogOpen) {
        AddSubscriptionDialog(
            onDismiss = { addDialogOpen = false },
            onConfirm = { name, url ->
                vm.addSubscription(name, url)
                addDialogOpen = false
            },
        )
    }

    removeTarget?.let { target ->
        AlertDialog(
            onDismissRequest = { removeTarget = null },
            title = { Text("删除订阅") },
            text = { Text("确定删除「${target.name}」?节点不会从已导入节点列表里自动撤销。") },
            confirmButton = {
                TextButton(onClick = {
                    vm.removeSubscription(target.id)
                    removeTarget = null
                }) { Text("删除", color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = {
                TextButton(onClick = { removeTarget = null }) { Text("取消") }
            },
        )
    }
}

@Composable
private fun SubscriptionCard(sub: SubscriptionDto, onRemove: () -> Unit) {
    ElevatedCard(modifier = Modifier.fillMaxWidth()) {
        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(sub.name.ifEmpty { "未命名" }, fontWeight = FontWeight.Bold)
                TextButton(onClick = onRemove) { Text("删除", color = MaterialTheme.colorScheme.error) }
            }
            Text(
                sub.url,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1,
            )
            Text(
                "${sub.nodeCount} 个节点" +
                    (if (sub.autoUpdate) " · 每 ${sub.intervalMinutes} 分钟自动更新" else ""),
                style = MaterialTheme.typography.bodySmall,
            )
            sub.userInfo?.let { QuotaRow(it) }
        }
    }
}

@Composable
private fun QuotaRow(info: UserInfoDto) {
    val used = info.upload + info.download
    val total = info.total.coerceAtLeast(1)
    val pct = (used.toFloat() / total.toFloat()).coerceIn(0f, 1f)
    Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
        LinearProgressIndicator(
            progress = { pct },
            modifier = Modifier.fillMaxWidth(),
        )
        Text(
            "已用 ${formatBytes(used)} / ${formatBytes(info.total)}" +
                (if (info.expire > 0) " · 到期 ${formatTimestamp(info.expire)}" else ""),
            style = MaterialTheme.typography.labelSmall,
        )
    }
}

private fun formatBytes(b: Long): String {
    if (b <= 0) return "0 B"
    val units = arrayOf("B", "KB", "MB", "GB", "TB")
    var v = b.toDouble()
    var i = 0
    while (v >= 1024 && i < units.size - 1) { v /= 1024; i++ }
    return String.format("%.2f %s", v, units[i])
}

private fun formatTimestamp(epochSec: Long): String {
    if (epochSec <= 0) return "—"
    val ms = epochSec * 1000
    val f = java.text.SimpleDateFormat("yyyy-MM-dd", java.util.Locale.getDefault())
    return f.format(java.util.Date(ms))
}

@Composable
private fun AddSubscriptionDialog(
    onDismiss: () -> Unit,
    onConfirm: (name: String, url: String) -> Unit,
) {
    var name by remember { mutableStateOf("") }
    var url by remember { mutableStateOf("") }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("添加订阅") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    value = name,
                    onValueChange = { name = it },
                    label = { Text("名称 (可选)") },
                    singleLine = true,
                )
                OutlinedTextField(
                    value = url,
                    onValueChange = { url = it },
                    label = { Text("订阅 URL") },
                    singleLine = true,
                    placeholder = { Text("https://...") },
                )
            }
        },
        confirmButton = {
            TextButton(
                onClick = { onConfirm(name, url) },
                enabled = url.isNotBlank(),
            ) { Text("导入") }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("取消") }
        },
    )
}
