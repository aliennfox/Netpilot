package com.pilotty.app.ui.subs

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
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
            Spacer(Modifier.height(8.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.Top,
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        "订阅",
                        color = pc.ink,
                        fontSize = 22.sp,
                        fontWeight = FontWeight.Bold,
                        letterSpacing = (-0.44).sp,
                    )
                    val totalNodes = ui.subscriptions.sumOf { it.nodeCount }
                    Text(
                        "${ui.subscriptions.size} sources · $totalNodes nodes",
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
                        text = "+",
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
                        Modifier.padding(20.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        com.pilotty.app.ui.components.Kicker("Empty · NO SUBS")
                        Text("还没有任何订阅", color = pc.ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                        Text(
                            "点右上「+」添加 —— 支持 Clash YAML · sing-box JSON · v2ray base64 三种格式。也可粘贴 vmess:// / ss:// / vless:// 等单节点 URI, 或从本地文件导入。",
                            color = pc.ink3,
                            fontSize = 12.5.sp,
                            lineHeight = 19.sp,
                        )
                    }
                }
            } else {
                LazyColumn(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    items(ui.subscriptions, key = { it.id }) { sub ->
                        SubscriptionCard(
                            sub = sub,
                            onRemove = { removeTarget = sub },
                            onUpdate = { vm.updateAll() },
                        )
                    }
                    item { Spacer(Modifier.height(24.dp)) }
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
            onFileImport = { name, bytes ->
                vm.importFromData(name, bytes)
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

/**
 * Mission Control SubCard —— 设计稿 07 屏结构:
 *   顶行: 名称 (15sp SemiBold) + mono URL ellipsized + chevron
 *   中部: `NODES n | 分隔条 | UPDATED xxx` 水平 meta
 *   下部: TRAFFIC kicker + used/total mono + Progress
 *   底部: [更新] mono button + [删除] destructive text
 */
@Composable
private fun SubscriptionCard(sub: SubscriptionDto, onRemove: () -> Unit, onUpdate: () -> Unit = {}) {
    val pc = LocalPilottyColors.current
    PilottyCard(modifier = Modifier.fillMaxWidth()) {
        Column(Modifier.padding(20.dp)) {
            // 顶行 name + URL + chevron
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        sub.name.ifEmpty { "未命名" },
                        color = pc.ink,
                        fontSize = 15.sp,
                        fontWeight = FontWeight.SemiBold,
                        letterSpacing = (-0.075).sp,
                    )
                    Spacer(Modifier.height(4.dp))
                    Text(
                        sub.url,
                        color = pc.ink3,
                        fontSize = 11.sp,
                        fontFamily = FontFamily.Monospace,
                        maxLines = 1,
                    )
                }
                Text(
                    "›",
                    color = pc.ink3,
                    fontSize = 18.sp,
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.padding(top = 2.dp),
                )
            }
            Spacer(Modifier.height(12.dp))
            // 水平 meta: NODES | UPDATED
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                Column {
                    com.pilotty.app.ui.components.Kicker("Nodes")
                    Spacer(Modifier.height(3.dp))
                    Text(
                        sub.nodeCount.toString(),
                        color = pc.ink,
                        fontSize = 13.5.sp,
                        fontWeight = FontWeight.SemiBold,
                        fontFamily = FontFamily.Monospace,
                    )
                }
                Box(
                    modifier = Modifier
                        .width(1.dp)
                        .height(26.dp)
                        .background(pc.hairline),
                )
                Column {
                    com.pilotty.app.ui.components.Kicker("Updated")
                    Spacer(Modifier.height(3.dp))
                    Text(
                        if (sub.autoUpdate) "自动 · ${sub.intervalMinutes} min" else "手动",
                        color = pc.ink2,
                        fontSize = 13.sp,
                        fontFamily = FontFamily.Monospace,
                    )
                }
            }
            // Quota block
            sub.userInfo?.let { info ->
                Spacer(Modifier.height(14.dp))
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.Bottom,
                ) {
                    com.pilotty.app.ui.components.Kicker("Traffic")
                    val used = info.upload + info.download
                    Text(
                        "${formatBytes(used)} / ${formatBytes(info.total)}" +
                            (if (info.expire > 0) " · ${formatTimestamp(info.expire)}" else ""),
                        color = pc.ink2,
                        fontSize = 11.5.sp,
                        fontFamily = FontFamily.Monospace,
                        fontWeight = FontWeight.Medium,
                    )
                }
                Spacer(Modifier.height(6.dp))
                val used = info.upload + info.download
                val pct = if (info.total > 0) (used.toFloat() / info.total.toFloat() * 100f).coerceIn(0f, 100f) else 0f
                com.pilotty.app.ui.components.Progress(pct = pct, height = 6)
            }
            // 底部 actions
            Spacer(Modifier.height(14.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                PilottyButton(
                    text = "更新",
                    onClick = onUpdate,
                    variant = PilottyButtonVariant.Mono,
                    modifier = Modifier.weight(1f),
                )
                TextButton(onClick = onRemove) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(4.dp),
                    ) {
                        Text("删除", color = pc.error, fontSize = 12.5.sp, fontWeight = FontWeight.Medium)
                    }
                }
            }
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

/** SAF URI → 展示文件名 (通常不含路径)。 OpenableColumns.DISPLAY_NAME 在 content:// 主流 provider 都填, 取不到返回空字符串。 */
private fun queryDisplayName(ctx: android.content.Context, uri: android.net.Uri): String {
    return try {
        ctx.contentResolver.query(
            uri,
            arrayOf(android.provider.OpenableColumns.DISPLAY_NAME),
            null,
            null,
            null,
        )?.use { c ->
            if (c.moveToFirst()) c.getString(0).orEmpty() else ""
        } ?: ""
    } catch (_: Throwable) { "" }
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
    onFileImport: (name: String, data: ByteArray) -> Unit = { _, _ -> },
) {
    val pc = LocalPilottyColors.current
    val ctx = LocalContext.current
    var name by remember { mutableStateOf("") }
    var url by remember { mutableStateOf("") }
    var pickError by remember { mutableStateOf<String?>(null) }
    val clipboardHit = remember { pickImportableFromClipboard(ctx) }

    // P1 · SAF 本地文件导入: MIME "*/*" 因为机场 .yaml 常被 ContentProvider 标成 application/octet-stream
    val fileLauncher = androidx.activity.compose.rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.OpenDocument(),
    ) { uri ->
        if (uri == null) return@rememberLauncherForActivityResult
        try {
            val bytes = ctx.contentResolver.openInputStream(uri)?.use { it.readBytes() }
                ?: run {
                    pickError = "无法读取文件"
                    return@rememberLauncherForActivityResult
                }
            val fileName = queryDisplayName(ctx, uri)
            val inferred = fileName.substringBeforeLast('.').ifEmpty { fileName.ifEmpty { "本地导入" } }
            onFileImport(name.ifBlank { inferred }, bytes)
        } catch (t: Throwable) {
            pickError = t.message ?: "读取失败"
        }
    }

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
                    PilottyButton(
                        text = "从文件导入",
                        onClick = { fileLauncher.launch(arrayOf("*/*")) },
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
                pickError?.let {
                    Text("文件读取失败: $it", color = pc.error, fontSize = 11.sp)
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
