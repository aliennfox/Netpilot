package com.pilotty.app.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.ui.theme.LocalPilottyColors

/**
 * Latency 环 —— 圆形进度 + 中心 ms 数字 + MS 小字。
 * 阈值: <100 nominal(绿) / <300 warn(黄) / >=300 alert(红); fill = ms/400 上限。
 */
@Composable
fun LatencyRing(
    ms: Int,
    modifier: Modifier = Modifier,
    size: Int = 64,
    stroke: Int = 5,
) {
    val pc = LocalPilottyColors.current
    val tier = when {
        ms < 100 -> "nominal"
        ms < 300 -> "warn"
        else -> "alert"
    }
    val color = when (tier) {
        "nominal" -> pc.accent
        "warn" -> pc.warn
        else -> pc.error
    }
    val textColor = if (tier == "nominal") pc.ink else color
    val pct = (ms / 400f).coerceAtMost(1f)
    Box(
        modifier = modifier.size(size.dp),
        contentAlignment = Alignment.Center,
    ) {
        Canvas(modifier = Modifier.fillMaxSize()) {
            val sw = stroke.dp.toPx()
            val diameter = this.size.minDimension - sw
            val topLeft = Offset((this.size.width - diameter) / 2f, (this.size.height - diameter) / 2f)
            val arcSize = Size(diameter, diameter)
            drawArc(
                color = pc.hairline,
                startAngle = 0f,
                sweepAngle = 360f,
                useCenter = false,
                topLeft = topLeft,
                size = arcSize,
                style = Stroke(width = sw),
            )
            drawArc(
                color = color,
                startAngle = -90f,
                sweepAngle = 360f * pct,
                useCenter = false,
                topLeft = topLeft,
                size = arcSize,
                style = Stroke(width = sw, cap = StrokeCap.Round),
            )
        }
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(
                text = ms.toString(),
                color = textColor,
                fontSize = (size * 0.26f).sp,
                fontWeight = FontWeight.Bold,
                letterSpacing = (-0.4).sp,
                fontFamily = FontFamily.Monospace,
            )
            Text(
                text = "MS",
                color = pc.ink3,
                fontSize = 9.sp,
                letterSpacing = 0.8.sp,
                fontWeight = FontWeight.Medium,
            )
        }
    }
}

/**
 * 折线 Sparkline —— 纯 Canvas, 数据长度 >=2 才画。
 */
@Composable
fun Sparkline(
    data: List<Float>,
    modifier: Modifier = Modifier,
    color: Color,
    strokeWidth: Float = 1.5f,
) {
    if (data.size < 2) return
    Canvas(modifier = modifier) {
        val min = data.min()
        val max = data.max()
        val range = (max - min).takeIf { it > 0f } ?: 1f
        val step = size.width / (data.size - 1)
        val path = Path()
        data.forEachIndexed { i, v ->
            val x = i * step
            val y = size.height - ((v - min) / range) * size.height
            if (i == 0) path.moveTo(x, y) else path.lineTo(x, y)
        }
        drawPath(
            path = path,
            color = color,
            style = Stroke(width = strokeWidth.dp.toPx(), cap = StrokeCap.Round),
        )
    }
}

/**
 * 双线叠加 Sparkline — up (粗, ink) + down (细, ink3)
 */
@Composable
fun SparklineDual(
    up: List<Float>,
    down: List<Float>,
    modifier: Modifier = Modifier,
) {
    val pc = LocalPilottyColors.current
    Box(modifier = modifier) {
        Sparkline(
            data = up,
            modifier = Modifier.fillMaxSize(),
            color = pc.ink,
            strokeWidth = 1.4f,
        )
        Sparkline(
            data = down,
            modifier = Modifier.fillMaxSize(),
            color = pc.ink3,
            strokeWidth = 1.2f,
        )
    }
}

/**
 * Mini bar chart — 数据归一到 height, 等宽柱
 */
@Composable
fun BarMini(
    data: List<Float>,
    modifier: Modifier = Modifier,
    color: Color,
) {
    Canvas(modifier = modifier) {
        val max = data.maxOrNull() ?: 1f
        val gap = 1.5f.dp.toPx()
        val bw = (size.width / data.size) - gap
        data.forEachIndexed { i, v ->
            val h = (v / max) * size.height
            drawRect(
                color = color,
                topLeft = Offset(i * (bw + gap), size.height - h),
                size = Size(bw.coerceAtLeast(1f), h.coerceAtLeast(1f)),
            )
        }
    }
}

/**
 * 横向进度条 —— pct 0..100, 阈值自动配色 (>85 alert / >60 warn / else nominal)
 */
