package com.pilotty.app.ui.settings

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.PowerManager
import android.provider.Settings
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.PilottyButton
import com.pilotty.app.ui.components.PilottyButtonVariant
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.theme.LocalPilottyColors

/**
 * Phase 10-C: Android 稳定性 / 合规三件套 (通知权限 + 电池白名单), 二者都是 VPN 类 App 的底层约束:
 *  - Android 13 (API 33) 引入 POST_NOTIFICATIONS 运行时权限, 未授予则前台服务 notification 无法显示,
 *    部分 ROM 进一步会把 service 优先级降到可被杀。 Play 合规也要求显式 prompt。
 *  - Doze / App Standby 会在屏幕灭且无交互后 kill 后台连接; VPN foreground service 在无电池白名单
 *    时仍可能被 OEM 定制的省电策略清掉。 不是所有厂商都守 foregroundServiceType=systemExempted。
 *
 * 策略: 不偷偷代劳, 每一步都有状态卡 + 明确按钮, 用户知道自己给了什么权限。
 */
@Composable
fun BatteryAndPermsSection() {
    val ctx = LocalContext.current
    val pc = LocalPilottyColors.current

    // ---- 通知权限 state ----
    var notificationGranted by remember {
        mutableStateOf(checkNotificationGranted(ctx))
    }
    val notifLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        notificationGranted = granted
    }
    // 从系统设置返回时也刷一次状态(用户可能进去关了再回来)
    LaunchedEffect(Unit) {
        notificationGranted = checkNotificationGranted(ctx)
    }

    // ---- 电池白名单 state ----
    var batteryIgnored by remember { mutableStateOf(checkBatteryIgnored(ctx)) }
    // 从系统设置返回同样刷新
    LaunchedEffect(Unit) {
        batteryIgnored = checkBatteryIgnored(ctx)
    }

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Kicker("稳定性 & 权限")

        // 通知权限 (Android 13+ 才需要 runtime prompt; API < 33 隐式授予)
        PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
            Column(Modifier.padding(14.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        "通知权限",
                        color = pc.ink,
                        fontSize = 14.sp,
                        fontWeight = FontWeight.SemiBold,
                        modifier = Modifier.weight(1f),
                    )
                    StatusBadge(granted = notificationGranted, labelOk = "已授予", labelBad = "未授予")
                }
                Spacer(Modifier.height(4.dp))
                Text(
                    "Android 13+ 需要明示授权,前台服务的通知才能显示,VPN 才不易被后台清理。",
                    color = pc.ink3,
                    fontSize = 11.sp,
                )
                if (!notificationGranted && Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                    Spacer(Modifier.height(10.dp))
                    PilottyButton(
                        text = "申请通知权限",
                        onClick = { notifLauncher.launch(Manifest.permission.POST_NOTIFICATIONS) },
                        variant = PilottyButtonVariant.Outline,
                        small = true,
                    )
                }
            }
        }

        // 电池白名单 (所有 API 级别都有, API 23 起可查询)
        PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
            Column(Modifier.padding(14.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        "电池白名单",
                        color = pc.ink,
                        fontSize = 14.sp,
                        fontWeight = FontWeight.SemiBold,
                        modifier = Modifier.weight(1f),
                    )
                    StatusBadge(granted = batteryIgnored, labelOk = "已加入", labelBad = "未加入")
                }
                Spacer(Modifier.height(4.dp))
                Text(
                    "把 Pilotty 从系统 Doze / 省电策略里排除,避免长时间后台被 OEM 清理。系统列表里找 Pilotty 选\"允许\"。",
                    color = pc.ink3,
                    fontSize = 11.sp,
                )
                Spacer(Modifier.height(10.dp))
                PilottyButton(
                    text = if (batteryIgnored) "打开电池优化设置" else "申请加入白名单",
                    onClick = { launchBatterySettings(ctx) },
                    variant = PilottyButtonVariant.Outline,
                    small = true,
                )
            }
        }
    }
}

@Composable
private fun StatusBadge(granted: Boolean, labelOk: String, labelBad: String) {
    val pc = LocalPilottyColors.current
    Text(
        if (granted) "● $labelOk" else "○ $labelBad",
        color = if (granted) pc.accentInk else pc.ink4,
        fontSize = 11.sp,
        fontFamily = FontFamily.Monospace,
        fontWeight = FontWeight.Medium,
    )
}

private fun checkNotificationGranted(ctx: Context): Boolean {
    if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) return true
    return ContextCompat.checkSelfPermission(
        ctx,
        Manifest.permission.POST_NOTIFICATIONS,
    ) == PackageManager.PERMISSION_GRANTED
}

private fun checkBatteryIgnored(ctx: Context): Boolean {
    val pm = ctx.getSystemService(Context.POWER_SERVICE) as? PowerManager ?: return false
    return pm.isIgnoringBatteryOptimizations(ctx.packageName)
}

/**
 * 打开系统电池优化设置。 优先跳 Pilotty 专属申请页 (一次点击), 失败回落到全局列表。
 * 专属申请页需 REQUEST_IGNORE_BATTERY_OPTIMIZATIONS permission 声明, 否则系统拒绝 Intent。
 */
private fun launchBatterySettings(ctx: Context) {
    // 直接跳全局列表最稳; Pilotty manifest 没声明 REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, 不走专属申请
    val intent = Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS).apply {
        addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
    }
    try {
        ctx.startActivity(intent)
        return
    } catch (_: Throwable) {
        // 极少数 ROM 无此 action
    }
    // 兜底: App 详情页, 用户手工找到"电池"选项
    try {
        val fallback = Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS).apply {
            data = Uri.parse("package:${ctx.packageName}")
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        ctx.startActivity(fallback)
    } catch (_: Throwable) {
        // 无能为力
    }
}
