package com.pilotty.app

import android.content.Intent
import android.os.Bundle
import android.util.Log
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Forum
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Hub
import androidx.compose.material.icons.filled.Layers
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.pilotty.app.ui.ChatScreen
import com.pilotty.app.ui.HomeScreen
import com.pilotty.app.ui.NodesScreen
import com.pilotty.app.ui.components.FloatingBottomNav
import com.pilotty.app.ui.components.NavItem
import com.pilotty.app.ui.import_.ImportBus
import com.pilotty.app.ui.observe.ConnectionsScreen
import com.pilotty.app.ui.observe.LogsScreen
import com.pilotty.app.ui.qr.QrScanScreen
import com.pilotty.app.ui.settings.SettingsScreen
import com.pilotty.app.ui.settings.ThemeMode
import com.pilotty.app.ui.settings.ThemePrefs
import androidx.compose.runtime.collectAsState
import com.pilotty.app.ui.subs.SubsScreen
import com.pilotty.app.ui.theme.LocalPilottyColors
import com.pilotty.app.ui.theme.PilottyTheme
import com.pilotty.app.vpn.PilottyTileService
import com.pilotty.app.vpn.StartVpnBus
import com.pilotty.app.vpn.VpnController
import androidx.compose.runtime.LaunchedEffect

