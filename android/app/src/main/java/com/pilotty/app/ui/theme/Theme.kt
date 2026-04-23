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
 * PilottyColorScheme — Mission Control 设计系统的完整 token 槽位。
 * 业务代码通过 `LocalPilottyColors.current` 读取, 不走 Material3 ColorScheme 的兜底。
 */
data class PilottyColorScheme(
    val bg: Color,
    val surface: Color,
    val surface2: Color,
    val surface3: Color,
    val stripe: Color,
    val ink: Color,
    val ink2: Color,
    val ink3: Color,
    val ink4: Color,
    val ink5: Color,
    val monoInk: Color,
    val hairline: Color,
    val hairlineStrong: Color,
    val navBg: Color,
    val accent: Color,
    val accentInk: Color,
    val accentOnBg: Color,
    val warn: Color,
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
    stripe = PilottyColors.Light.Stripe,
    ink = PilottyColors.Light.Ink,
    ink2 = PilottyColors.Light.Ink2,
    ink3 = PilottyColors.Light.Ink3,
    ink4 = PilottyColors.Light.Ink4,
    ink5 = PilottyColors.Light.Ink5,
    monoInk = PilottyColors.Light.MonoInk,
    hairline = PilottyColors.Light.Hairline,
    hairlineStrong = PilottyColors.Light.HairlineStrong,
    navBg = PilottyColors.Light.NavBg,
    accent = PilottyColors.Accent,
    accentInk = PilottyColors.AccentInk,
    accentOnBg = PilottyColors.AccentOnBg,
    warn = PilottyColors.Warn,
    error = PilottyColors.Error,
    isDark = false,
)

private fun buildDark() = PilottyColorScheme(
    bg = PilottyColors.Dark.Bg,
    surface = PilottyColors.Dark.Surface,
    surface2 = PilottyColors.Dark.Surface2,
    surface3 = PilottyColors.Dark.Surface3,
    stripe = PilottyColors.Dark.Stripe,
    ink = PilottyColors.Dark.Ink,
    ink2 = PilottyColors.Dark.Ink2,
    ink3 = PilottyColors.Dark.Ink3,
    ink4 = PilottyColors.Dark.Ink4,
    ink5 = PilottyColors.Dark.Ink5,
    monoInk = PilottyColors.Dark.MonoInk,
    hairline = PilottyColors.Dark.Hairline,
    hairlineStrong = PilottyColors.Dark.HairlineStrong,
    navBg = PilottyColors.Dark.NavBg,
    accent = PilottyColors.Accent,
    accentInk = PilottyColors.Accent,
    accentOnBg = PilottyColors.AccentOnBg,
    warn = PilottyColors.Warn,
    error = PilottyColors.Error,
    isDark = true,
)

private val AppTypography = Typography(
    labelSmall = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.SemiBold,
        fontSize = 10.sp,
        letterSpacing = 1.6.sp,
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
        fontWeight = FontWeight.Bold,
        fontSize = 22.sp,
        letterSpacing = (-0.44).sp,
    ),
)

@Composable
fun PilottyTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit,
) {
    val pc = if (darkTheme) buildDark() else buildLight()
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
