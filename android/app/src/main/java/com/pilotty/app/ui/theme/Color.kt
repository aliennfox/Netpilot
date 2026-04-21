package com.pilotty.app.ui.theme

import androidx.compose.ui.graphics.Color

/**
 * Color tokens 对齐设计稿 styles.css。
 * Light = 三档灰 + 柠檬绿 accent;Dark = pure black + 柠檬绿 pops。
 */
object PilottyColors {
    // 品牌 accent — 柠檬绿带黄调, 非霓虹
    val Accent = Color(0xFFC7F860)
    val AccentInk = Color(0xFF6F8F1E)      // 浅色 bg 上的文字用这个 darker variant
    val AccentOnBg = Color(0xFF1A2300)     // 柠檬绿 bg 上的文字

    val Error = Color(0xFFE5484D)

    // Light
    object Light {
        val Bg = Color(0xFFFFFFFF)
        val Surface = Color(0xFFFAFAFA)
        val Surface2 = Color(0xFFF7F7F8)
        val Surface3 = Color(0xFFF0F0F2)
        val Ink = Color(0xFF0A0A0B)
        val Ink2 = Color(0xFF3F3F46)
        val Ink3 = Color(0xFF71717A)
        val Ink4 = Color(0xFFA1A1AA)
        val Ink5 = Color(0xFFD4D4D8)
        val Hairline = Color(0xFFEDEDEF)
        val HairlineStrong = Color(0xFFD7D7DA)
        val NavBg = Color(0xC8FFFFFF)      // rgba(255,255,255,0.78)
    }

    // Dark — pure black, lemon pops
    object Dark {
        val Bg = Color(0xFF000000)
        val Surface = Color(0xFF0D0D0D)
        val Surface2 = Color(0xFF141414)
        val Surface3 = Color(0xFF1C1C1C)
        val Ink = Color(0xFFFFFFFF)
        val Ink2 = Color(0xFFD4D4D4)
        val Ink3 = Color(0xFFA3A3A3)
        val Ink4 = Color(0xFF737373)
        val Ink5 = Color(0xFF404040)
        val Hairline = Color(0xFF1F1F1F)
        val HairlineStrong = Color(0xFF2A2A2A)
        val NavBg = Color(0xB8141414)      // rgba(20,20,20,0.72)
    }
}
