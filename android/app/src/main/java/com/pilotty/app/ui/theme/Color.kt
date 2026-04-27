package com.pilotty.app.ui.theme

import androidx.compose.ui.graphics.Color

/**
 * Color tokens 对齐 Mission Control 设计稿 styles.css v3。
 *   Light: Bg=#FAFAFA 底 / Surface=#FFF 卡 (cards 比 bg 更白, 实现"抬升"感)
 *   Dark : Bg=#000 / Surface=#141414
 * 3 色语义: Accent (royal blue) / Warn (amber) / Error (alert red)
 */
object PilottyColors {
    // 品牌语义 3 色 — meaning only, no decoration
    // Accent 改 royal blue: 替代原荧光柠檬绿, 工程感/控制台调性更贴 Pilotty 命名,
    // 且与白文字对比度足 (~5.2:1 AA pass for normal text), 与黑底对比度 ~6.0:1 AA pass.
    val Accent = Color(0xFF2563EB)        // nominal / active
    val AccentInk = Color(0xFF1D4ED8)     // 浅色 bg 上的品牌色文字 (深一档)
    val AccentOnBg = Color(0xFFFFFFFF)    // accent bg 上的文字 (高对比白)
    val Warn = Color(0xFFF5A623)          // caution / degraded
    val Error = Color(0xFFE5484D)         // critical / alert

    // Light — 抬升感 = cards 比 bg 更白
    // ink3/ink4 较 v3 提亮一档, 解决浅底上 meta/placeholder 文字对比度不足 (#9B9B9B 在 #FAFAFA 上仅 2.6:1, AA fail)
    object Light {
        val Bg = Color(0xFFFAFAFA)        // stage 底
        val Surface = Color(0xFFFFFFFF)   // cards
        val Surface2 = Color(0xFFF4F4F5)  // 内嵌次层 (chip bg / input bg)
        val Surface3 = Color(0xFFEDEDEF)  // 更深次层 (展开的 payload bg)
        val Stripe = Color(0xFFF7F7F8)    // agent log 斑马条 B 行
        val Ink = Color(0xFF0A0A0A)       // 主文
        val Ink2 = Color(0xFF555555)      // 次文 / 副标 (原 #6B6B6B → 提亮)
        val Ink3 = Color(0xFF7A7A7A)      // meta / mono 色 (原 #9B9B9B → AA 通过 4.7:1)
        val Ink4 = Color(0xFF969696)      // 占位 / disabled (原 #BDBDBD → 3.4:1)
        val Ink5 = Color(0xFFC5C5C8)      // 极浅
        val MonoInk = Color(0xFF2D2D2D)   // mono 字专用色
        val Hairline = Color(0xFFE8E8E8)  // divider
        val HairlineStrong = Color(0xFFD7D7DA)
        val NavBg = Color(0xFFFAFAFA)     // 底 nav 跟 bg 同色, 只用一条 divider 区分
    }

    // Dark — pure black + accent pops
    // ink3/ink4 较 v3 提亮 (#6B6B6B → #8E8E8E 等), 黑底上次级文字 contrast 5.7:1 AA 通过
    object Dark {
        val Bg = Color(0xFF000000)
        val Surface = Color(0xFF141414)   // cards 比 bg 亮
        val Surface2 = Color(0xFF1C1C1C)  // chip / input
        val Surface3 = Color(0xFF222225)  // payload bg
        val Stripe = Color(0xFF181818)
        val Ink = Color(0xFFFFFFFF)
        val Ink2 = Color(0xFFB8B8B8)      // 次文 (原 #A0A0A0 → 提亮)
        val Ink3 = Color(0xFF8E8E8E)      // meta (原 #6B6B6B → 5.7:1 AA pass)
        val Ink4 = Color(0xFF6B6B6B)      // 占位 (原 #4A4A4A → 3.7:1)
        val Ink5 = Color(0xFF3A3A3A)      // 极浅 / disabled
        val MonoInk = Color(0xFFD0D0D0)
        val Hairline = Color(0xFF2A2A2A)
        val HairlineStrong = Color(0xFF3A3A3A)
        val NavBg = Color(0xFF000000)
    }
}
