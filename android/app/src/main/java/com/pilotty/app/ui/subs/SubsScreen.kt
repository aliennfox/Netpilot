package com.pilotty.app.ui.subs

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.pilotty.app.data.SubscriptionDto
import com.pilotty.app.data.UserInfoDto
import com.pilotty.app.ui.components.PilottyButton
import com.pilotty.app.ui.components.PilottyButtonVariant
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.import_.ImportBus
import com.pilotty.app.ui.theme.LocalPilottyColors

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SubsScreen(vm: SubsViewModel = viewModel(), onNavigateQrScan: () -> Unit = {}) {
    val pc = LocalPilottyColors.current
    val ui by vm.state.collectAsStateWithLifecycle()
    val snackbar = remember { SnackbarHostState() }
    var addDialogOpen by remember { mutableStateOf(false) }
    var removeTarget by remember { mutableStateOf<SubscriptionDto?>(null) }

    // M16 Deep Link: 有 pending URI 时弹确认 dialog
    val pendingImport by ImportBus.pending.collectAsStateWithLifecycle()

    LaunchedEffect(ui.toast) {
        ui.toast?.let { snackbar.showSnackbar(it); vm.dismissToast() }
    }

    Scaffold(
        snackbarHost = {
            // 抬升 96dp, 让 snackbar 落在 floating nav 上方, 否则被导航栏遮挡
            Box(modifier = Modifier.padding(bottom = 96.dp)) {
                SnackbarHost(snackbar)
            }
        },
        containerColor = pc.bg,
    ) { pad ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(pad)
                .padding(horizontal = 20.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Spacer(Modifier.height(4.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column {
                    Text(
                        "Subscriptions",
                        color = pc.ink,
                        fontSize = 20.sp,
                        fontWeight = FontWeight.SemiBold,
                        letterSpacing = (-0.3).sp,
                    )
                    Text(
                        "${ui.subscriptions.size} sources · tap 添加 to import",
                        color = pc.ink3,
                        fontSize = 10.5.sp,
                        letterSpacing = 0.2.sp,
                        fontFamily = FontFamily.Monospace,
                    )
                }
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    PilottyButton(
                        text = "全部更新",
                        onClick = { vm.updateAll() },
                        variant = PilottyButtonVariant.Ghost,
                        small = true,
                        enabled = !ui.loading && ui.subscriptions.isNotEmpty(),
                    )
                    PilottyButton(
                        text = "添加",
                        onClick = { addDialogOpen = true },
                        variant = PilottyButtonVariant.Primary,
                        small = true,
                        enabled = !ui.loading,
                    )
                }
            }
            if (ui.loading) LinearProgressIndicator(
                modifier = Modifier.fillMaxWidth(),
                color = pc.accent,
                trackColor = pc.surface3,
            )
            ui.error?.let {
                Text("错误: $it", color = pc.error)
                PilottyButton(text = "清除", onClick = { vm.dismissError() }, variant = PilottyButtonVariant.Ghost, small = true)
            }

            if (ui.subscriptions.isEmpty() && !ui.loading) {
                PilottyCard(modifier = Modifier.fillMaxWidth()) {
                    Column(
                        Modifier.padding(16.dp),
                        verticalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        Text("还没有订阅", color = pc.ink, fontSize = 14.sp, fontWeight = FontWeight.Medium)
                        Text(
                            "点右上「添加」粘贴 URL 导入。支持 v2ray subscribe / clash / sing-box 格式。",
                            color = pc.ink3,
                            fontSize = 12.5.sp,
                        )
                    }
                }
            } else {
                LazyColumn(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    items(ui.subscriptions, key = { it.id }) { sub ->
                        SubscriptionCard(
                            sub = sub,
                            onRemove = { removeTarget = sub },
                        )
                    }
                    item { Spacer(Modifier.height(96.dp)) } // floating nav 留白
                }
            }
        }
    }

    if (addDialogOpen) {
        AddSubscriptionDialog(
            onDismiss = { addDialogOpen = false },
            onScanQr = onNavigateQrScan,
            onConfirm = { name, url ->
                vm.addSubscription(name, url)
                addDialogOpen = false
            },
        )
    }

    removeTarget?.let { target ->
        AlertDialog(
            onDismissRequest = { removeTarget = null },
            containerColor = pc.surface,
            title = { Text("删除订阅", color = pc.ink) },
            text = { Text("确定删除「${target.name}」?节点不会从已导入节点列表里自动撤销。", color = pc.ink2) },
            confirmButton = {
                TextButton(onClick = {
                    vm.removeSubscription(target.id)
                    removeTarget = null
                }) { Text("删除", color = pc.error) }
            },
            dismissButton = {
                TextButton(onClick = { removeTarget = null }) { Text("取消", color = pc.ink2) }
            },
        )
    }

    pendingImport?.let { item ->
        ImportConfirmDialog(
            uri = item.uri,
            onDismiss = { ImportBus.consume() },
            onConfirm = {
                vm.importNodeURI(item.uri)
                ImportBus.consume()
            },
        )
    }
}

@Composable
private fun ImportConfirmDialog(
    uri: String,
    onDismiss: () -> Unit,
    onConfirm: () -> Unit,
) {
    val pc = LocalPilottyColors.current
    val scheme = uri.substringBefore("://", missingDelimiterValue = "?").uppercase()
    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = pc.surface,
        title = { Text("导入节点 · $scheme", color = pc.ink) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text(
                    "检测到外部 App 分享的节点 URI, 是否导入?",
                    color = pc.ink2,
                    fontSize = 13.sp,
                )
                Text(
                    uri,
                    color = pc.ink3,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                    maxLines = 3,
                )
            }
        },
        confirmButton = {
            TextButton(onClick = onConfirm) { Text("导入", color = pc.accentInk, fontWeight = FontWeight.SemiBold) }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("忽略", color = pc.ink3) }
        },
    )
}

