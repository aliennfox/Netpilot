package com.pilotty.app.ui.settings

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.ui.RulesSection
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.components.PilottyChip
import com.pilotty.app.ui.theme.LocalPilottyColors

/**
 * Settings 顶层 —— 承载之前独立 tab 的 "规则" (改为内联 section) + 主题切换 + LLM / 关于占位。
 */
@Composable
fun SettingsScreen(
    themeMode: ThemeMode,
    onThemeChange: (ThemeMode) -> Unit,
) {
    val pc = LocalPilottyColors.current
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(pc.bg)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 20.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Spacer(Modifier.height(8.dp))
        Text(
            "Settings",
            color = pc.ink,
            fontSize = 20.sp,
            fontWeight = FontWeight.SemiBold,
            letterSpacing = (-0.3).sp,
        )
        Text(
            "主题 · 分流规则 · 账户",
            color = pc.ink3,
            fontSize = 11.sp,
            fontFamily = FontFamily.Monospace,
            letterSpacing = 0.2.sp,
        )

        // 主题切换
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Kicker("主题")
            PilottyCard(modifier = Modifier.fillMaxWidth()) {
                Row(
                    modifier = Modifier.padding(14.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    PilottyChip(
                        text = "跟随系统",
                        selected = themeMode == ThemeMode.System,
                        onClick = { onThemeChange(ThemeMode.System) },
                    )
                    PilottyChip(
                        text = "浅色",
                        selected = themeMode == ThemeMode.Light,
                        onClick = { onThemeChange(ThemeMode.Light) },
                    )
                    PilottyChip(
                        text = "深色",
                        selected = themeMode == ThemeMode.Dark,
                        onClick = { onThemeChange(ThemeMode.Dark) },
                    )
                }
            }
        }

        // LLM 占位 (M8 做完后替换)
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Kicker("Agent · LLM")
            PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
                Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text("DeepSeek via 硅基流动", color = pc.ink, fontSize = 14.sp, fontWeight = FontWeight.Medium)
                    Text(
                        "API Key 尚未配置。Chat tab 需此 key 才能启用 Agent。",
                        color = pc.ink3,
                        fontSize = 12.sp,
                    )
                    Text(
                        "TODO M8: EncryptedSharedPreferences 存取入口",
                        color = pc.ink4,
                        fontSize = 11.sp,
                        fontFamily = FontFamily.Monospace,
                    )
                }
            }
        }

        // 分流规则 (原 Rules tab 内联) —— RulesSection 内部已有 Templates / Active rules
        // 两个 Kicker 作为子标题,不再在外层重复"分流规则" 标题
        RulesSection()

        // 关于
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Kicker("About")
            PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
                Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text("Pilotty", color = pc.ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
                    Text(
                        "版本 0.1.0 · sing-box + gomobile",
                        color = pc.ink3,
                        fontSize = 11.sp,
                        fontFamily = FontFamily.Monospace,
                    )
                }
            }
        }

        Spacer(Modifier.height(96.dp)) // floating nav 留白
    }
}

enum class ThemeMode { System, Light, Dark }
