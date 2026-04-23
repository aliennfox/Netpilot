package com.pilotty.app.ui.settings

import android.util.Log
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.data.FailoverStatusDto
import com.pilotty.app.data.PilottyRepository
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.theme.LocalPilottyColors
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * Phase 10-F-1: Failover UI 接入。
 *
 * 后端 Phase 2.5 就做完了 (internal/failover/monitor.go), Go API 完整 —— 但 UI 零入口,
 * 直到本次 orphan 审计被捞出来。
 *
 * 机制: 默认每 30s 探测一次当前 selector group 的活跃节点, 连续 3 次失败就自动切换到
 * 同组内的备选节点, 60s 冷却防止抖动。
 *
 * UI 简化为一个 Switch + 状态行:
 *   当前节点 · 连续失败 N 次 · 上次切换 @xxx → yyy
 */
@Composable
fun FailoverSection() {
    val pc = LocalPilottyColors.current
    val scope = rememberCoroutineScope()
    var status by remember { mutableStateOf<FailoverStatusDto?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }

    // 每 5s 拉一次状态 (轻量 poll, 比给 Go 加 callback 简单)
    LaunchedEffect(Unit) {
        while (true) {
            try {
                status = PilottyRepository.failoverStatus()
                error = null
            } catch (t: Throwable) {
                Log.w(TAG, "failoverStatus failed", t)
                error = t.message
            }
            delay(5000L)
        }
    }

    val s = status
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Kicker("故障自动切换")
        PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
            Column(Modifier.padding(14.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text(
                            "节点健康监控",
                            color = pc.ink,
                            fontSize = 14.sp,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Spacer(Modifier.height(2.dp))
                        Text(
                            "每 30 秒探测一次, 连续 3 次失败自动切到同组备选节点 (60s 冷却)。",
                            color = pc.ink3,
                            fontSize = 11.sp,
                        )
                    }
                    Switch(
                        checked = s?.running == true,
                        enabled = !busy,
                        onCheckedChange = { enable ->
                            busy = true
                            scope.launch {
                                try {
                                    status = if (enable) PilottyRepository.startFailover()
                                    else PilottyRepository.stopFailover()
                                    error = null
                                } catch (t: Throwable) {
                                    Log.w(TAG, "toggle failover failed", t)
                                    error = t.message
                                } finally {
                                    busy = false
                                }
                            }
                        },
                        colors = SwitchDefaults.colors(
                            checkedThumbColor = pc.accent,
                            checkedTrackColor = pc.accent.copy(alpha = 0.35f),
                        ),
                    )
                }

                if (s != null && s.running) {
                    Spacer(Modifier.height(8.dp))
                    val parts = buildList {
                        add("group=${s.group.ifEmpty { "?" }}")
                        add("当前=${s.currentNode.ifEmpty { "-" }}")
                        if (s.consecutiveFails > 0) add("连败×${s.consecutiveFails}")
                        if (s.lastSwitchTo.isNotEmpty()) add("上次切→${s.lastSwitchTo}")
                    }
                    Text(
                        parts.joinToString(" · "),
                        color = pc.ink3,
                        fontSize = 11.sp,
                        fontFamily = FontFamily.Monospace,
                    )
                    if (s.lastError.isNotEmpty()) {
                        Spacer(Modifier.height(2.dp))
                        Text(
                            "最近错误: ${s.lastError}",
                            color = pc.error,
                            fontSize = 11.sp,
                            fontFamily = FontFamily.Monospace,
                        )
                    }
                }

                error?.let {
                    Spacer(Modifier.height(4.dp))
                    Text("· $it", color = pc.error, fontSize = 11.sp)
                }
            }
        }
    }
}

private const val TAG = "FailoverSection"