@Composable
private fun SubscriptionCard(sub: SubscriptionDto, onRemove: () -> Unit) {
    val pc = LocalPilottyColors.current
    PilottyCard(modifier = Modifier.fillMaxWidth()) {
        Column(
            Modifier.padding(14.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    sub.name.ifEmpty { "未命名" },
                    color = pc.ink,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                )
                TextButton(onClick = onRemove) {
                    Text("删除", color = pc.error, fontSize = 12.sp)
                }
            }
            Text(
                sub.url,
                color = pc.ink3,
                fontSize = 11.5.sp,
                fontFamily = FontFamily.Monospace,
                maxLines = 1,
            )
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Text(
                    "${sub.nodeCount} 个节点",
                    color = pc.ink2,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Medium,
                )
                if (sub.autoUpdate) {
                    Text(
                        "·  自动更新 ${sub.intervalMinutes} min",
                        color = pc.ink4,
                        fontSize = 11.sp,
                    )
                }
            }
            sub.userInfo?.let { QuotaRow(it) }
        }
    }
}

@Composable
private fun QuotaRow(info: UserInfoDto) {
    val pc = LocalPilottyColors.current
    val used = info.upload + info.download
    val total = info.total.coerceAtLeast(1)
    val pct = (used.toFloat() / total.toFloat()).coerceIn(0f, 1f)
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(4.dp)
                .clip(RoundedCornerShape(2.dp)),
        ) {
            Canvas(modifier = Modifier.fillMaxSize()) {
                drawRect(color = pc.hairline)
                drawRect(color = pc.accent, size = Size(size.width * pct, size.height))
            }
        }
        Text(
            "已用 ${formatBytes(used)} / ${formatBytes(info.total)}" +
                (if (info.expire > 0) "  ·  到期 ${formatTimestamp(info.expire)}" else ""),
            color = pc.ink3,
            fontSize = 10.5.sp,
            fontFamily = FontFamily.Monospace,
            letterSpacing = 0.2.sp,
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

/** 剪贴板若含协议 URI 或 http(s) 订阅 URL, 返回清洗后的文本。 */
private fun pickImportableFromClipboard(ctx: android.content.Context): String? {
    val cm = ctx.getSystemService(android.content.Context.CLIPBOARD_SERVICE)
        as? android.content.ClipboardManager ?: return null
    val clip = cm.primaryClip ?: return null
    if (clip.itemCount == 0) return null
    val text = clip.getItemAt(0).coerceToText(ctx)?.toString()?.trim() ?: return null
    if (text.isEmpty()) return null
    val schemes = listOf(
        "ss://", "vmess://", "vless://", "trojan://", "tuic://",
        "hysteria://", "hysteria2://", "hy2://", "wireguard://", "wg://",
        "anytls://", "shadowtls://", "http://", "https://",
    )
    return if (schemes.any { text.startsWith(it, ignoreCase = true) }) text else null
}

@Composable
private fun AddSubscriptionDialog(
    onDismiss: () -> Unit,
    onConfirm: (name: String, url: String) -> Unit,
    onScanQr: () -> Unit = {},
) {
    val pc = LocalPilottyColors.current
    val ctx = LocalContext.current
    var name by remember { mutableStateOf("") }
    var url by remember { mutableStateOf("") }
    val clipboardHit = remember { pickImportableFromClipboard(ctx) }
    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = pc.surface,
        title = { Text("添加订阅", color = pc.ink) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                // Phase 10-E-C: 扫码入口, 优先于剪贴板 (QR 是分享节点主流路径)
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    PilottyButton(
                        text = "扫码导入",
                        onClick = {
                            onDismiss()
                            onScanQr()
                        },
                        variant = PilottyButtonVariant.Outline,
                        small = true,
                    )
                    if (clipboardHit != null && url.isBlank()) {
                        PilottyButton(
                            text = "使用剪贴板",
                            onClick = { url = clipboardHit },
                            variant = PilottyButtonVariant.Outline,
                            small = true,
                        )
                    }
                }
                OutlinedTextField(
                    value = name,
                    onValueChange = { name = it },
                    label = { Text("名称 (可选)") },
                    singleLine = true,
                    colors = OutlinedTextFieldDefaults.colors(
                        focusedBorderColor = pc.ink,
                        unfocusedBorderColor = pc.hairlineStrong,
                    ),
                )
                OutlinedTextField(
                    value = url,
                    onValueChange = { url = it },
                    label = { Text("订阅 URL / 节点 URI") },
                    singleLine = true,
                    placeholder = { Text("https:// 或 vmess:// / ss:// / ...") },
                    colors = OutlinedTextFieldDefaults.colors(
                        focusedBorderColor = pc.ink,
                        unfocusedBorderColor = pc.hairlineStrong,
                    ),
                )
                Text(
                    "提示: 粘贴 http(s) 订阅 URL 将拉取节点列表; 粘贴 vmess:// 等单节点 URI 将直接导入单个节点。",
                    color = pc.ink4,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                )
            }
        },
        confirmButton = {
            TextButton(
                onClick = { onConfirm(name, url) },
                enabled = url.isNotBlank(),
            ) { Text("导入", color = if (url.isNotBlank()) pc.accentInk else pc.ink4) }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("取消", color = pc.ink2) }
        },
    )
}