class MainActivity : ComponentActivity() {
    lateinit var vpn: VpnController

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        vpn = VpnController(this).also { it.registerLauncher() }
        // App 被 Deep Link 冷启动时, intent 在 onCreate 里读; 热启动走 onNewIntent
        handleIncomingUri(intent)
        handleTileIntent(intent)
        val themePrefs = ThemePrefs.get(applicationContext)
        setContent {
            // Phase 10-E: ThemeMode 持久化, SharedPreferences 跨重启
            val themeMode by themePrefs.mode.collectAsState()
            val dark = when (themeMode) {
                ThemeMode.System -> isSystemInDarkTheme()
                ThemeMode.Light -> false
                ThemeMode.Dark -> true
            }
            PilottyTheme(darkTheme = dark) {
                val pc = LocalPilottyColors.current
                Surface(modifier = Modifier.fillMaxSize(), color = pc.bg) {
                    PilottyApp(
                        onStartVpn = { vpn.start() },
                        onStopVpn = { vpn.stop() },
                        themeMode = themeMode,
                        onThemeChange = { themePrefs.set(it) },
                    )
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleIncomingUri(intent)
        handleTileIntent(intent)
    }

    /** QS Tile 点击启动 VPN 时, 由 tile 的 startActivityAndCollapse 带这个 action 进来;
     *  Activity 完成 VpnService.prepare() 授权流程 (tile 本身不是 Activity 做不了)。 */
    private fun handleTileIntent(intent: Intent?) {
        if (intent?.action == PilottyTileService.ACTION_START_VPN_FROM_TILE) {
            Log.i("MainActivity", "tile → start VPN")
            vpn.start()
        }
    }

    /**
     * 消费 intent.data: 既支持 `vmess://...` 节点 URI (走 ImportNodeURI),
     * 也支持 `sub://...` / `clash://...` / `pilotty://subscribe?url=...` 订阅 URL。
     * 当前策略: 所有 scheme 都丢给 ImportBus, 由 UI 层弹 dialog 让用户确认再导入。
     */
    private fun handleIncomingUri(intent: Intent?) {
        val uri = intent?.data?.toString() ?: return
        if (uri.isBlank()) return
        Log.i("MainActivity", "deep-link received: $uri")
        ImportBus.post(uri)
    }
}

@Composable
fun PilottyApp(
    onStartVpn: () -> Unit,
    onStopVpn: () -> Unit,
    themeMode: ThemeMode,
    onThemeChange: (ThemeMode) -> Unit,
) {
    val navItems = listOf(
        NavItem("home", androidx.compose.ui.res.stringResource(R.string.nav_home), Icons.Filled.Home),
        NavItem("chat", androidx.compose.ui.res.stringResource(R.string.nav_chat), Icons.Filled.Forum),
        NavItem("nodes", androidx.compose.ui.res.stringResource(R.string.nav_nodes), Icons.Filled.Hub),
        NavItem("subs", androidx.compose.ui.res.stringResource(R.string.nav_subs), Icons.Filled.Layers),
        NavItem("settings", androidx.compose.ui.res.stringResource(R.string.nav_settings), Icons.Filled.Settings),
    )
    val nav = rememberNavController()
    val backStack by nav.currentBackStackEntryAsState()
    val current = backStack?.destination?.route ?: "home"

    // Nodes 测速等场景 ViewModel 层需要拉起 VPN, 但 VpnService.prepare 要 Activity-scope
    // launcher, 这里 collect StartVpnBus 的一次性请求转给 onStartVpn (现有链路)。
    LaunchedEffect(Unit) {
        Log.i("MainActivity", "StartVpnBus collector attached")
        StartVpnBus.requests.collect {
            Log.i("MainActivity", "StartVpnBus → onStartVpn")
            onStartVpn()
        }
    }
    LaunchedEffect(Unit) {
        Log.i("MainActivity", "StopVpnBus collector attached")
        com.pilotty.app.vpn.StopVpnBus.requests.collect {
            Log.i("MainActivity", "StopVpnBus → onStopVpn")
            onStopVpn()
        }
    }

    // Mission Control 设计: 底部 nav flush, 和内容共享 Column 布局 (非 floating)
    // 注: 此前曾用 imeVisible 隐藏 nav, 但 Chat 场景 TextField 一聚焦就让用户回不到 Home
    //    (按 Home 键也没用, IME 是前台, 关 IME 前用户一直被困在 Chat)。
    //    windowInset 会自动把整个 Column 抬到 IME 上方, nav 不会压到输入, 放心常驻。

    // 子页面 (node_detail / agent_tools / logs / connections / qr) 不显示底部 nav
    val isSubPage = current.startsWith("node_detail") || current == "agent_tools" ||
        current == "logs" || current == "connections" || current == "qr" ||
        current.startsWith("settings_")

    androidx.compose.foundation.layout.Column(modifier = Modifier.fillMaxSize()) {
        NavHost(
            navController = nav,
            startDestination = "home",
            modifier = Modifier.weight(1f).fillMaxSize(),
        ) {
            composable("home") {
                HomeScreen(
                    onStartVpn = onStartVpn,
                    onStopVpn = onStopVpn,
                    onNavigateChat = { nav.navigate("chat") },
                    onNavigateLogs = { nav.navigate("logs") },
                )
            }
            composable("chat") {
                ChatScreen(onNavigateAgentTrace = { nav.navigate("settings_agent") })
            }
            composable("nodes") {
                NodesScreen(
                    onNavigateSubs = { nav.navigate("subs") },
                    onOpenDetail = { id -> nav.navigate("node_detail/$id") },
                )
            }
            composable("node_detail/{nodeId}") { entry ->
                val nodeId = entry.arguments?.getString("nodeId") ?: ""
                com.pilotty.app.ui.NodeDetailScreen(nodeId = nodeId, onBack = { nav.popBackStack() })
            }
            composable("subs") { SubsScreen(onNavigateQrScan = { nav.navigate("qr") }) }
            composable("settings") {
                SettingsScreen(
                    themeMode = themeMode,
                    onThemeChange = onThemeChange,
                    onNavigateLogs = { nav.navigate("logs") },
                    onNavigateConnections = { nav.navigate("connections") },
                    onNavigateAgentTools = { nav.navigate("agent_tools") },
                    onNavigateAgent = { nav.navigate("settings_agent") },
                    onNavigateVpnAdvanced = { nav.navigate("settings_vpn_advanced") },
                    onNavigateNetwork = { nav.navigate("settings_network") },
                    onNavigateBackup = { nav.navigate("settings_backup") },
                    onNavigateSystem = { nav.navigate("settings_system") },
                )
            }
            composable("agent_tools") {
                com.pilotty.app.ui.settings.AgentToolsScreen(onBack = { nav.popBackStack() })
            }
            composable("settings_agent") {
                com.pilotty.app.ui.settings.SettingsAgentScreen(
                    onBack = { nav.popBackStack() },
                    onNavigateAgentTools = { nav.navigate("agent_tools") },
                )
            }
            composable("settings_vpn_advanced") {
                com.pilotty.app.ui.settings.SettingsVpnAdvancedScreen(onBack = { nav.popBackStack() })
            }
            composable("settings_network") {
                com.pilotty.app.ui.settings.SettingsNetworkScreen(
                    onBack = { nav.popBackStack() },
                    onNavigateChat = {
                        nav.popBackStack()
                        nav.navigate("chat")
                    },
                )
            }
            composable("settings_backup") {
                com.pilotty.app.ui.settings.SettingsBackupScreen(onBack = { nav.popBackStack() })
            }
            composable("settings_system") {
                com.pilotty.app.ui.settings.SettingsSystemScreen(onBack = { nav.popBackStack() })
            }
            composable("logs") { LogsScreen(onBack = { nav.popBackStack() }) }
            composable("connections") { ConnectionsScreen(onBack = { nav.popBackStack() }) }
            composable("qr") {
                QrScanScreen(
                    onBack = { nav.popBackStack() },
                    onResult = { scanned ->
                        com.pilotty.app.ui.import_.ImportBus.post(scanned)
                        nav.popBackStack()
                    },
                )
            }
        }

        if (!isSubPage) {
            FloatingBottomNav(
                items = navItems,
                current = current,
                onTab = { route ->
                    if (current != route) {
                        nav.navigate(route) {
                            popUpTo(nav.graph.startDestinationId) { saveState = true }
                            launchSingleTop = true
                            restoreState = true
                        }
                    }
                },
            )
        }
    }
}
