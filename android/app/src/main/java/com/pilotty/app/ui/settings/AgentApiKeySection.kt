package com.pilotty.app.ui.settings

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.pilotty.app.agent.ApiKeyPrefs
import com.pilotty.app.ui.components.SectionHead
import com.pilotty.app.ui.components.PilottyButton
import com.pilotty.app.ui.components.PilottyButtonVariant
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.theme.LocalPilottyColors

/**
 * LLM apiKey 配置卡 (M8 最短路径)。
 *
 * 用户填 DeepSeek/硅基流动 key → 保存到 [ApiKeyPrefs] (明文 SP) → 提示需要重启 App。
 * 重启后 [com.pilotty.app.PilottyApp.onCreate] 会读 SP 值传给 PilottyCore.init, 真正激活 Agent。
 */
@Composable
fun AgentApiKeySection() {
    val ctx = LocalContext.current
    val pc = LocalPilottyColors.current
    val prefs = remember { ApiKeyPrefs.get(ctx) }
    val savedKey by prefs.state.collectAsStateWithLifecycle()
    val isConfigured = savedKey.isNotBlank()

    var editing by remember { mutableStateOf(false) }
    var draft by remember(editing) { mutableStateOf(if (editing) savedKey else "") }
    var visible by remember { mutableStateOf(false) }

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        SectionHead("Agent · LLM")
        PilottyCard(modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("DeepSeek via 硅基流动", color = pc.ink, fontSize = 14.sp, fontWeight = FontWeight.Medium)

                if (!editing) {
                    if (isConfigured) {
                        Text(
                            "已配置 · ${prefs.masked()}",
                            color = pc.accentInk,
                            fontSize = 13.sp,
                            fontFamily = FontFamily.Monospace,
                        )
                        Text(
                            "Chat tab 输入自然语言, Agent 会真调工具执行 (保存立即生效, 热重载)。",
                            color = pc.ink3,
                            fontSize = 12.sp,
                        )
                    } else {
                        Text(
                            "尚未配置。 Chat tab 只能用本地 IntentRouter, Agent 未启用。",
                            color = pc.ink3,
                            fontSize = 12.sp,
                        )
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        PilottyButton(
                            text = if (isConfigured) "修改" else "配置 Key",
                            onClick = { editing = true },
                            variant = PilottyButtonVariant.Primary,
                            small = true,
                        )
                        if (isConfigured) {
                            PilottyButton(
                                text = "清除",
                                onClick = { prefs.clear() },
                                variant = PilottyButtonVariant.Ghost,
                                small = true,
                            )
                        }
                    }
                } else {
                    OutlinedTextField(
                        value = draft,
                        onValueChange = { draft = it },
                        label = { Text("API Key (sk-...)") },
                        singleLine = true,
                        visualTransformation = if (visible) VisualTransformation.None else PasswordVisualTransformation(),
                        trailingIcon = {
                            Text(
                                if (visible) "隐藏" else "显示",
                                color = pc.ink3,
                                fontSize = 11.sp,
                                modifier = Modifier
                                    .clickable { visible = !visible }
                                    .padding(horizontal = 8.dp, vertical = 6.dp),
                            )
                        },
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedBorderColor = pc.ink,
                            unfocusedBorderColor = pc.hairlineStrong,
                        ),
                        modifier = Modifier.fillMaxWidth(),
                    )
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        PilottyButton(
                            text = "保存",
                            onClick = {
                                prefs.set(draft)
                                editing = false
                            },
                            variant = PilottyButtonVariant.Primary,
                            small = true,
                            enabled = draft.isNotBlank(),
                        )
                        PilottyButton(
                            text = "取消",
                            onClick = { editing = false },
                            variant = PilottyButtonVariant.Ghost,
                            small = true,
                        )
                    }
                }

                Text(
                    "提示: 保存立即生效 (Go orchestrator 热重载, 无需重启)。 当前明文存储, 自用可接受, 公共分发前必须升 EncryptedSharedPreferences。",
                    color = pc.ink4,
                    fontSize = 10.5.sp,
                    fontFamily = FontFamily.Monospace,
                )
            }
        }
    }
}
