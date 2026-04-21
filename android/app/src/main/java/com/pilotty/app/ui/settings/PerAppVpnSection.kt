package com.pilotty.app.ui.settings

import android.content.Context
import android.graphics.drawable.Drawable
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Checkbox
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.core.graphics.drawable.toBitmap
import com.pilotty.app.perapp.InstalledAppsLoader
import com.pilotty.app.perapp.PerAppVpnPrefs
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.components.PilottyChip
import com.pilotty.app.ui.theme.LocalPilottyColors
import com.pilotty.app.vpn.PilottyVpnService
import kotlinx.coroutines.launch

/**
 * Per-App VPN 设置卡 (M14)。
 *
 * 显示在 Settings 里, 展示当前模式 + 已选 App 数 + 打开 App 选择器 dialog 的入口。
 * 任何状态变更 → 写 PerAppVpnPrefs + 请求 VpnService reload (若在跑)。
 */
@Composable
fun PerAppVpnSection() {
    val ctx = LocalContext.current
    val prefs = remember { PerAppVpnPrefs.get(ctx) }
    val snapshot by prefs.state.collectAsState()
    var showPicker by rememberSaveable { mutableStateOf(false) }
    val pc = LocalPilottyColors.current

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Kicker("按 App 代理")
        PilottyCard(modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                Row(
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    PilottyChip(
                        text = "关闭",
                        selected = snapshot.mode == PerAppVpnPrefs.Mode.Off,
                        onClick = { prefs.setMode(PerAppVpnPrefs.Mode.Off); reloadIfRunning(ctx) },
                    )
                    PilottyChip(
                        text = "白名单",
                        selected = snapshot.mode == PerAppVpnPrefs.Mode.Allow,
                        onClick = { prefs.setMode(PerAppVpnPrefs.Mode.Allow); reloadIfRunning(ctx) },
                    )
                    PilottyChip(
                        text = "黑名单",
                        selected = snapshot.mode == PerAppVpnPrefs.Mode.Deny,
                        onClick = { prefs.setMode(PerAppVpnPrefs.Mode.Deny); reloadIfRunning(ctx) },
                    )
                }

                val modeDesc = when (snapshot.mode) {
                    PerAppVpnPrefs.Mode.Off -> "所有 App 流量走代理"
                    PerAppVpnPrefs.Mode.Allow -> "只有选中的 ${snapshot.packages.size} 个 App 走代理"
                    PerAppVpnPrefs.Mode.Deny -> "选中的 ${snapshot.packages.size} 个 App 直连, 其它走代理"
                }
                Text(modeDesc, color = pc.ink3, fontSize = 12.sp)

                if (snapshot.mode != PerAppVpnPrefs.Mode.Off) {
                    Text(
                        text = if (snapshot.packages.isEmpty()) "尚未选择 App → 点击下方按钮" else "已选 ${snapshot.packages.size} 个 App",
                        color = pc.ink,
                        fontSize = 13.sp,
                        fontWeight = FontWeight.Medium,
                    )
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clip(RoundedCornerShape(10.dp))
                            .background(pc.surface2)
                            .clickable { showPicker = true }
                            .padding(horizontal = 12.dp, vertical = 10.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text("选择应用 ...", color = pc.ink, fontSize = 13.sp, fontWeight = FontWeight.Medium)
                        Text("→", color = pc.ink3, fontSize = 14.sp)
                    }
                }

                Text(
                    "切换模式或选择后, VPN 若在运行会自动热重载一次 tun 配置。",
                    color = pc.ink4,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                )
            }
        }
    }

    if (showPicker) {
        AppPickerDialog(
            initial = snapshot.packages,
            onDismiss = { showPicker = false },
            onConfirm = { newSet ->
                prefs.setPackages(newSet)
                reloadIfRunning(ctx)
                showPicker = false
            },
        )
    }
}

private fun reloadIfRunning(ctx: Context) {
    PilottyVpnService.requestReload(ctx)
}

