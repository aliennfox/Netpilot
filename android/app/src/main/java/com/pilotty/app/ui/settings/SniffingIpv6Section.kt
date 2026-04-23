package com.pilotty.app.ui.settings

import android.content.Context
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.ui.components.SectionHead
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.components.PilottyChip
import com.pilotty.app.ui.theme.LocalPilottyColors
import com.pilotty.app.vpn.PilottyVpnService
import com.pilotty.app.vpn.SniffingIpv6Prefs

/**
 * Phase P1-A: Sniffing 档位 + IPv6 模式 UI。
 *
 * 两项都是 sing-box 后端已吃的 option, 本轮只补 UI 入口。
 *  - Sniff on → route.rules[0] 塞 `{action:"sniff"}`, 允许域名类规则命中 IP-直连应用
 *  - IPv6 模式 → 改 dns.strategy + 按需剥 tun address 的 v4/v6 段
 * 改动立即写 SharedPreferences + 若 VPN 在跑则请求 libbox 热重载。
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun SniffingIpv6Section() {
    val ctx = LocalContext.current
    val pc = LocalPilottyColors.current
    val prefs = remember { SniffingIpv6Prefs.get(ctx) }
    val snapshot by prefs.state.collectAsState()

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        SectionHead("网络引擎")
        PilottyCard(modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {

                // --- Sniffing switch ---
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text(
                            "流量嗅探 (Sniffing)",
                            color = pc.ink,
                            fontSize = 14.sp,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Spacer(Modifier.height(2.dp))
                        Text(
                            "从 TCP/UDP 首包嗅出域名, 让 domain_suffix / keyword 类规则能命中直接走 IP 的 App (如部分游戏)。 关掉省极少 CPU, 但域名规则命中率会掉。",
                            color = pc.ink3,
                            fontSize = 11.sp,
                        )
                    }
                    Switch(
                        checked = snapshot.sniff,
                        onCheckedChange = {
                            prefs.setSniff(it)
                            PilottyVpnService.requestReload(ctx)
                        },
                        colors = SwitchDefaults.colors(
                            checkedThumbColor = pc.accent,
                            checkedTrackColor = pc.accent.copy(alpha = 0.35f),
                        ),
                    )
                }

                // --- IPv6 mode chips ---
                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text(
                        "IPv6 模式",
                        color = pc.ink,
                        fontSize = 14.sp,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Text(
                        "DNS 解析偏好 + tun 接口地址族。 多数用户用 「优先 IPv4」 兼容性最佳。",
                        color = pc.ink3,
                        fontSize = 11.sp,
                    )
                    FlowRow(
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                        verticalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        SniffingIpv6Prefs.Ipv6Mode.values().forEach { mode ->
                            PilottyChip(
                                text = mode.label,
                                selected = snapshot.ipv6Mode == mode,
                                onClick = {
                                    prefs.setIpv6Mode(mode)
                                    PilottyVpnService.requestReload(ctx)
                                },
                            )
                        }
                    }
                    Text(
                        "raw=${snapshot.ipv6Mode.raw} · sniff=${if (snapshot.sniff) "on" else "off"}",
                        color = pc.ink4,
                        fontSize = 10.sp,
                        fontFamily = FontFamily.Monospace,
                    )
                }
            }
        }
    }
}
