package com.pilotty.app.ui.components

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.ui.theme.LocalPilottyColors

/**
 * MissionStrip —— 顶部状态条 NOMINAL / VPN ON · T+uptime
 * Home 屏专用, 放在系统 status bar 之下。
 */
@Composable
fun MissionStrip(
    state: String = "nominal",
    vpn: Boolean = true,
    uptime: String = "02:14:37",
    modifier: Modifier = Modifier,
) {
    val pc = LocalPilottyColors.current
    val color = when (state) {
        "alert" -> pc.error
        "warn" -> pc.warn
        else -> pc.accent
    }
    val label = when (state) {
        "alert" -> "CRITICAL"
        "warn" -> "CAUTION"
        else -> "NOMINAL"
    }
    Row(
        modifier = modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            StatusDot(state = state)
            Text(
                text = label,
                color = color,
                fontSize = 10.sp,
                fontWeight = FontWeight.Bold,
                letterSpacing = 1.5.sp,
            )
        }
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Text(
                text = "VPN ${if (vpn) "ON" else "OFF"}",
                color = if (vpn) pc.ink else pc.ink3,
                fontSize = 10.sp,
                fontWeight = FontWeight.Bold,
                letterSpacing = 1.5.sp,
            )
            // 只有 VPN 运行才显 uptime, 否则 00:00:00 没意义且挤字
            if (vpn) {
                Text(
                    text = "T+$uptime",
                    color = pc.ink2,
                    fontSize = 10.sp,
                    fontFamily = FontFamily.Monospace,
                )
            }
        }
    }
}

/**
 * TelemetryTile —— 2x2 Home 网格的单元格。 label + 可选 icon + 大号数字 + mini chart。
 */
@Composable
fun TelemetryTile(
    label: String,
    modifier: Modifier = Modifier,
    state: String = "nominal",
    icon: (@Composable () -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    val pc = LocalPilottyColors.current
    val border = if (state == "alert") BorderStroke(1.5.dp, pc.error) else null
    Surface(
        modifier = modifier.defaultMinSize(minHeight = 112.dp),
        shape = RoundedCornerShape(16.dp),
        color = pc.surface,
        tonalElevation = 0.dp,
        shadowElevation = if (pc.isDark) 0.dp else 4.dp,
        border = border ?: (if (pc.isDark) BorderStroke(1.dp, pc.hairline) else null),
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Kicker(text = label)
                if (icon != null) {
                    Box(contentAlignment = Alignment.Center) { icon() }
                }
            }
            Column(
                modifier = Modifier.weight(1f),
                verticalArrangement = Arrangement.Bottom,
                content = content,
            )
        }
    }
}

/**
 * Row —— 卡组列表的单行。 label + value (可选) + 右侧元素。
 */
@Composable
fun CardRow(
    label: String,
    modifier: Modifier = Modifier,
    hint: String? = null,
    value: String? = null,
    valueColor: Color? = null,
    valueMono: Boolean = true,
    onClick: (() -> Unit)? = null,
    trailing: (@Composable () -> Unit)? = null,
) {
    val pc = LocalPilottyColors.current
    val base = modifier.fillMaxWidth()
    val clickMod = if (onClick != null) base.clickable { onClick() } else base
    Row(
        modifier = clickMod.padding(horizontal = 18.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Column(
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            Text(
                text = label,
                color = pc.ink,
                fontSize = 14.sp,
                fontWeight = FontWeight.Normal,
                letterSpacing = (-0.07).sp,
            )
            if (hint != null) {
                Text(
                    text = hint,
                    color = pc.ink3,
                    fontSize = 11.5.sp,
                )
            }
        }
        if (value != null) {
            Text(
                text = value,
                color = valueColor ?: pc.ink2,
                fontSize = 13.sp,
                fontFamily = if (valueMono) FontFamily.Monospace else FontFamily.SansSerif,
            )
        }
        trailing?.invoke()
    }
}

/**
 * 行分隔线 —— card-group 内部。
 */
@Composable
fun RowDivider(modifier: Modifier = Modifier) {
    val pc = LocalPilottyColors.current
    Box(
        modifier = modifier
            .fillMaxWidth()
            .height(1.dp)
            .background(pc.hairline),
    )
}