@Composable
private fun AppPickerDialog(
    initial: Set<String>,
    onDismiss: () -> Unit,
    onConfirm: (Set<String>) -> Unit,
) {
    val ctx = LocalContext.current
    val pc = LocalPilottyColors.current
    val scope = rememberCoroutineScope()

    var entries by remember { mutableStateOf<List<InstalledAppsLoader.Entry>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var query by rememberSaveable { mutableStateOf("") }
    val selected = remember(initial) { mutableStateListOf<String>().also { it.addAll(initial) } }

    LaunchedEffect(Unit) {
        scope.launch {
            entries = InstalledAppsLoader.load(ctx)
            loading = false
        }
    }

    val filtered = remember(entries, query) {
        val q = query.trim().lowercase()
        if (q.isEmpty()) entries
        else entries.filter { it.label.lowercase().contains(q) || it.packageName.lowercase().contains(q) }
    }

    Dialog(
        onDismissRequest = onDismiss,
        properties = DialogProperties(usePlatformDefaultWidth = false, dismissOnBackPress = true, dismissOnClickOutside = false),
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .background(pc.bg),
        ) {
            // 顶栏
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 12.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column {
                    Text("选择 App", color = pc.ink, fontSize = 16.sp, fontWeight = FontWeight.SemiBold)
                    Text("${selected.size} 选中", color = pc.ink3, fontSize = 11.sp, fontFamily = FontFamily.Monospace)
                }
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Text(
                        "取消",
                        color = pc.ink3,
                        fontSize = 13.sp,
                        modifier = Modifier.clickable { onDismiss() }.padding(6.dp),
                    )
                    Text(
                        "完成",
                        color = pc.accent,
                        fontSize = 13.sp,
                        fontWeight = FontWeight.SemiBold,
                        modifier = Modifier.clickable { onConfirm(selected.toSet()) }.padding(6.dp),
                    )
                }
            }
            HorizontalDivider(color = pc.hairline)

            // 搜索
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 10.dp)
                    .clip(RoundedCornerShape(10.dp))
                    .background(pc.surface2)
                    .padding(horizontal = 12.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                BasicTextField(
                    value = query,
                    onValueChange = { query = it },
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.None),
                    textStyle = androidx.compose.ui.text.TextStyle(color = pc.ink, fontSize = 13.sp),
                    cursorBrush = androidx.compose.ui.graphics.SolidColor(pc.accent),
                    modifier = Modifier.fillMaxWidth().weight(1f),
                    decorationBox = { inner ->
                        if (query.isEmpty()) {
                            Text("搜索 App 名或 package", color = pc.ink4, fontSize = 13.sp)
                        }
                        inner()
                    },
                )
                if (query.isNotEmpty()) {
                    Text(
                        "清除",
                        color = pc.ink3,
                        fontSize = 12.sp,
                        modifier = Modifier.clickable { query = "" }.padding(start = 6.dp),
                    )
                }
            }

            // 列表
            if (loading) {
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    Text("加载 App 列表中...", color = pc.ink3, fontSize = 13.sp)
                }
            } else {
                LazyColumn(modifier = Modifier.fillMaxSize()) {
                    items(filtered, key = { it.packageName }) { entry ->
                        val isSelected = selected.contains(entry.packageName)
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable {
                                    if (isSelected) selected.remove(entry.packageName)
                                    else selected.add(entry.packageName)
                                }
                                .padding(horizontal = 16.dp, vertical = 10.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(12.dp),
                        ) {
                            AppIcon(entry.icon)
                            Column(modifier = Modifier.weight(1f)) {
                                Text(entry.label, color = pc.ink, fontSize = 14.sp, fontWeight = FontWeight.Medium)
                                Text(
                                    entry.packageName + if (entry.isSystem) "  · system" else "",
                                    color = pc.ink4,
                                    fontSize = 11.sp,
                                    fontFamily = FontFamily.Monospace,
                                )
                            }
                            Checkbox(checked = isSelected, onCheckedChange = null)
                        }
                        HorizontalDivider(color = pc.hairline.copy(alpha = 0.5f))
                    }
                }
            }
        }
    }
}

@Composable
private fun AppIcon(drawable: Drawable) {
    val bmp = remember(drawable) {
        runCatching { drawable.toBitmap(width = 40, height = 40) }.getOrNull()
    }
    if (bmp != null) {
        androidx.compose.foundation.Image(
            bitmap = bmp.asImageBitmap(),
            contentDescription = null,
            modifier = Modifier.size(28.dp),
        )
    } else {
        Box(
            modifier = Modifier
                .size(28.dp)
                .clip(RoundedCornerShape(6.dp))
                .background(Color.Gray.copy(alpha = 0.3f)),
        )
    }
}
