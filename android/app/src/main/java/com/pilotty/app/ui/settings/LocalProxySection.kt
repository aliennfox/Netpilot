package com.pilotty.app.ui.settings

import android.content.Context
import android.net.ConnectivityManager
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.ui.components.SectionHead
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.theme.LocalPilottyColors
import com.pilotty.app.vpn.LocalProxyPrefs
import com.pilotty.app.vpn.PilottyVpnService
import java.net.Inet4Address

/**
 * A4 LAN 代理入站设置卡。
 *
 * 开关 + 端口, 没有鉴权 —— 设计上故意朴素, 让用户自己评估 "这 WiFi 我信不信得过"。
 * 启用后其他设备能把本手机当 SOCKS5 / HTTP 代理出口 (mixed inbound 同时响应两种协议)。
 */
@Composable
fun LocalProxySection() {
    val ctx = LocalContext.current
    val prefs = remember { LocalProxyPrefs.get(ctx) }
    val snapshot by prefs.state.collectAsState()
    val pc = LocalPilottyColors.current

    var portText by remember(snapshot.port) { mutableStateOf(snapshot.port.toString()) }
    var portError by remember { mutableStateOf<String?>(null) }
    val lanIp by produceState<String?>(initialValue = null, snapshot.enabled) {
        value = if (snapshot.enabled) getLanIpv4(ctx) else null
    }

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        SectionHead("LAN 代理入站")
        PilottyCard(modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            "SOCKS5 + HTTP 本地入站",
                            color = pc.ink,
                            fontSize = 14.sp,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Text(
                            if (snapshot.enabled) "已开 · LAN 里的设备可把这台手机当代理出口"
                            else "关闭中 · 只有手机自己的流量走 VPN",
                            color = pc.ink3,
                            fontSize = 11.5.sp,
                        )
                    }
                    Switch(
                        checked = snapshot.enabled,
                        onCheckedChange = {
                            prefs.setEnabled(it)
                            PilottyVpnService.requestReload(ctx)
                        },
                    )
                }

                if (snapshot.enabled) {
                    HorizontalDivider(color = pc.hairline, thickness = 1.dp)
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text("端口", color = pc.ink2, fontSize = 13.sp, modifier = Modifier.width(44.dp))
                        PilottyCard(modifier = Modifier.weight(1f), soft = true) {
                            BasicTextField(
                                value = portText,
                                onValueChange = { s ->
                                    portText = s.filter { it.isDigit() }.take(5)
                                    portError = null
                                },
                                singleLine = true,
                                textStyle = TextStyle(
                                    color = pc.ink,
                                    fontSize = 14.sp,
                                    fontFamily = FontFamily.Monospace,
                                ),
                                cursorBrush = SolidColor(pc.ink),
                                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(horizontal = 10.dp, vertical = 8.dp),
                            )
                        }
                        Text(
                            "应用",
                            color = pc.accent,
                            fontSize = 13.sp,
                            fontWeight = FontWeight.SemiBold,
                            modifier = Modifier
                                .clip(RoundedCornerShape(6.dp))
                                .clickable {
                                    val p = portText.toIntOrNull()
                                    if (p == null || p !in 1025..65535) {
                                        portError = "端口须在 1025-65535"
                                        return@clickable
                                    }
                                    prefs.setPort(p)
                                    PilottyVpnService.requestReload(ctx)
                                }
                                .padding(horizontal = 10.dp, vertical = 6.dp),
                        )
                    }
                    portError?.let {
                        Text(it, color = pc.error, fontSize = 11.sp)
                    }

                    if (!lanIp.isNullOrEmpty()) {
                        Text(
                            "连接方式: SOCKS5 / HTTP  →  $lanIp:${snapshot.port}",
                            color = pc.ink2,
                            fontSize = 12.sp,
                            fontFamily = FontFamily.Monospace,
                        )
                    } else {
                        Text(
                            "未获取到 LAN IP (WiFi 未连接或权限不足)。 启动 VPN 后端口会监听 0.0.0.0:${snapshot.port}",
                            color = pc.ink3,
                            fontSize = 11.5.sp,
                        )
                    }
                    Text(
                        "⚠ 此入站无鉴权, 同一 WiFi 里任何人知道端口都能借道。 仅在可信网络 (家 / 公司) 开启, 咖啡厅 / 公共 WiFi 切勿开启。",
                        color = pc.warn,
                        fontSize = 11.sp,
                        lineHeight = 15.sp,
                    )
                }
            }
        }
    }
}

/** 取当前非 VPN 网络 (WiFi / 以太网) 的 IPv4 地址。 拿不到就返回 null。 */
private fun getLanIpv4(ctx: Context): String? {
    return try {
        val cm = ctx.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager
            ?: return null
        @Suppress("DEPRECATION")
        cm.allNetworks.forEach { n ->
            val caps = cm.getNetworkCapabilities(n) ?: return@forEach
            if (caps.hasTransport(android.net.NetworkCapabilities.TRANSPORT_VPN)) return@forEach
            val props = cm.getLinkProperties(n) ?: return@forEach
            val addr = props.linkAddresses
                .map { it.address }
                .filterIsInstance<Inet4Address>()
                .firstOrNull { !it.isLoopbackAddress && !it.isLinkLocalAddress }
            if (addr != null) return addr.hostAddress
        }
        null
    } catch (_: Throwable) {
        null
    }
}
