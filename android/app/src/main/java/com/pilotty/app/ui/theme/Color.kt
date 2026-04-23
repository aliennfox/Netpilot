package com.pilotty.app.ui.theme

import androidx.compose.ui.graphics.Color

/**
 * Color tokens 对齐 Mission Control 设计稿 styles.css v3。
 *   Light: Bg=#FAFAFA 底 / Surface=#FFF 卡 (cards 比 bg 更白, 实现"抬升"感)
 *   Dark : Bg=#000 / Surface=#141414 (pure black, lemon pops)
 * 3 色语义: Accent (lemon) / Warn (amber) / Error (alert red)
 */
object PilottyColors {
    // 品牌语义 3 色 — meaning only, no decoration
    val Accent = Color(0xFFC7F860)        // nominal / active
    val AccentInk = Color(0xFF6F8F1E)     // 浅色 bg 上的 lemon 文字 (contrast 兜底)
    val AccentOnBg = Color(0xFF1A2300)    // 柠檬绿 bg 上的文字
    val Warn = Color(0xFFF5A623)          // caution / degraded
    val Error = Color(0xFFE5484D)         // critical / alert

    // Light — 抬升感 = cards 比 bg 更白
    object Light {
        val Bg = Color(0xFFFAFAFA)        // stage 底
        val Surface = Color(0xFFFFFFFF)   // cards
        val Surface2 = Color(0xFFF4F4F5)  // 内嵌次层 (chip bg / input bg)
        val Surface3 = Color(0xFFEDEDEF)  // 更深次层 (展开的 payload bg)
        val Stripe = Color(0xFFF7F7F8)    // agent log 斑马条 B 行
        val Ink = Color(0xFF0A0A0A)       // 主文
        val Ink2 = Color(0xFF6B6B6B)      // 次文 / 副标
        val Ink3 = Color(0xFF9B9B9B)      // meta / mono 色
        val Ink4 = Color(0xFFBDBDBD)      // 占位 / disabled
        val Ink5 = Color(0xFFD4D4D8)      // 极浅
        val MonoInk = Color(0xFF2D2D2D)   // mono 字专用色
        val Hairline = Color(0xFFE8E8E8)  // divider
        val HairlineStrong = Color(0xFFD7D7DA)
        val NavBg = Color(0xFFFAFAFA)     // 底 nav 跟 bg 同色, 只用一条 divider 区分
    }

    // Dark — pure black + 柠檬绿 pops
    object Dark {
        val Bg = Color(0xFF000000)
        val Surface = Color(0xFF141414)   // cards 比 bg 亮
        val Surface2 = Color(0xFF1C1C1C)  // chip / input
        val Surface3 = Color(0xFF222225)  // payload bg
        val Stripe = Color(0xFF181818)
        val Ink = Color(0xFFFFFFFF)
        val Ink2 = Color(0xFFA0A0A0)
        val Ink3 = Color(0xFF6B6B6B)
        val Ink4 = Color(0xFF4A4A4A)
        val Ink5 = Color(0xFF2A2A2A)
        val MonoInk = Color(0xFFD0D0D0)
        val Hairline = Color(0xFF2A2A2A)
        val HairlineStrong = Color(0xFF3A3A3A)
        val NavBg = Color(0xFF000000)
    }
}
