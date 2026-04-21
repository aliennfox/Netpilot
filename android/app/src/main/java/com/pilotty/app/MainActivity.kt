package com.pilotty.app

import android.os.Bundle
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
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
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
            composable("nodes") { NodesScreen() }
            composable("subs") { SubsScreen() }
            composable("settings") {
                SettingsScreen(themeMode = themeMode, onThemeChange = onThemeChange)
            }
        }

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
