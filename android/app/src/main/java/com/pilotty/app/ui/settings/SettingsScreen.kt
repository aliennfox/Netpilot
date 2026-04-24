package com.pilotty.app.ui.settings

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.foundation.layout.imePadding
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.ui.RulesSection
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.SectionHead
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
    onNavigateLogs: () -> Unit = {},
    onNavigateConnections: () -> Unit = {},
    onNavigateAgentTools: () -> Unit = {},
) {
    val pc = LocalPilottyColors.current
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(pc.bg)
            .verticalScroll(rememberScrollState())
            // imePadding: IME 打开时列缩到 (屏高 - IME 高), 配合末尾 96dp floating nav 留白,
            // 用户可滚到 AgentApiKey 保存/取消按钮 (Phase 4 修复, 原先被 IME + nav 双重遮挡)
            .imePadding()
            .padding(horizontal = 20.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Spacer(Modifier.height(8.dp))
        Text(
            stringResource(com.pilotty.app.R.string.settings_title),
            color = pc.ink,
            fontSize = 22.sp,
            fontWeight = FontWeight.Bold,
            letterSpacing = (-0.44).sp,
        )
        Text(
            stringResource(com.pilotty.app.R.string.settings_subtitle_format, "1.4.2"),
            color = pc.ink3,
            fontSize = 10.5.sp,
            fontFamily = FontFamily.Monospace,
            letterSpacing = 0.2.sp,
        )

        // 主题切换
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            SectionHead(stringResource(com.pilotty.app.R.string.settings_theme))
            PilottyCard(modifier = Modifier.fillMaxWidth()) {
                Row(
                    modifier = Modifier.padding(14.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    PilottyChip(
                        text = stringResource(com.pilotty.app.R.string.settings_theme_system),
                        selected = themeMode == ThemeMode.System,
                        onClick = { onThemeChange(ThemeMode.System) },
                    )
                    PilottyChip(
                        text = stringResource(com.pilotty.app.R.string.settings_theme_light),
                        selected = themeMode == ThemeMode.Light,
                        onClick = { onThemeChange(ThemeMode.Light) },
                    )
                    PilottyChip(
                        text = stringResource(com.pilotty.app.R.string.settings_theme_dark),
                        selected = themeMode == ThemeMode.Dark,
                        onClick = { onThemeChange(ThemeMode.Dark) },
                    )
                }
            }
        }

        // Phase 10-C: 通知权限 (Android 13+) + 电池白名单 引导
        BatteryAndPermsSection()

        // Agent · LLM apiKey 配置 (M8 最短路径)
        AgentApiKeySection()

        // Mission Control 设计新增: Agent 工具权限矩阵入口 (09 屏)
        AgentToolsEntry(onNavigateAgentTools = onNavigateAgentTools)

        // Per-App VPN (M14)
        PerAppVpnSection()

        // Kill Switch 引导 (M15 方式 A)
        KillSwitchSection()

        // A4 LAN 代理入站 (手机当出口代理, 供其他设备借用)
        LocalProxySection()

        // Phase 9B-UI1: 观测 (实时日志 / 活跃连接) — 两个入口卡
        ObserveEntriesSection(
            onNavigateLogs = onNavigateLogs,
            onNavigateConnections = onNavigateConnections,
        )

        // Phase 10-E-D: 配置备份 / 恢复
        BackupRestoreSection()

        // Phase 10-E-E: WebDAV 云同步
        WebDAVSyncSection()

        // Phase 10-F-1: Failover UI (后端 Phase 2.5 做完未接 UI, 本轮补齐)
        FailoverSection()

        // Phase P1-A: Sniffing 档位 + IPv6 模式 (后端已就绪, 补 UI 入口)
        SniffingIpv6Section()

        // Phase P1-B: 网络自检 (5 段探测)
        NetCheckSection()

        // Phase P1-C: 自定义 DNS / DoH / DoQ (server 列表 CRUD + default 切换)
        DNSConfigSection()

        // 分流规则 (原 Rules tab 内联) —— RulesSection 内部已有 Templates / Active rules
        // 两个 Kicker 作为子标题,不再在外层重复"分流规则" 标题
        RulesSection()

        // 关于 — Phase 10-F-4: 版本号从 Go 层读, 不硬编; Go 侧含 sing-box / go 版本信息
        val coreVersion = remember { com.pilotty.app.data.PilottyRepository.version() }
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            SectionHead(stringResource(com.pilotty.app.R.string.settings_about))
            PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
                Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text(stringResource(com.pilotty.app.R.string.app_name), color = pc.ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
                    Text(
                        coreVersion.ifEmpty { stringResource(com.pilotty.app.R.string.settings_about_version_unknown) },
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

/**
 * Agent 工具权限矩阵的入口 —— 点击进入 AgentToolsScreen (09 屏)。
 */
@Composable
private fun AgentToolsEntry(onNavigateAgentTools: () -> Unit) {
    val pc = LocalPilottyColors.current
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        SectionHead(stringResource(com.pilotty.app.R.string.settings_agent_tools))
        PilottyCard(
            modifier = Modifier
                .fillMaxWidth()
                .clickable { onNavigateAgentTools() },
            soft = true,
        ) {
            Column(Modifier.padding(14.dp)) {
                Text(
                    stringResource(com.pilotty.app.R.string.settings_agent_tools_title),
                    color = pc.ink,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                )
                Text(
                    stringResource(com.pilotty.app.R.string.settings_agent_tools_subtitle),
                    color = pc.ink3,
                    fontSize = 11.sp,
                )
            }
        }
    }
}

@Composable
private fun ObserveEntriesSection(
    onNavigateLogs: () -> Unit,
    onNavigateConnections: () -> Unit,
) {
    val pc = LocalPilottyColors.current
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        SectionHead(stringResource(com.pilotty.app.R.string.settings_observe))
        PilottyCard(
            modifier = Modifier
                .fillMaxWidth()
                .clickable { onNavigateLogs() },
            soft = true,
        ) {
            Column(Modifier.padding(14.dp)) {
                Text(
                    stringResource(com.pilotty.app.R.string.settings_observe_logs_title),
                    color = pc.ink,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                )
                Text(
                    stringResource(com.pilotty.app.R.string.settings_observe_logs_subtitle),
                    color = pc.ink3,
                    fontSize = 11.sp,
                )
            }
        }
        PilottyCard(
            modifier = Modifier
                .fillMaxWidth()
                .clickable { onNavigateConnections() },
            soft = true,
        ) {
            Column(Modifier.padding(14.dp)) {
                Text(
                    stringResource(com.pilotty.app.R.string.settings_observe_conns_title),
                    color = pc.ink,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                )
                Text(
                    stringResource(com.pilotty.app.R.string.settings_observe_conns_subtitle),
                    color = pc.ink3,
                    fontSize = 11.sp,
                )
            }
        }
    }
}
