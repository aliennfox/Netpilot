package com.pilotty.app.ui.settings

import android.content.Intent
import android.provider.Settings
import androidx.compose.foundation.layout.*
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.PilottyButton
import com.pilotty.app.ui.components.PilottyButtonVariant
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.theme.LocalPilottyColors

/**
 * Kill Switch 引导卡 (M15 方式 A)。
 *
 * 为什么是引导式而不是自己实现:
 *  - Android 不允许普通 App 持有 "拒绝所有非 VPN 流量" 的网络权限
 *  - 系统内置的 "始终开启 VPN + 屏蔽非 VPN 连接" (lockdown) 是唯一权威答案
 *  - 它由 SystemUI 维护, VPN 崩溃/切换/重启场景都走系统层兜底
 *
 * 因此这里只做 Intent 跳转 + 教育文案, 把用户一跳带到系统 VPN 设置页,
 * 让他手动勾选 Pilotty 旁边的齿轮 → "始终开启的 VPN" + "屏蔽非此 VPN 的连接"。
 */
@Composable
fun KillSwitchSection() {
    val ctx = LocalContext.current
    val pc = LocalPilottyColors.current

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Kicker("防泄漏 · Kill Switch")
        PilottyCard(modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                Text(
                    "VPN 若意外断开, 流量会明文回到物理网络 (Wi-Fi / 4G), 隐私短暂暴露。",
                    color = pc.ink,
                    fontSize = 13.sp,
                    fontWeight = FontWeight.Medium,
                )
                Text(
                    "Android 不允许普通 App 自己拦住裸奔流量 —— 真正管用的是系统内置的" +
                        " \"始终开启 VPN + 屏蔽非 VPN 连接\" (lockdown) 模式, 由 SystemUI 兜底。",
                    color = pc.ink3,
                    fontSize = 12.sp,
                )
                Column(
                    verticalArrangement = Arrangement.spacedBy(4.dp),
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    StepRow("1", "下方按钮 → 系统 VPN 设置页")
                    StepRow("2", "在 Pilotty 右侧点齿轮图标")
                    StepRow("3", "勾选「始终开启的 VPN」")
                    StepRow("4", "勾选「屏蔽非此 VPN 的连接」 (lockdown)")
                }
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    PilottyButton(
                        text = "打开系统 VPN 设置",
                        onClick = {
                            val intent = Intent(Settings.ACTION_VPN_SETTINGS).apply {
                                flags = Intent.FLAG_ACTIVITY_NEW_TASK
                            }
                            runCatching { ctx.startActivity(intent) }
                        },
                        variant = PilottyButtonVariant.Primary,
                        small = true,
                    )
                }
                Text(
                    "注: lockdown 模式会在 VPN 未建立前拦住全部流量, 开启前请先在 Pilotty 里试连一次, 避免" +
                        " \"开了 lockdown 却没建立 VPN → 手机全程断网\" 的窘境。",
                    color = pc.ink4,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                )
            }
        }
    }
}

@Composable
private fun StepRow(num: String, text: String) {
    val pc = LocalPilottyColors.current
    Row(
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.Top,
    ) {
        Text(
            num,
            color = pc.accent,
            fontSize = 12.sp,
            fontWeight = FontWeight.SemiBold,
            fontFamily = FontFamily.Monospace,
            modifier = Modifier.width(16.dp),
        )
        Text(
            text,
            color = pc.ink2,
            fontSize = 12.5.sp,
        )
    }
}
