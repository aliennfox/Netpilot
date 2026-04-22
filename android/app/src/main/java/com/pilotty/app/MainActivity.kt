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
import com.pilotty.app.ui.settings.SettingsScreen
import com.pilotty.app.ui.settings.ThemeMode
import com.pilotty.app.ui.subs.SubsScreen
import com.pilotty.app.ui.theme.LocalPilottyColors
import com.pilotty.app.ui.theme.PilottyTheme
import com.pilotty.app.vpn.VpnController

class MainActivity : ComponentActivity() {
    lateinit var vpn: VpnController

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        vpn = VpnController(this).also { it.registerLauncher() }
        // App 被 Deep Link 冷启动时, intent 在 onCreate 里读; 热启动走 onNewIntent
        handleIncomingUri(intent)
        setContent {
            var themeMode by rememberSaveable { mutableStateOf(ThemeMode.System) }
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
                        onThemeChange = { themeMode = it },
                    )
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleIncomingUri(intent)
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

    Box(modifier = Modifier.fillMaxSize()) {
        NavHost(
            navController = nav,
            startDestination = "home",
            modifier = Modifier.fillMaxSize(),
        ) {
            composable("home") {
                HomeScreen(
                    onStartVpn = onStartVpn,
                    onStopVpn = onStopVpn,
                    onNavigateChat = { nav.navigate("chat") },
                )
            }
            composable("chat") { ChatScreen() }
            composable("nodes") { NodesScreen(onNavigateSubs = { nav.navigate("subs") }) }
            composable("subs") { SubsScreen() }
            composable("settings") {
                SettingsScreen(themeMode = themeMode, onThemeChange = onThemeChange)
            }
        }

        // Phase 4: IME 打开时隐藏 floating nav, 避免它覆盖 Settings AgentApiKey 的保存/取消 按钮。
        // 直接用 ViewTreeObserver + WindowInsetsCompat — 比 compose-foundation 的 isImeVisible 可靠
        // (后者在没显式 setDecorFitsSystemWindows(false) 的情况下状态不刷新, 实测首帧就卡 true)。
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
        if (!imeVisible) {
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
                modifier = Modifier.align(Alignment.BottomCenter),
            )
        }
    }
}
