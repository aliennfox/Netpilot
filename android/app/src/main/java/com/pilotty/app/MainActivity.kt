package com.pilotty.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Forum
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Hub
import androidx.compose.material.icons.filled.Rule
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.pilotty.app.ui.ChatScreen
import com.pilotty.app.ui.DashboardScreen
import com.pilotty.app.ui.NodesScreen
import com.pilotty.app.ui.RulesScreen
import com.pilotty.app.ui.settings.SettingsScreen
import com.pilotty.app.vpn.VpnController

class MainActivity : ComponentActivity() {
    lateinit var vpn: VpnController

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        vpn = VpnController(this).also { it.registerLauncher() }
        setContent {
            MaterialTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    PilottyApp(onStartVpn = { vpn.start() }, onStopVpn = { vpn.stop() })
                }
            }
        }
    }
}

private data class NavItem(val route: String, val label: String, val icon: ImageVector)

private val navItems = listOf(
    NavItem("dashboard", "首页", Icons.Filled.Home),
    NavItem("chat", "对话", Icons.Filled.Forum),
    NavItem("nodes", "节点", Icons.Filled.Hub),
    NavItem("rules", "规则", Icons.Filled.Rule),
    NavItem("settings", "设置", Icons.Filled.Settings),
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PilottyApp(onStartVpn: () -> Unit, onStopVpn: () -> Unit) {
    val nav = rememberNavController()
    val backStack by nav.currentBackStackEntryAsState()
    val current = backStack?.destination?.route ?: "dashboard"

    Scaffold(
        topBar = {
            TopAppBar(title = { Text("Pilotty · " + (navItems.firstOrNull { it.route == current }?.label ?: "")) })
        },
        bottomBar = {
            NavigationBar {
                navItems.forEach { item ->
                    NavigationBarItem(
                        selected = current == item.route ||
                            (backStack?.destination?.hierarchy?.any { it.route == item.route } == true),
                        onClick = {
                            if (current != item.route) {
                                nav.navigate(item.route) {
                                    popUpTo(nav.graph.startDestinationId) { saveState = true }
                                    launchSingleTop = true
                                    restoreState = true
                                }
                            }
                        },
                        icon = { Icon(item.icon, contentDescription = item.label) },
                        label = { Text(item.label) },
                    )
                }
            }
        },
    ) { pad ->
        Box(modifier = Modifier.fillMaxSize().padding(pad)) {
            NavHost(navController = nav, startDestination = "dashboard") {
                composable("dashboard") { DashboardScreen(onStartVpn = onStartVpn, onStopVpn = onStopVpn) }
                composable("chat") { ChatScreen() }
                composable("nodes") { NodesScreen() }
                composable("rules") { RulesScreen() }
                composable("settings") { SettingsScreen() }
            }
        }
    }
}
