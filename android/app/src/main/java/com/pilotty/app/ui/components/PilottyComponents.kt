package com.pilotty.app.ui.components

import androidx.compose.animation.core.*
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.ui.theme.LocalPilottyColors

/**
 * 设计稿 .card —— 16px 圆角, 20px padding (默认), shadow-only 无边框。
 * soft = 14dp padding (card-tight), compact = 14x16 padding (card-compact)
 */
@Composable
fun PilottyCard(
    modifier: Modifier = Modifier,
    strong: Boolean = false,
    soft: Boolean = false,
    content: @Composable () -> Unit,
) {
    val pc = LocalPilottyColors.current
    // Dark 下 shadow 不可见, 改用极细 hairline 边框
    val elevation = when {
        pc.isDark -> 0.dp
        strong -> 6.dp
        soft -> 3.dp
        else -> 4.dp
    }
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(16.dp),
        color = pc.surface,
        tonalElevation = 0.dp,
        shadowElevation = elevation,
        border = if (pc.isDark) BorderStroke(1.dp, pc.hairline) else null,
    ) {
        content()
    }
}

/**
 * Card-group —— 多个 Row 垂直堆叠, 共享卡片外壳, 每行之间 hairline 分隔。
 * 用于 Settings / NodeDetail Config / Agent Log 等列表。
 */
@Composable
fun CardGroup(
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
    val pc = LocalPilottyColors.current
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(16.dp),
        color = pc.surface,
        tonalElevation = 0.dp,
        shadowElevation = if (pc.isDark) 0.dp else 4.dp,
        border = if (pc.isDark) BorderStroke(1.dp, pc.hairline) else null,
    ) {
        Column(content = content)
    }
}

/**
 * Kicker —— 全大写小号分区标题, 10sp + 1.6sp 字距。
 */
@Composable
fun Kicker(text: String, modifier: Modifier = Modifier, color: Color? = null) {
    val pc = LocalPilottyColors.current
    Text(
        text = text.uppercase(),
        modifier = modifier,
        color = color ?: pc.ink3,
        fontSize = 10.sp,
        fontWeight = FontWeight.Bold,
        letterSpacing = 1.6.sp,
    )
}

/**
 * SectionHead —— kicker 的放大版, 用于 Home / NodeDetail 分段头,
 * 11sp + 1.8sp letter-spacing + ink2 字色 (比 kicker 略深)
 */
@Composable
fun SectionHead(text: String, modifier: Modifier = Modifier) {
    val pc = LocalPilottyColors.current
    Text(
        text = text.uppercase(),
        modifier = modifier,
        color = pc.ink2,
        fontSize = 11.sp,
        fontWeight = FontWeight.Bold,
        letterSpacing = 1.98.sp,
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
        Box(
            modifier = Modifier
                .size((6 * scale).dp.coerceAtMost(16.dp))
                .clip(CircleShape)
                .background(pc.accent.copy(alpha = alpha)),
        )
        Box(
            modifier = Modifier
                .size(6.dp)
                .clip(CircleShape)
                .background(pc.accent),
        )
    }
}

/**
 * 静态圆点 (无动画)。
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
 * 国家代码胸牌 —— 2 字母大写, 28x20, 浅灰底色。
 */
@Composable
fun FlagChip(code: String, modifier: Modifier = Modifier) {
    val pc = LocalPilottyColors.current
    Box(
        modifier = modifier
            .size(width = 28.dp, height = 20.dp)
            .clip(RoundedCornerShape(5.dp))
            .background(pc.surface2),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = code.uppercase(),
            color = pc.monoInk,
            fontSize = 10.sp,
            fontWeight = FontWeight.Bold,
            letterSpacing = 0.4.sp,
            fontFamily = FontFamily.Monospace,
        )
    }
}

/**
 * 负载条 —— 固定宽 48dp, 4dp 高, 用于 Nodes 列表。
 */
@Composable
fun LoadBar(pct: Int, modifier: Modifier = Modifier, tone: String = "nominal") {
    val pc = LocalPilottyColors.current
    val color = when (tone) {
        "warn" -> pc.warn
        "alert" -> pc.error
        else -> pc.accent
    }
    Box(
        modifier = modifier
            .size(width = 48.dp, height = 4.dp)
            .clip(RoundedCornerShape(2.dp))
            .background(pc.surface2),
    ) {
        Box(
            modifier = Modifier
                .fillMaxHeight()
                .fillMaxWidth(pct.coerceIn(0, 100) / 100f)
                .background(color),
        )
    }
}

/**
 * Chip —— 32dp 高, 12 padding, 选中时黑底白字 (active), 非选中 surface + shadow。
 */
