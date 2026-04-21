package com.pilotty.app.ui.components

import androidx.compose.animation.core.*
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.ui.theme.LocalPilottyColors

/**
 * 无边框抬升卡片 —— 对齐设计稿 .card (box-shadow 两层柔和阴影)。
 * Android 侧用 Material3 Surface + shadowElevation 逼近;shape 14dp 圆角。
 */
@Composable
fun PilottyCard(
    modifier: Modifier = Modifier,
    strong: Boolean = false,
    soft: Boolean = false,
    content: @Composable () -> Unit,
) {
    val pc = LocalPilottyColors.current
    val elevation = when {
        strong -> 6.dp
        soft -> 1.dp
        else -> 3.dp
    }
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(14.dp),
        color = pc.surface,
        tonalElevation = 0.dp,
        shadowElevation = if (pc.isDark) 0.dp else elevation,
        border = if (pc.isDark) BorderStroke(1.dp, pc.hairline) else null,
    ) {
        content()
    }
}

/**
 * Kicker — 全大写小号行号, 用于分区标题。
 */
@Composable
fun Kicker(text: String, modifier: Modifier = Modifier) {
    val pc = LocalPilottyColors.current
    Text(
        text = text.uppercase(),
        modifier = modifier,
        color = pc.ink3,
        fontSize = 10.sp,
        fontWeight = FontWeight.SemiBold,
        letterSpacing = 1.4.sp,
    )
}

/**
 * Live 脉冲小圆点 —— 柠檬绿呼吸灯。
 */
@Composable
fun LiveDot(modifier: Modifier = Modifier) {
    val pc = LocalPilottyColors.current
    val infinite = rememberInfiniteTransition(label = "livedot")
    val scale by infinite.animateFloat(
        initialValue = 1f,
        targetValue = 2.6f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 1600, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Restart,
        ),
        label = "livedot-scale",
    )
    val alpha by infinite.animateFloat(
        initialValue = 0.55f,
        targetValue = 0f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 1600, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Restart,
        ),
        label = "livedot-alpha",
    )
    Box(
        modifier = modifier.size(16.dp),
        contentAlignment = Alignment.Center,
    ) {
        // 外层脉冲圈
        Box(
            modifier = Modifier
                .size((6 * scale).dp.coerceAtMost(16.dp))
                .clip(CircleShape)
                .background(pc.accent.copy(alpha = alpha)),
        )
        // 核心点
        Box(
            modifier = Modifier
                .size(6.dp)
                .clip(CircleShape)
                .background(pc.accent),
        )
    }
}

/**
 * 静态圆点 (无动画, 用于列表分隔 / 非活跃状态)。
 */
@Composable
fun StaticDot(color: Color, modifier: Modifier = Modifier, size: Int = 6) {
    Box(
        modifier = modifier
            .size(size.dp)
            .clip(CircleShape)
            .background(color),
    )
}

/**
 * 国家代码胸牌 —— 2 字母大写单字母色块。
 */
@Composable
fun FlagChip(code: String, modifier: Modifier = Modifier) {
    val pc = LocalPilottyColors.current
    Box(
        modifier = modifier
            .size(width = 26.dp, height = 18.dp)
            .clip(RoundedCornerShape(4.dp))
            .background(pc.surface3),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = code.uppercase(),
            color = pc.ink2,
            fontSize = 10.sp,
            fontWeight = FontWeight.Bold,
            letterSpacing = 0.4.sp,
            fontFamily = FontFamily.Monospace,
        )
    }
}

/**
 * 负载条 —— 灰底进度条, 宽 42dp。
 */
@Composable
fun LoadBar(pct: Int, modifier: Modifier = Modifier) {
    val pc = LocalPilottyColors.current
    Box(
        modifier = modifier
            .size(width = 42.dp, height = 4.dp)
            .clip(RoundedCornerShape(2.dp))
            .background(pc.hairline),
    ) {
        Box(
            modifier = Modifier
                .fillMaxHeight()
                .fillMaxWidth(pct.coerceIn(0, 100) / 100f)
                .background(pc.ink2),
        )
    }
}

/**
 * 胸牌 chip —— 固定高 30dp, 边框样式。用于筛选 / 模式切换。
 */
@Composable
fun PilottyChip(
    text: String,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    leading: (@Composable () -> Unit)? = null,
) {
    val pc = LocalPilottyColors.current
    val bg = if (selected) pc.ink else pc.surface
    val fg = if (selected) pc.bg else pc.ink2
    Surface(
        modifier = modifier.height(30.dp),
        shape = RoundedCornerShape(10.dp),
        color = bg,
        border = if (selected) null else BorderStroke(1.dp, pc.hairlineStrong),
        onClick = onClick,
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            leading?.invoke()
            Text(
                text = text,
                color = fg,
                fontSize = 12.5.sp,
                fontWeight = FontWeight.Medium,
                letterSpacing = (-0.05).sp,
            )
        }
    }
}

/**
 * 按钮 —— 3 种 variant: primary (黑底白字) / accent (柠檬绿) / outline (默认)。
 */
enum class PilottyButtonVariant { Primary, Accent, Outline, Ghost }

@Composable
fun PilottyButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    variant: PilottyButtonVariant = PilottyButtonVariant.Outline,
    small: Boolean = false,
    leading: (@Composable () -> Unit)? = null,
    enabled: Boolean = true,
) {
    val pc = LocalPilottyColors.current
    val (bg, fg, border) = when (variant) {
        PilottyButtonVariant.Primary -> Triple(pc.ink, pc.bg, null)
        PilottyButtonVariant.Accent -> Triple(pc.accent, PilottyColorsAccentOn, null)
        PilottyButtonVariant.Outline -> Triple(pc.surface, pc.ink, BorderStroke(1.dp, pc.hairlineStrong))
        PilottyButtonVariant.Ghost -> Triple(Color.Transparent, pc.ink2, null)
    }
    val height = if (small) 28.dp else 38.dp
    val shape = if (small) RoundedCornerShape(8.dp) else RoundedCornerShape(10.dp)
    Surface(
        modifier = modifier.height(height),
        shape = shape,
        color = if (enabled) bg else bg.copy(alpha = 0.5f),
        border = border,
        onClick = onClick,
        enabled = enabled,
    ) {
        Row(
            modifier = Modifier.padding(horizontal = if (small) 10.dp else 14.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            leading?.invoke()
            Text(
                text = text,
                color = fg,
                fontSize = if (small) 12.sp else 13.5.sp,
                fontWeight = if (variant == PilottyButtonVariant.Accent) FontWeight.SemiBold else FontWeight.Medium,
                letterSpacing = (-0.05).sp,
            )
        }
    }
}

private val PilottyColorsAccentOn = Color(0xFF1A2300)
