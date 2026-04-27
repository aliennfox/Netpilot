package com.pilotty.app.ui.theme

import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.SpringSpec
import androidx.compose.animation.core.TweenSpec
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import android.os.Build
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
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

// M3 完整 13-slot Typography scale (https://m3.material.io/styles/typography/type-scale-tokens)
// Pilotty 选 SansSerif + 紧字距 + 不浮夸字号, 保留品牌"工程感"调性。
// 业务代码继续可以 inline TextStyle, 但新组件应优先 MaterialTheme.typography.<slot>.
private val AppTypography = Typography(
    // Display — 仅用于 splash / huge headline; 未来 hero scenes
    displayLarge = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.Bold,
        fontSize = 57.sp, lineHeight = 64.sp, letterSpacing = (-0.25).sp,
    ),
    displayMedium = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.Bold,
        fontSize = 45.sp, lineHeight = 52.sp, letterSpacing = 0.sp,
    ),
    displaySmall = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.Bold,
        fontSize = 36.sp, lineHeight = 44.sp, letterSpacing = 0.sp,
    ),
    // Headline — section 大标题
    headlineLarge = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.SemiBold,
        fontSize = 32.sp, lineHeight = 40.sp, letterSpacing = 0.sp,
    ),
    headlineMedium = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.SemiBold,
        fontSize = 28.sp, lineHeight = 36.sp, letterSpacing = 0.sp,
    ),
    headlineSmall = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.SemiBold,
        fontSize = 24.sp, lineHeight = 32.sp, letterSpacing = 0.sp,
    ),
    // Title — 卡片 / 行标题
    titleLarge = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.Bold,
        fontSize = 22.sp, lineHeight = 28.sp, letterSpacing = (-0.44).sp,
    ),
    titleMedium = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.SemiBold,
        fontSize = 15.sp, lineHeight = 24.sp, letterSpacing = (-0.15).sp,
    ),
    titleSmall = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.SemiBold,
        fontSize = 13.sp, lineHeight = 20.sp, letterSpacing = 0.1.sp,
    ),
    // Body — 主要内容
    bodyLarge = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.Normal,
        fontSize = 16.sp, lineHeight = 24.sp, letterSpacing = 0.5.sp,
    ),
    bodyMedium = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.Normal,
        fontSize = 14.sp, lineHeight = 21.sp, letterSpacing = 0.25.sp,
    ),
    bodySmall = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.Normal,
        fontSize = 12.sp, lineHeight = 16.sp, letterSpacing = 0.4.sp,
    ),
    // Label — 按钮 / chip / overline
    labelLarge = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.SemiBold,
        fontSize = 14.sp, lineHeight = 20.sp, letterSpacing = 0.1.sp,
    ),
    labelMedium = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.SemiBold,
        fontSize = 12.sp, lineHeight = 16.sp, letterSpacing = 0.5.sp,
    ),
    labelSmall = TextStyle(
        fontFamily = FontFamily.SansSerif, fontWeight = FontWeight.SemiBold,
        fontSize = 10.sp, lineHeight = 14.sp, letterSpacing = 1.6.sp,
    ),
)

// M3 Shapes 系统 (https://m3.material.io/styles/shape/shape-scale-tokens)
// Pilotty 沿用偏圆调性: extraSmall/Small 用 chip / 小标签;
// medium 用普通 Card; large 用 prominent surface; extraLarge 接近 pill.
// MaterialTheme 把这些自动透给 Card/Button/MenuItem 等, 业务代码新组件
// 应用 MaterialTheme.shapes.<size> 而不是 RoundedCornerShape inline.
private val AppShapes = Shapes(
    extraSmall = RoundedCornerShape(4.dp),
    small = RoundedCornerShape(8.dp),
    medium = RoundedCornerShape(12.dp),
    large = RoundedCornerShape(16.dp),
    extraLarge = RoundedCornerShape(28.dp),
)