@Composable
fun PilottyChip(
    text: String,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    leading: (@Composable () -> Unit)? = null,
    height: Int = 32,
) {
    val pc = LocalPilottyColors.current
    val bg = if (selected) pc.ink else pc.surface
    val fg = if (selected) pc.bg else pc.ink
    Surface(
        modifier = modifier.height(height.dp),
        shape = RoundedCornerShape(999.dp),
        color = bg,
        shadowElevation = if (!selected && !pc.isDark) 3.dp else 0.dp,
        border = if (pc.isDark && !selected) BorderStroke(1.dp, pc.hairline) else null,
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
 * Button —— 6 种 variant (Outline/Accent 是旧名 alias, 保留向后兼容)。
 *   Primary: ink bg / bg fg, 48dp, 主 CTA
 *   Power:   accent bg / accentOnBg fg, 48dp, uppercase — 断开 / 连接
 *   Ghost:   outline border + ink 字, 40dp — 次要 action
 *   Mono:    surface2 bg + ink fg, 36dp — 工具栏 small CTA
 *   Outline: Ghost 旧名 (兼容)
 *   Accent:  Power 旧名 (兼容)
 */
enum class PilottyButtonVariant { Primary, Power, Ghost, Mono, Outline, Accent }

@Composable
fun PilottyButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    variant: PilottyButtonVariant = PilottyButtonVariant.Ghost,
    leading: (@Composable () -> Unit)? = null,
    enabled: Boolean = true,
    small: Boolean = false,
) {
    val pc = LocalPilottyColors.current
    // 归一化旧变体名
    val v = when (variant) {
        PilottyButtonVariant.Outline -> PilottyButtonVariant.Ghost
        PilottyButtonVariant.Accent -> PilottyButtonVariant.Power
        else -> variant
    }
    val (bg, fg, border, baseHeight, fontWeight) = when (v) {
        PilottyButtonVariant.Primary -> PilottyBtnSpec(pc.ink, pc.bg, null, 48.dp, FontWeight.SemiBold)
        PilottyButtonVariant.Power -> PilottyBtnSpec(pc.accent, pc.accentOnBg, null, 48.dp, FontWeight.Bold)
        PilottyButtonVariant.Ghost -> PilottyBtnSpec(Color.Transparent, pc.ink, BorderStroke(1.dp, pc.ink), 40.dp, FontWeight.Medium)
        PilottyButtonVariant.Mono -> PilottyBtnSpec(pc.surface2, pc.ink, null, 36.dp, FontWeight.Medium)
        else -> PilottyBtnSpec(Color.Transparent, pc.ink, BorderStroke(1.dp, pc.ink), 40.dp, FontWeight.Medium)
    }
    val height = if (small) (baseHeight - 8.dp).coerceAtLeast(28.dp) else baseHeight
    Surface(
        modifier = modifier.height(height),
        shape = RoundedCornerShape(999.dp),
        color = if (enabled) bg else bg.copy(alpha = 0.5f),
        border = border,
        onClick = onClick,
        enabled = enabled,
    ) {
        Row(
            modifier = Modifier.padding(horizontal = if (small) 12.dp else 16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp, Alignment.CenterHorizontally),
        ) {
            leading?.invoke()
            Text(
                text = text,
                color = if (enabled) fg else fg.copy(alpha = 0.6f),
                fontSize = if (small) 12.sp else 13.5.sp,
                fontWeight = fontWeight,
                letterSpacing = if (v == PilottyButtonVariant.Power) 0.4.sp else (-0.05).sp,
            )
        }
    }
}

private data class PilottyBtnSpec(
    val bg: Color,
    val fg: Color,
    val border: BorderStroke?,
    val height: androidx.compose.ui.unit.Dp,
    val fontWeight: FontWeight,
)

/**
 * Toggle —— 36x22 胶囊开关, on 时 accent 底, off 时 ink4 底。
 */
@Composable
fun PilottyToggle(
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
    scale: Float = 1f,
) {
    val pc = LocalPilottyColors.current
    val track = if (checked) pc.accent else pc.ink4
    val thumbBg = if (checked) pc.accentOnBg else Color.White
    Box(
        modifier = modifier
            .size(width = (36 * scale).dp, height = (22 * scale).dp)
            .clip(RoundedCornerShape((11 * scale).dp))
            .background(track)
            .clickable { onCheckedChange(!checked) }
            .padding((2 * scale).dp),
    ) {
        Box(
            modifier = Modifier
                .size((18 * scale).dp)
                .offset(x = if (checked) (16 * scale).dp else 0.dp)
                .clip(CircleShape)
                .background(thumbBg),
        )
    }
}
