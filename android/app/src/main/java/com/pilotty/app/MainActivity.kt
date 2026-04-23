package com.pilotty.app

import android.content.Intent
import android.os.Bundle
import android.util.Log
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Forum
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Hub
import androidx.compose.material.icons.filled.Layers
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalView
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
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

private val navItems = listOf(
    NavItem("home", "Home", Icons.Filled.Home),
    NavItem("chat", "Chat", Icons.Filled.Forum),
    NavItem("nodes", "Nodes", Icons.Filled.Hub),
    NavItem("subs", "Subs", Icons.Filled.Layers),
    NavItem("settings", "Settings", Icons.Filled.Settings),
)

@Composable
fun PilottyApp(
    onStartVpn: () -> Unit,
    onStopVpn: () -> Unit,
    themeMode: ThemeMode,
    onThemeChange: (ThemeMode) -> Unit,
) {
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
    // IME 弹起时仍隐藏 nav, 避免遮住输入保存按钮
    val view = LocalView.current
    var imeVisible by remember { mutableStateOf(false) }
    DisposableEffect(view) {
        val listener = android.view.ViewTreeObserver.OnGlobalLayoutListener {
            val insets = ViewCompat.getRootWindowInsets(view)
            imeVisible = insets?.isVisible(WindowInsetsCompat.Type.ime()) ?: false
        }
        view.viewTreeObserver.addOnGlobalLayoutListener(listener)
        onDispose { view.viewTreeObserver.removeOnGlobalLayoutListener(listener) }
    }

    // 子页面 (node_detail / agent_tools / logs / connections / qr) 不显示底部 nav
    val isSubPage = current.startsWith("node_detail") || current == "agent_tools" ||
        current == "logs" || current == "connections" || current == "qr"

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
                )
            }
            composable("chat") { ChatScreen() }
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
                )
            }
            composable("agent_tools") {
                com.pilotty.app.ui.settings.AgentToolsScreen(onBack = { nav.popBackStack() })
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

        if (!imeVisible && !isSubPage) {
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