// M3 Motion 推荐 spec (https://m3.material.io/styles/motion/easing-and-duration/applying-easing-and-duration)
// 暴露给业务代码作 animateFloatAsState / AnimatedVisibility 等的默认 animationSpec.
// 不用 staticCompositionLocalOf — 这些是纯函数 spec, 全局共用。
object PilottyMotion {
    // Standard — 通用 UI 状态变化 (按下回弹, fade, expand)
    val springStandard: SpringSpec<Float> = spring(
        dampingRatio = Spring.DampingRatioNoBouncy,
        stiffness = Spring.StiffnessMediumLow,
    )
    // Emphasized — 重要状态切换 (页面进出, hero animation)
    val springEmphasized: SpringSpec<Float> = spring(
        dampingRatio = Spring.DampingRatioLowBouncy,
        stiffness = Spring.StiffnessMedium,
    )
    // Tween — 当时长可预测/必须线性时 (progress / 节流脉冲)
    val tweenShort: TweenSpec<Float> = tween(durationMillis = 200)
    val tweenMedium: TweenSpec<Float> = tween(durationMillis = 350)
    val tweenLong: TweenSpec<Float> = tween(durationMillis = 500)
}

/**
 * PilottyTheme — Material You dynamic color (Android 12+) + brand fallback (≤Android 11).
 *
 * Android 12+ 下颜色跟随系统壁纸自动派生,颜色 token (accent / surface / ink) 全部从
 * `dynamicDarkColorScheme` / `dynamicLightColorScheme` 拉, 萤光绿不再硬写。
 * Android 11- 走 buildDark()/buildLight() 保留经典萤光绿品牌作 fallback。
 *
 * 业务代码继续通过 `LocalPilottyColors.current` 拿 token —— 屏蔽 Material You 与 brand
 * fallback 的差异, 只看 PilottyColorScheme 这层抽象。
 */
@Composable
fun PilottyTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit,
) {
    val context = LocalContext.current
    val supportsDynamic = Build.VERSION.SDK_INT >= Build.VERSION_CODES.S

    val m3 = when {
        supportsDynamic && darkTheme -> dynamicDarkColorScheme(context)
        supportsDynamic && !darkTheme -> dynamicLightColorScheme(context)
        darkTheme -> {
            val brand = buildDark()
            darkColorScheme(
                primary = brand.accent, onPrimary = brand.accentOnBg,
                background = brand.bg, onBackground = brand.ink,
                surface = brand.surface, onSurface = brand.ink,
                surfaceVariant = brand.surface2, onSurfaceVariant = brand.ink2,
                error = brand.error,
            )
        }
        else -> {
            val brand = buildLight()
            lightColorScheme(
                primary = brand.accent, onPrimary = brand.accentOnBg,
                background = brand.bg, onBackground = brand.ink,
                surface = brand.surface, onSurface = brand.ink,
                surfaceVariant = brand.surface2, onSurfaceVariant = brand.ink2,
                error = brand.error,
            )
        }
    }

    // PilottyColorScheme 层: dynamic 时从 m3.colorScheme 派生, fallback 时用品牌色
    val pc = if (supportsDynamic) {
        // 从 dynamic colorScheme 派生 PilottyColorScheme 各 slot. ink/surface 跟随壁纸,
        // hairline 等"工程感"灰 token 仍然用品牌固定值 (壁纸派生的灰太软, 卡片边界看不清).
        val brand = if (darkTheme) buildDark() else buildLight()
        brand.copy(
            accent = m3.primary,
            accentInk = m3.onPrimary,
            accentOnBg = m3.primary,
            bg = m3.background,
            surface = m3.surface,
            surface2 = m3.surfaceVariant,
            ink = m3.onSurface,
            ink2 = m3.onSurfaceVariant,
            error = m3.error,
        )
    } else if (darkTheme) buildDark() else buildLight()

    CompositionLocalProvider(LocalPilottyColors provides pc) {
        MaterialTheme(
            colorScheme = m3,
            typography = AppTypography,
            shapes = AppShapes,
            content = content,
        )
    }
}
