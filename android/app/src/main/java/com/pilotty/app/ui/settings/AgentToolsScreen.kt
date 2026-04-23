package com.pilotty.app.ui.settings

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.ui.components.*
import com.pilotty.app.ui.theme.LocalPilottyColors

/**
 * AgentToolsScreen —— 每个 tool 的开关 + auto 自动确认开关。
 * 设计稿 09 屏 (Dark 变体示范)。
 *
 * 当前: 前端仅展示视觉; 后端 Permission 系统实际由 Go 的 TrustMode (ask/auto/strict) 管控。
 * 未来可以做成每 tool 独立权限持久化。
 */
data class AgentTool(
    val name: String,
    val desc: String,
    val enabled: Boolean,
    val auto: Boolean,
)

private val DEFAULT_TOOLS = listOf(
    AgentTool("list_nodes", "列出可用节点", enabled = true, auto = true),
    AgentTool("test_latency", "探测节点延迟", enabled = true, auto = true),
    AgentTool("switch_node", "切换活动节点", enabled = true, auto = true),
    AgentTool("create_route", "创建应用路由", enabled = true, auto = false),
    AgentTool("rollback", "回滚到上个快照", enabled = true, auto = false),
    AgentTool("delete_node", "删除节点", enabled = false, auto = false),
    AgentTool("edit_config", "编辑全局配置", enabled = false, auto = false),
)

@Composable
fun AgentToolsScreen(onBack: () -> Unit) {
    val pc = LocalPilottyColors.current
    var tools by remember { mutableStateOf(DEFAULT_TOOLS) }

    Column(modifier = Modifier.fillMaxSize().background(pc.bg)) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Row(
                modifier = Modifier.clickable { onBack() },
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                Text("‹", color = pc.ink2, fontSize = 18.sp, fontWeight = FontWeight.Bold)
                Text("System", color = pc.ink2, fontSize = 13.sp)
            }
        }
        HorizontalDivider(color = pc.hairline, thickness = 1.dp)

        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp, vertical = 14.dp),
        ) {
            Text(
                "Agent 工具",
                color = pc.ink,
                fontSize = 20.sp,
                fontWeight = FontWeight.Bold,
                letterSpacing = (-0.4).sp,
            )
            Spacer(Modifier.height(4.dp))
            Text(
                "控制 Agent 可调用的工具。关闭 Auto 时, 调用前会要求你确认。",
                color = pc.ink2,
                fontSize = 12.5.sp,
                lineHeight = 19.sp,
            )
            Spacer(Modifier.height(16.dp))

            CardGroup(modifier = Modifier.fillMaxWidth()) {
                tools.forEachIndexed { i, t ->
                    if (i > 0) HorizontalDivider(color = pc.hairline, thickness = 1.dp)
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 18.dp, vertical = 14.dp),
                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                t.name,
                                color = pc.ink,
                                fontSize = 13.sp,
                                fontWeight = FontWeight.SemiBold,
                                fontFamily = FontFamily.Monospace,
                            )
                            Spacer(Modifier.height(3.dp))
                            Text(
                                t.desc,
                                color = pc.ink2,
                                fontSize = 12.sp,
                            )
                            if (t.enabled) {
                                Spacer(Modifier.height(6.dp))
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                                ) {
                                    Text(
                                        "AUTO",
                                        color = pc.ink3,
                                        fontSize = 10.5.sp,
                                        fontWeight = FontWeight.Bold,
                                        letterSpacing = 1.05.sp,
                                    )
                                    PilottyToggle(
                                        checked = t.auto,
                                        onCheckedChange = { v ->
                                            tools = tools.toMutableList().apply {
                                                set(i, t.copy(auto = v))
                                            }
                                        },
                                        scale = 0.8f,
                                    )
                                }
                            }
                        }
                        PilottyToggle(
                            checked = t.enabled,
                            onCheckedChange = { v ->
                                tools = tools.toMutableList().apply {
                                    set(i, t.copy(enabled = v, auto = if (!v) false else t.auto))
                                }
                            },
                        )
                    }
                }
            }

            Spacer(Modifier.height(16.dp))
            Surface(
                color = pc.surface2,
                shape = androidx.compose.foundation.shape.RoundedCornerShape(12.dp),
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(
                    "关闭的工具不会被 Agent 调用; AUTO 关闭的工具会先弹出确认 (由 Go 侧 TrustMode 实现)。",
                    color = pc.ink2,
                    fontSize = 12.sp,
                    lineHeight = 18.sp,
                    modifier = Modifier.padding(14.dp),
                )
            }
        }
    }
}

