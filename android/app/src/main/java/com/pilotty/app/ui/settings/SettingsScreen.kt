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
 * Settings 顶层 — 经"原子组件归并"重构 (2026-04-27): 之前 16 个平级 section 一字排开难读,
 * 现按职能分 5 个子菜单 (Agent / VPN 进阶 / 节点与路由 / 备份与同步 / 系统权限),
 * 主页只保留高频项: 主题 / 网络自检 / 观测入口 / 关于。
 */
@Composable
fun SettingsScreen(
    themeMode: ThemeMode,
    onThemeChange: (ThemeMode) -> Unit,
    onNavigateLogs: () -> Unit = {},
    onNavigateConnections: () -> Unit = {},
    onNavigateAgentTools: () -> Unit = {},
    onNavigateAgent: () -> Unit = {},
    onNavigateVpnAdvanced: () -> Unit = {},
    onNavigateNetwork: () -> Unit = {},
    onNavigateBackup: () -> Unit = {},
    onNavigateSystem: () -> Unit = {},
) {
    val pc = LocalPilottyColors.current
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(pc.bg)
            .verticalScroll(rememberScrollState())
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

        // 主题切换 (高频, 保留 inline)
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

        // 5 子菜单入口列表 (按重要性排序: Agent → VPN → 网络 → 备份 → 系统)
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            SectionHead("分组")
            EntryCard(
                title = "Agent",
                subtitle = "API Key · 工具权限 · 最近活动",
                onClick = onNavigateAgent,
            )
            EntryCard(
                title = "VPN 进阶",
                subtitle = "Per-App · Kill Switch · LAN 代理入站",
                onClick = onNavigateVpnAdvanced,
            )
            EntryCard(
                title = "节点与路由",
                subtitle = "故障切换 · Sniffing/IPv6 · DNS · 分流规则",
                onClick = onNavigateNetwork,
            )
            EntryCard(
                title = "备份与同步",
                subtitle = "本地导出 · WebDAV 云同步",
                onClick = onNavigateBackup,
            )
            EntryCard(
                title = "系统权限",
                subtitle = "通知 · 电池白名单",
                onClick = onNavigateSystem,
            )
        }

        // 高频排错: 网络自检 (用户连不上时一键跑 5 段探测) — 保留 inline
        NetCheckSection()

        // 观测入口 (实时日志 / 活跃连接) — 高频且已是跳转卡, 保留
        ObserveEntriesSection(
            onNavigateLogs = onNavigateLogs,
            onNavigateConnections = onNavigateConnections,
        )

        // 关于
        val coreVersion = remember { com.pilotty.app.data.PilottyRepository.version() }
        val ctx = androidx.compose.ui.platform.LocalContext.current
        val privacyUrl = stringResource(com.pilotty.app.R.string.settings_privacy_policy_url)
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
                    Text(
                        stringResource(com.pilotty.app.R.string.settings_privacy_policy),
                        color = pc.accentInk,
                        fontSize = 12.sp,
                        fontWeight = FontWeight.SemiBold,
                        modifier = Modifier
                            .clickable {
                                val intent = android.content.Intent(
                                    android.content.Intent.ACTION_VIEW,
                                    android.net.Uri.parse(privacyUrl),
                                ).apply {
                                    addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK)
                                }
                                runCatching { ctx.startActivity(intent) }
                            }
                            .padding(top = 4.dp),
                    )
                }
            }
        }

        Spacer(Modifier.height(96.dp))
    }
}

enum class ThemeMode { System, Light, Dark }

@Composable
private fun EntryCard(title: String, subtitle: String, onClick: () -> Unit) {
    val pc = LocalPilottyColors.current
    PilottyCard(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onClick() },
        soft = true,
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 14.dp, vertical = 13.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(title, color = pc.ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
                Text(subtitle, color = pc.ink3, fontSize = 11.sp)
            }
            Text("›", color = pc.ink3, fontSize = 18.sp, fontWeight = FontWeight.Bold)
        }
    }
}

@Composable
private fun ObserveEntriesSection(
    onNavigateLogs: () -> Unit,
    onNavigateConnections: () -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        SectionHead(stringResource(com.pilotty.app.R.string.settings_observe))
        EntryCard(
            title = stringResource(com.pilotty.app.R.string.settings_observe_logs_title),
            subtitle = stringResource(com.pilotty.app.R.string.settings_observe_logs_subtitle),
            onClick = onNavigateLogs,
        )
        EntryCard(
            title = stringResource(com.pilotty.app.R.string.settings_observe_conns_title),
            subtitle = stringResource(com.pilotty.app.R.string.settings_observe_conns_subtitle),
            onClick = onNavigateConnections,
        )
    }
}

/* ====================== 5 子菜单 Screen ====================== */

@Composable
private fun SubScreenScaffold(
    title: String,
    onBack: () -> Unit,
    content: @Composable () -> Unit,
) {
    val pc = LocalPilottyColors.current
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(pc.bg)
            .verticalScroll(rememberScrollState())
            .imePadding(),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(
                "‹",
                color = pc.ink,
                fontSize = 24.sp,
                fontWeight = FontWeight.Bold,
                modifier = Modifier
                    .clickable { onBack() }
                    .padding(horizontal = 8.dp, vertical = 4.dp),
            )
            Text(title, color = pc.ink, fontSize = 18.sp, fontWeight = FontWeight.Bold)
        }
        Column(
            modifier = Modifier.padding(horizontal = 20.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            content()
            Spacer(Modifier.height(96.dp))
        }
    }
}

@Composable
fun SettingsAgentScreen(onBack: () -> Unit, onNavigateAgentTools: () -> Unit) {
    SubScreenScaffold(title = "Agent", onBack = onBack) {
        AgentLLMSection()
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            SectionHead(stringResource(com.pilotty.app.R.string.settings_agent_tools))
            EntryCard(
                title = stringResource(com.pilotty.app.R.string.settings_agent_tools_title),
                subtitle = stringResource(com.pilotty.app.R.string.settings_agent_tools_subtitle),
                onClick = onNavigateAgentTools,
            )
        }
        AgentActivitySection()
    }
}

@Composable
fun SettingsVpnAdvancedScreen(onBack: () -> Unit) {
    SubScreenScaffold(title = "VPN 进阶", onBack = onBack) {
        PerAppVpnSection()
        KillSwitchSection()
        LocalProxySection()
    }
}

@Composable
fun SettingsNetworkScreen(onBack: () -> Unit, onNavigateChat: () -> Unit = {}) {
    SubScreenScaffold(title = "节点与路由", onBack = onBack) {
        FailoverSection()
        SniffingIpv6Section()
        DNSConfigSection()
        com.pilotty.app.ui.RulesSection(onNavigateChat = onNavigateChat)
    }
}

@Composable
fun SettingsBackupScreen(onBack: () -> Unit) {
    SubScreenScaffold(title = "备份与同步", onBack = onBack) {
        BackupRestoreSection()
        WebDAVSyncSection()
    }
}

@Composable
fun SettingsSystemScreen(onBack: () -> Unit) {
    SubScreenScaffold(title = "系统权限", onBack = onBack) {
        BatteryAndPermsSection()
    }
}