@Composable
fun Progress(
    pct: Float,
    modifier: Modifier = Modifier,
    height: Int = 6,
    tone: String? = null,
) {
    val pc = LocalPilottyColors.current
    val auto = when {
        pct > 85f -> "alert"
        pct > 60f -> "warn"
        else -> "nominal"
    }
    val t = tone ?: auto
    val color = when (t) {
        "alert" -> pc.error
        "warn" -> pc.warn
        else -> pc.accent
    }
    Box(
        modifier = modifier
            .height(height.dp)
            .fillMaxWidth()
            .clip(RoundedCornerShape((height / 2).dp))
            .background(pc.surface2),
    ) {
        Box(
            modifier = Modifier
                .fillMaxHeight()
                .fillMaxWidth(pct.coerceIn(0f, 100f) / 100f)
                .background(color),
        )
    }
}

/**
 * TraceDot —— ok / err / running / pending, 形状+颜色双编码
 */
@Composable
fun TraceDot(state: String, modifier: Modifier = Modifier) {
    val pc = LocalPilottyColors.current
    val size = 14.dp
    when (state) {
        "ok" -> Box(
            modifier = modifier
                .size(size)
                .clip(CircleShape)
                .background(pc.accent),
            contentAlignment = Alignment.Center,
        ) {
            Canvas(Modifier.size(9.dp)) {
                val s = this.size.minDimension
                val path = Path().apply {
                    moveTo(s * 0.2f, s * 0.55f)
                    lineTo(s * 0.45f, s * 0.78f)
                    lineTo(s * 0.82f, s * 0.25f)
                }
                drawPath(
                    path = path,
                    color = pc.accentOnBg,
                    style = Stroke(width = 1.8.dp.toPx(), cap = StrokeCap.Round),
                )
            }
        }
        "err" -> Box(
            modifier = modifier
                .size(size)
                .clip(CircleShape)
                .background(pc.error),
            contentAlignment = Alignment.Center,
        ) {
            Canvas(Modifier.size(9.dp)) {
                val s = this.size.minDimension
                drawLine(
                    color = Color.White,
                    start = Offset(s * 0.2f, s * 0.2f),
                    end = Offset(s * 0.8f, s * 0.8f),
                    strokeWidth = 1.8.dp.toPx(),
                    cap = StrokeCap.Round,
                )
                drawLine(
                    color = Color.White,
                    start = Offset(s * 0.8f, s * 0.2f),
                    end = Offset(s * 0.2f, s * 0.8f),
                    strokeWidth = 1.8.dp.toPx(),
                    cap = StrokeCap.Round,
                )
            }
        }
        "running" -> Box(
            modifier = modifier
                .size(size)
                .clip(CircleShape)
                .background(Color.Transparent),
            contentAlignment = Alignment.Center,
        ) {
            Canvas(Modifier.fillMaxSize()) {
                val sw = 1.5.dp.toPx()
                drawCircle(
                    color = pc.accent,
                    radius = (this.size.minDimension - sw) / 2f,
                    style = Stroke(width = sw),
                )
            }
            Box(
                Modifier
                    .size(5.dp)
                    .clip(CircleShape)
                    .background(pc.accent),
            )
        }
        else -> Box(
            modifier = modifier
                .size(size)
                .clip(CircleShape)
                .background(Color.Transparent),
        ) {
            Canvas(Modifier.fillMaxSize()) {
                val sw = 1.5.dp.toPx()
                drawCircle(
                    color = pc.ink4,
                    radius = (this.size.minDimension - sw) / 2f,
                    style = Stroke(width = sw),
                )
            }
        }
    }
}

/**
 * Protocol badge — VLESS / Trojan / Shadowsocks 等
 */
@Composable
fun ProtoBadge(name: String, modifier: Modifier = Modifier) {
    val pc = LocalPilottyColors.current
    Box(
        modifier = modifier
            .height(18.dp)
            .clip(RoundedCornerShape(3.dp))
            .background(pc.surface2)
            .padding(horizontal = 5.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = name.uppercase(),
            color = pc.monoInk,
            fontSize = 9.5.sp,
            fontWeight = FontWeight.Bold,
            letterSpacing = 0.8.sp,
            fontFamily = FontFamily.Monospace,
        )
    }
}

/**
 * 状态 dot (无动画) — nominal / warn / alert 三色
 */
@Composable
fun StatusDot(state: String, modifier: Modifier = Modifier) {
    val pc = LocalPilottyColors.current
    val color = when (state) {
        "warn" -> pc.warn
        "alert" -> pc.error
        else -> pc.accent
    }
    Box(
        modifier = modifier
            .size(6.dp)
            .clip(CircleShape)
            .background(color),
    )
}
