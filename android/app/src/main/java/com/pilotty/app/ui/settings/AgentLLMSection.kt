package com.pilotty.app.ui.settings

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.background
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.pilotty.app.PilottyCore
import com.pilotty.app.agent.LLMConfigPrefs
import com.pilotty.app.ui.components.SectionHead
import com.pilotty.app.ui.components.PilottyButton
import com.pilotty.app.ui.components.PilottyButtonVariant
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.theme.LocalPilottyColors
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

/**
 * Agent · LLM 三件套配置 (#37)。
 *
 * 取代 AgentApiKeySection 的单字段 API Key 方案。 用户可填:
 *  - BaseURL (硅基流动 / dashscope / OpenRouter / 自定义)
 *  - Model (手动填或点 "拉模型列表" 从 GET /v1/models 选)
 *  - API Key
 *
 * 任一字段空 → BaseURL/Model 走 Go default, ApiKey 空 = 关 Agent。
 */
@Composable
fun AgentLLMSection() {
    val ctx = LocalContext.current
    val pc = LocalPilottyColors.current
    val prefs = remember { LLMConfigPrefs.get(ctx) }
    val cfg by prefs.state.collectAsStateWithLifecycle()
    val scope = rememberCoroutineScope()

    var editing by remember { mutableStateOf(false) }
    var draftBase by remember(editing) { mutableStateOf(if (editing) cfg.baseURL else "") }
    var draftModel by remember(editing) { mutableStateOf(if (editing) cfg.model else "") }
    var draftKey by remember(editing) { mutableStateOf(if (editing) cfg.apiKey else "") }
    var keyVisible by remember { mutableStateOf(false) }

    // 模型下拉缓存
    var modelList by remember { mutableStateOf<List<String>>(emptyList()) }
    var modelListLoading by remember { mutableStateOf(false) }
    var modelListError by remember { mutableStateOf<String?>(null) }
    var modelMenuOpen by remember { mutableStateOf(false) }

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        SectionHead("Agent · LLM")
        PilottyCard(modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {

                if (!editing) {
                    // —— 只读视图 ——
                    if (cfg.isConfigured) {
                        Text("已配置", color = pc.accentInk, fontSize = 13.sp, fontWeight = FontWeight.Medium)
                        InfoLine(label = "BaseURL", value = cfg.baseURL.ifBlank { "默认" }, pc = pc)
                        InfoLine(label = "Model",   value = cfg.model.ifBlank { "默认" }, pc = pc)
                        InfoLine(label = "API Key", value = prefs.maskedKey(), pc = pc, mono = true)
                        Text(
                            "Chat tab 输入自然语言, Agent 会真调工具执行。 改任一字段保存立即生效 (热重载)。",
                            color = pc.ink3, fontSize = 12.sp,
                        )
                    } else {
                        Text(
                            "尚未配置。 Chat tab 只能用本地 IntentRouter, Agent 未启用。",
                            color = pc.ink3, fontSize = 12.sp,
                        )
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        PilottyButton(
                            text = if (cfg.isConfigured) "修改" else "配置 LLM",
                            onClick = { editing = true },
                            variant = PilottyButtonVariant.Primary,
                            small = true,
                        )
                        if (cfg.isConfigured) {
                            PilottyButton(
                                text = "清除",
                                onClick = { prefs.clear() },
                                variant = PilottyButtonVariant.Ghost,
                                small = true,
                            )
                        }
                    }
                } else {
                    // —— 编辑视图 ——
                    Text("Provider 预设", color = pc.ink, fontSize = 13.sp, fontWeight = FontWeight.Medium)
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        LLMConfigPrefs.PRESETS.forEach { p ->
                            val active = draftBase == p.baseURL && p.baseURL.isNotBlank()
                            Box(
                                modifier = Modifier
                                    .weight(1f)
                                    .clip(RoundedCornerShape(6.dp))
                                    .background(if (active) pc.accent.copy(alpha = 0.18f) else pc.surface2)
                                    .clickable {
                                        draftBase = p.baseURL
                                        if (draftModel.isBlank() && p.defaultModel.isNotBlank()) {
                                            draftModel = p.defaultModel
                                        }
                                        modelList = emptyList()
                                        modelListError = null
                                    }
                                    .padding(vertical = 8.dp, horizontal = 6.dp),
                                contentAlignment = Alignment.Center,
                            ) {
                                Text(
                                    p.label,
                                    color = if (active) pc.accentInk else pc.ink2,
                                    fontSize = 11.sp,
                                    fontWeight = if (active) FontWeight.Medium else FontWeight.Normal,
                                )
                            }
                        }
                    }

                    OutlinedTextField(
                        value = draftBase,
                        onValueChange = { draftBase = it },
                        label = { Text("BaseURL (留空走默认 SiliconFlow)") },
                        singleLine = true,
                        colors = textFieldColors(pc),
                        modifier = Modifier.fillMaxWidth(),
                    )

                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        Box(modifier = Modifier.weight(1f)) {
                            OutlinedTextField(
                                value = draftModel,
                                onValueChange = { draftModel = it },
                                label = { Text("Model (留空走默认)") },
                                singleLine = true,
                                colors = textFieldColors(pc),
                                modifier = Modifier.fillMaxWidth(),
                            )
                            DropdownMenu(
                                expanded = modelMenuOpen,
                                onDismissRequest = { modelMenuOpen = false },
                            ) {
                                modelList.forEach { id ->
                                    DropdownMenuItem(
                                        text = { Text(id, fontSize = 12.sp, fontFamily = FontFamily.Monospace) },
                                        onClick = {
                                            draftModel = id
                                            modelMenuOpen = false
                                        },
                                    )
                                }
                            }
                        }
                        PilottyButton(
                            text = if (modelListLoading) "..." else "拉列表",
                            onClick = {
                                modelListError = null
                                modelListLoading = true
                                scope.launch {
                                    val result = withContext(Dispatchers.IO) {
                                        runCatching {
                                            PilottyCore.listLLMModels(draftBase.trim(), draftKey.trim())
                                        }.getOrElse { """{"error":"${it.message}"}""" }
                                    }
                                    modelListLoading = false
                                    val parsed = runCatching { Json.decodeFromString<ModelsResp>(result) }
                                        .getOrNull()
                                    if (parsed?.error != null) {
                                        modelListError = parsed.error
                                    } else if (parsed?.models != null) {
                                        modelList = parsed.models
                                        modelMenuOpen = modelList.isNotEmpty()
                                        if (modelList.isEmpty()) modelListError = "无模型返回"
                                    } else {
                                        modelListError = "解析失败"
                                    }
                                }
                            },
                            variant = PilottyButtonVariant.Ghost,
                            small = true,
                            enabled = !modelListLoading && draftKey.isNotBlank(),
                        )
                    }
                    modelListError?.let {
                        Text("拉模型列表失败: $it", color = pc.ink3, fontSize = 11.sp)
                    }

                    OutlinedTextField(
                        value = draftKey,
                        onValueChange = { draftKey = it },
                        label = { Text("API Key (sk-...)") },
                        singleLine = true,
                        visualTransformation = if (keyVisible) VisualTransformation.None else PasswordVisualTransformation(),
                        trailingIcon = {
                            Text(
                                if (keyVisible) "隐藏" else "显示",
                                color = pc.ink3, fontSize = 11.sp,
                                modifier = Modifier
                                    .clickable { keyVisible = !keyVisible }
                                    .padding(horizontal = 8.dp, vertical = 6.dp),
                            )
                        },
                        colors = textFieldColors(pc),
                        modifier = Modifier.fillMaxWidth(),
                    )

                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        PilottyButton(
                            text = "保存",
                            onClick = {
                                prefs.set(draftBase, draftModel, draftKey)
                                editing = false
                            },
                            variant = PilottyButtonVariant.Primary,
                            small = true,
                            enabled = draftKey.isNotBlank(),
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
                    "保存立即生效 (Go orchestrator 热重载, 无需重启)。 当前明文存储, 公共分发前需升 EncryptedSharedPreferences。",
                    color = pc.ink4, fontSize = 10.5.sp, fontFamily = FontFamily.Monospace,
                )
            }
        }
    }
}

@Serializable
private data class ModelsResp(
    val models: List<String>? = null,
    val error: String? = null,
)

@Composable
private fun InfoLine(label: String, value: String, pc: com.pilotty.app.ui.theme.PilottyColorScheme, mono: Boolean = false) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text(label, color = pc.ink3, fontSize = 12.sp, modifier = Modifier.width(64.dp))
        Text(
            value,
            color = pc.ink,
            fontSize = 12.sp,
            fontFamily = if (mono) FontFamily.Monospace else FontFamily.Default,
            modifier = Modifier.weight(1f),
        )
    }
}

@Composable
private fun textFieldColors(pc: com.pilotty.app.ui.theme.PilottyColorScheme) =
    OutlinedTextFieldDefaults.colors(
        focusedBorderColor = pc.ink,
        unfocusedBorderColor = pc.hairlineStrong,
    )
