package com.pilotty.app.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

/**
 * PilottyColorScheme — 暴露设计稿的三档灰 + accent + hairline,
 * 弥补 Material3 ColorScheme 的槽位不足。业务代码用
 * `LocalPilottyColors.current` 读取。
 */
data class PilottyColorScheme(
    val bg: Color,
    val surface: Color,
    val surface2: Color,
    val surface3: Color,
    val ink: Color,
    val ink2: Color,
    val ink3: Color,
    val ink4: Color,
    val ink5: Color,
    val hairline: Color,
    val hairlineStrong: Color,
    val navBg: Color,
    val accent: Color,
    val accentInk: Color,
    val accentOnBg: Color,
    val error: Color,
    val isDark: Boolean,
)

val LocalPilottyColors = staticCompositionLocalOf<PilottyColorScheme> {
    error("PilottyTheme not installed")
}

private fun buildLight() = PilottyColorScheme(
    bg = PilottyColors.Light.Bg,
    surface = PilottyColors.Light.Surface,
    surface2 = PilottyColors.Light.Surface2,
    surface3 = PilottyColors.Light.Surface3,
    ink = PilottyColors.Light.Ink,
    ink2 = PilottyColors.Light.Ink2,
    ink3 = PilottyColors.Light.Ink3,
    ink4 = PilottyColors.Light.Ink4,
    ink5 = PilottyColors.Light.Ink5,
    hairline = PilottyColors.Light.Hairline,
    hairlineStrong = PilottyColors.Light.HairlineStrong,
    navBg = PilottyColors.Light.NavBg,
    accent = PilottyColors.Accent,
    accentInk = PilottyColors.AccentInk,
    accentOnBg = PilottyColors.AccentOnBg,
    error = PilottyColors.Error,
    isDark = false,
)

private fun buildDark() = PilottyColorScheme(
    bg = PilottyColors.Dark.Bg,
    surface = PilottyColors.Dark.Surface,
    surface2 = PilottyColors.Dark.Surface2,
    surface3 = PilottyColors.Dark.Surface3,
    ink = PilottyColors.Dark.Ink,
    ink2 = PilottyColors.Dark.Ink2,
    ink3 = PilottyColors.Dark.Ink3,
    ink4 = PilottyColors.Dark.Ink4,
    ink5 = PilottyColors.Dark.Ink5,
    hairline = PilottyColors.Dark.Hairline,
    hairlineStrong = PilottyColors.Dark.HairlineStrong,
    navBg = PilottyColors.Dark.NavBg,
    accent = PilottyColors.Accent,
    accentInk = PilottyColors.Accent,
    accentOnBg = PilottyColors.AccentOnBg,
    error = PilottyColors.Error,
    isDark = true,
)

private val AppTypography = Typography(
    // JetBrains Mono 仅在个别位置 (kicker / mono 数值) 用, 不替换默认字体
    labelSmall = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.SemiBold,
        fontSize = 10.sp,
        letterSpacing = 1.4.sp,
    ),
    bodyMedium = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.Normal,
        fontSize = 14.sp,
        lineHeight = 21.sp,
    ),
    titleMedium = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.SemiBold,
        fontSize = 15.sp,
        letterSpacing = (-0.15).sp,
    ),
    titleLarge = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.SemiBold,
        fontSize = 20.sp,
        letterSpacing = (-0.3).sp,
    ),
)

@Composable
fun PilottyTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit,
) {
    val pc = if (darkTheme) buildDark() else buildLight()
    // 给 Material3 的 ColorScheme 填上最小子集, 避免 ripple / 系统组件默认色怪异
    val m3 = if (darkTheme) {
        darkColorScheme(
            primary = pc.accent,
            onPrimary = pc.accentOnBg,
            background = pc.bg,
            onBackground = pc.ink,
            surface = pc.surface,
            onSurface = pc.ink,
            surfaceVariant = pc.surface2,
            onSurfaceVariant = pc.ink2,
            error = pc.error,
        )
    } else {
        lightColorScheme(
            primary = pc.accent,
            onPrimary = pc.accentOnBg,
            background = pc.bg,
            onBackground = pc.ink,
            surface = pc.surface,
            onSurface = pc.ink,
            surfaceVariant = pc.surface2,
            onSurfaceVariant = pc.ink2,
            error = pc.error,
        )
    }
    CompositionLocalProvider(LocalPilottyColors provides pc) {
        MaterialTheme(colorScheme = m3, typography = AppTypography, content = content)
    }
}
