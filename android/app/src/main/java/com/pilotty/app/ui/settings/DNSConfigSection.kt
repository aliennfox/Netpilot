package com.pilotty.app.ui.settings

import android.util.Log
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.data.DNSConfigDto
import com.pilotty.app.data.DNSServerDto
import com.pilotty.app.data.PilottyRepository
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.PilottyButton
import com.pilotty.app.ui.components.PilottyButtonVariant
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.components.PilottyChip
import com.pilotty.app.ui.theme.LocalPilottyColors
import kotlinx.coroutines.launch

/**
 * Phase P1-C: 自定义 DNS / DoH / DoQ UI。
 *
 * 后端 overlay 只支持整块替换 (SetDNS), 不做单 server/rule CRUD — 所以 UI 侧做成:
 *  1. 列表显示当前 Servers (tag / type / server · port · detour), 长按 server 可选为 "默认 (final)" 或删除
 *  2. "添加服务器" 按钮 → Dialog 表单 (type chip + tag + server + port + detour chip)
 *  3. 3 个预设快速添加: Cloudflare DoH / Google DoT / AliDNS UDP
 *  4. "恢复默认" 按钮 → 重置到 secure 模式 (8.8.8.8 DoT via proxy-group)
 *
 * Rules CRUD 不做, 交给 Chat Agent (LLM 已能生成 DNS rule JSON)。
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun DNSConfigSection() {
    val pc = LocalPilottyColors.current
    val scope = rememberCoroutineScope()
    var config by remember { mutableStateOf<DNSConfigDto?>(null) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var addDialogOpen by remember { mutableStateOf(false) }
    var contextServer by remember { mutableStateOf<DNSServerDto?>(null) }

    fun reload() = scope.launch {
        try {
            config = PilottyRepository.dnsConfig()
            error = null
        } catch (t: Throwable) {
            Log.w(TAG, "dnsConfig failed", t)
            error = t.message
        }
    }

    LaunchedEffect(Unit) { reload() }

    fun save(newCfg: DNSConfigDto) = scope.launch {
        busy = true
        try {
            PilottyRepository.setDnsConfig(newCfg)
            config = newCfg
            error = null
        } catch (t: Throwable) {
            Log.w(TAG, "setDnsConfig failed", t)
            error = t.message
        } finally {
            busy = false
        }
    }

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Kicker("DNS")
        PilottyCard(modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {

                Row(verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text(
                            "DNS 服务器",
                            color = pc.ink,
                            fontSize = 14.sp,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Spacer(Modifier.height(2.dp))
                        Text(
                            "订阅一个 server 作为 default, 其余按 DNS 规则或 action=route 分流。 Rules 请用 Chat 修改。",
                            color = pc.ink3,
                            fontSize = 11.sp,
                        )
                    }
                }

                val cfg = config
                if (cfg == null) {
                    Text("加载中...", color = pc.ink4, fontSize = 11.sp, fontFamily = FontFamily.Monospace)
                } else {
                    if (cfg.servers.isEmpty()) {
                        Text("没有已配置的 DNS 服务器", color = pc.ink3, fontSize = 11.sp)
                    } else {
                        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            cfg.servers.forEach { s ->
                                DNSServerRow(
                                    server = s,
                                    isFinal = s.tag == cfg.final,
                                    onClick = { contextServer = s },
                                )
                            }
                        }
                    }

                    Spacer(Modifier.height(2.dp))
                    Text(
                        "default=${cfg.final.ifEmpty { "-" }} · strategy=${cfg.strategy.ifEmpty { "-" }} · rules=${cfg.rules.size}",
                        color = pc.ink4,
                        fontSize = 10.sp,
                        fontFamily = FontFamily.Monospace,
                    )
                }

                error?.let {
                    Text("错误: $it", color = pc.error, fontSize = 11.sp)
                }

                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    PilottyButton(
                        text = "添加服务器",
                        onClick = { addDialogOpen = true },
                        variant = PilottyButtonVariant.Primary,
                        small = true,
                        enabled = !busy,
                    )
                    PilottyButton(
                        text = "恢复默认",
                        onClick = {
                            scope.launch {
                                busy = true
                                try {
                                    PilottyRepository.resetDnsConfig()
                                    reload()
                                } catch (t: Throwable) {
                                    error = t.message
                                } finally {
                                    busy = false
                                }
                            }
                        },
                        variant = PilottyButtonVariant.Ghost,
                        small = true,
                        enabled = !busy,
                    )
                }

                // 3 个预设快速添加
                val cfg2 = config
                if (cfg2 != null) {
                    FlowRow(
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                        verticalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        Presets.list.forEach { preset ->
                            PilottyChip(
                                text = "+ ${preset.label}",
                                selected = false,
                                onClick = {
                                    // 去重: 按 tag 匹配, 已存在则跳过
                                    if (cfg2.servers.any { it.tag == preset.server.tag }) {
                                        error = "${preset.server.tag} 已存在"
                                        return@PilottyChip
                                    }
                                    save(cfg2.copy(servers = cfg2.servers + preset.server))
                                },
                            )
                        }
                    }
                }
            }
        }
    }

    val cfgNow = config
    if (addDialogOpen && cfgNow != null) {
        AddDNSServerDialog(
            onDismiss = { addDialogOpen = false },
            onConfirm = { newServer ->
                if (cfgNow.servers.any { it.tag == newServer.tag }) {
                    error = "tag ${newServer.tag} 已存在"
                } else {
                    save(cfgNow.copy(servers = cfgNow.servers + newServer))
                }
                addDialogOpen = false
            },
        )
    }

    contextServer?.let { s ->
        val cur = config
        if (cur != null) {
            ServerActionDialog(
                server = s,
                isFinal = s.tag == cur.final,
                canDelete = cur.servers.size > 1,
                onDismiss = { contextServer = null },
                onSetFinal = {
                    save(cur.copy(final = s.tag))
                    contextServer = null
                },
                onDelete = {
                    val newFinal = if (cur.final == s.tag) {
                        cur.servers.firstOrNull { it.tag != s.tag }?.tag.orEmpty()
                    } else cur.final
                    save(cur.copy(servers = cur.servers.filter { it.tag != s.tag }, final = newFinal))
                    contextServer = null
                },
            )
        }
    }
}

@Composable
private fun DNSServerRow(server: DNSServerDto, isFinal: Boolean, onClick: () -> Unit) {
    val pc = LocalPilottyColors.current
    PilottyCard(
        modifier = Modifier.fillMaxWidth(),
        soft = true,
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(10.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text(
                        server.tag,
                        color = if (isFinal) pc.accentInk else pc.ink,
                        fontSize = 13.sp,
                        fontWeight = FontWeight.SemiBold,
                    )
                    if (isFinal) {
                        Text("· default", color = pc.accent, fontSize = 10.sp, fontFamily = FontFamily.Monospace)
                    }
                }
                val detail = buildList {
                    add(server.type)
                    if (server.server.isNotEmpty()) add(server.server)
                    if (server.serverPort > 0) add(":${server.serverPort}")
                    if (server.detour.isNotEmpty()) add("via ${server.detour}")
                }.joinToString(" ")
                Text(
                    detail,
                    color = pc.ink3,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                )
            }
            TextButton(onClick = onClick) {
                Text("...", color = pc.ink4, fontSize = 14.sp, fontWeight = FontWeight.Bold)
            }
        }
    }
}

@Composable
private fun AddDNSServerDialog(
    onDismiss: () -> Unit,
    onConfirm: (DNSServerDto) -> Unit,
) {
    val pc = LocalPilottyColors.current
    var type by remember { mutableStateOf("tls") }
    var tag by remember { mutableStateOf("") }
    var server by remember { mutableStateOf("") }
    var port by remember { mutableStateOf("") }
    var detour by remember { mutableStateOf("proxy-group") }

    // port 按 type 联动默认
    LaunchedEffect(type) {
        if (port.isBlank()) {
            port = when (type) {
                "tls", "quic" -> "853"
                "https", "h3" -> "443"
                "udp", "tcp" -> "53"
                else -> ""
            }
        }
    }

    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = pc.surface,
        title = { Text("添加 DNS 服务器", color = pc.ink) },
        text = {
            @OptIn(ExperimentalLayoutApi::class)
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("协议", color = pc.ink2, fontSize = 11.sp)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(4.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    listOf("udp", "tcp", "tls", "https", "quic", "h3", "local").forEach { t ->
                        PilottyChip(text = t, selected = type == t, onClick = { type = t })
                    }
                }
                OutlinedTextField(
                    value = tag, onValueChange = { tag = it },
                    label = { Text("Tag (唯一)") }, singleLine = true,
                    colors = OutlinedTextFieldDefaults.colors(focusedBorderColor = pc.ink, unfocusedBorderColor = pc.hairlineStrong),
                )
                OutlinedTextField(
                    value = server, onValueChange = { server = it },
                    label = { Text("地址 (IP 或域名, local 类型留空)") }, singleLine = true,
                    colors = OutlinedTextFieldDefaults.colors(focusedBorderColor = pc.ink, unfocusedBorderColor = pc.hairlineStrong),
                )
                OutlinedTextField(
                    value = port, onValueChange = { port = it.filter { c -> c.isDigit() } },
                    label = { Text("端口") }, singleLine = true,
                    colors = OutlinedTextFieldDefaults.colors(focusedBorderColor = pc.ink, unfocusedBorderColor = pc.hairlineStrong),
                )
                Text("Detour (哪个 outbound 出去)", color = pc.ink2, fontSize = 11.sp)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(4.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    listOf("proxy-group", "direct-out", "").forEach { d ->
                        PilottyChip(
                            text = if (d.isEmpty()) "不指定" else d,
                            selected = detour == d,
                            onClick = { detour = d },
                        )
                    }
                }
            }
        },
        confirmButton = {
            TextButton(
                onClick = {
                    onConfirm(
                        DNSServerDto(
                            type = type,
                            tag = tag.trim(),
                            server = server.trim(),
                            serverPort = port.toIntOrNull() ?: 0,
                            detour = detour,
                        )
                    )
                },
                enabled = tag.isNotBlank() && (type == "local" || server.isNotBlank()),
            ) { Text("添加", color = if (tag.isNotBlank()) pc.accentInk else pc.ink4) }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("取消", color = pc.ink2) }
        },
    )
}

@Composable
private fun ServerActionDialog(
    server: DNSServerDto,
    isFinal: Boolean,
    canDelete: Boolean,
    onDismiss: () -> Unit,
    onSetFinal: () -> Unit,
    onDelete: () -> Unit,
) {
    val pc = LocalPilottyColors.current
    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = pc.surface,
        title = { Text(server.tag, color = pc.ink) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(
                    "type=${server.type} · ${server.server}${if (server.serverPort > 0) ":${server.serverPort}" else ""}${if (server.detour.isNotEmpty()) " via ${server.detour}" else ""}",
                    color = pc.ink3,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                )
                if (!isFinal) {
                    TextButton(onClick = onSetFinal) {
                        Text("设为默认 (default)", color = pc.accentInk)
                    }
                }
                if (canDelete) {
                    TextButton(onClick = onDelete) {
                        Text("删除", color = pc.error)
                    }
                } else {
                    Text("不能删除最后一个 server", color = pc.ink4, fontSize = 10.sp)
                }
            }
        },
        confirmButton = {},
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("关闭", color = pc.ink2) }
        },
    )
}

/**
 * 预设 DNS 服务器 (常见免费 DoH / DoT / UDP)。
 * 所有 `detour=proxy-group` 的预设保证走代理 (避免本地 DNS 污染);
 * `direct-out` 的预设用于国内直连 + 本地 resolver。
 */
private object Presets {
    data class Preset(val label: String, val server: DNSServerDto)

    val list = listOf(
        Preset(
            "Cloudflare DoH",
            DNSServerDto(type = "https", tag = "cloudflare-doh", server = "1.1.1.1", serverPort = 443, detour = "proxy-group"),
        ),
        Preset(
            "Google DoT",
            DNSServerDto(type = "tls", tag = "google-dot", server = "8.8.8.8", serverPort = 853, detour = "proxy-group"),
        ),
        Preset(
            "AliDNS UDP",
            DNSServerDto(type = "udp", tag = "ali-dns", server = "223.5.5.5", serverPort = 53, detour = "direct-out"),
        ),
        Preset(
            "DNSPod UDP",
            DNSServerDto(type = "udp", tag = "dnspod-dns", server = "119.29.29.29", serverPort = 53, detour = "direct-out"),
        ),
    )
}

private const val TAG = "DNSConfigSection"
