package com.pilotty.app.ui.settings

import android.util.Log
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.data.PilottyRepository
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.PilottyButton
import com.pilotty.app.ui.components.PilottyButtonVariant
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.theme.LocalPilottyColors
import kotlinx.coroutines.launch

/**
 * Phase 10-E-E: WebDAV 同步。
 *
 * UI 心智:
 *  - 配置卡展示当前 state + "配置" / "测试" / "上传" / "下载" 四个动作
 *  - 点"配置"弹 Dialog 编辑 URL / user / password / path, 明文输入密码可 toggle
 *  - 测试: 调 webDAVTest → HEAD baseURL → 200 / 404 / 405 都算认证通过
 *  - 上传: 调 webDAVPush → Go 侧 Export + PUT, 无需 UI 再 Export 一遍
 *  - 下载: 弹二次确认 (同 BackupRestore, 会覆盖当前 overlay + 订阅)
 *
 * 国内用户路径: 默认填 https://dav.jianguoyun.com/dav/ + 坚果云账号密码 + 路径 pilotty-backup.json
 * 参考 Karing 的 webdav 集成 —— 这是 Karing 在 "跨设备同步" 方向上的最大卖点, 我们这里追平。
 */
@Composable
fun WebDAVSyncSection() {
    val ctx = LocalContext.current
    val pc = LocalPilottyColors.current
    val scope = rememberCoroutineScope()
    val prefs = remember { WebDAVPrefs.get(ctx) }

    var editOpen by remember { mutableStateOf(false) }
    var confirmPullOpen by remember { mutableStateOf(false) }
    var toast by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }

    // 把 prefs 读出来作为 UI state, edit 保存时再回写
    var url by remember { mutableStateOf(prefs.url) }
    var user by remember { mutableStateOf(prefs.user) }
    var pass by remember { mutableStateOf(prefs.password) }
    var path by remember { mutableStateOf(prefs.path) }

    fun runAction(label: String, action: suspend () -> String) {
        busy = true
        scope.launch {
            try {
                val msg = action()
                toast = "$label: $msg"
            } catch (t: Throwable) {
                Log.w(TAG, "$label failed", t)
                toast = "$label 失败: ${t.message}"
            } finally {
                busy = false
            }
        }
    }

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Kicker("WebDAV 同步")
        PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
            Column(Modifier.padding(14.dp)) {
                Text(
                    "WebDAV 云备份",
                    color = pc.ink,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                )
                Spacer(Modifier.height(2.dp))
                Text(
                    if (prefs.isConfigured()) prefs.summary() else "未配置 · 支持坚果云 / AList / Nextcloud",
                    color = pc.ink3,
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                )
                Spacer(Modifier.height(10.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    PilottyButton(
                        text = "配置",
                        onClick = {
                            url = prefs.url
                            user = prefs.user
                            pass = prefs.password
                            path = prefs.path
                            editOpen = true
                        },
                        variant = PilottyButtonVariant.Outline,
                        small = true,
                    )
                    PilottyButton(
                        text = "测试",
                        onClick = {
                            if (!prefs.isConfigured()) {
                                toast = "请先配置 URL / 用户 / 密码"; return@PilottyButton
                            }
                            runAction("测试") {
                                PilottyRepository.webDAVTest(prefs.url, prefs.user, prefs.password).message
                            }
                        },
                        variant = PilottyButtonVariant.Outline,
                        small = true,
                    )
                    PilottyButton(
                        text = "上传",
                        onClick = {
                            if (!prefs.isConfigured()) {
                                toast = "请先配置"; return@PilottyButton
                            }
                            runAction("上传") {
                                PilottyRepository.webDAVPush(prefs.url, prefs.user, prefs.password, prefs.path).message
                            }
                        },
                        variant = PilottyButtonVariant.Outline,
                        small = true,
                    )
                    PilottyButton(
                        text = "下载",
                        onClick = {
                            if (!prefs.isConfigured()) {
                                toast = "请先配置"; return@PilottyButton
                            }
                            confirmPullOpen = true
                        },
                        variant = PilottyButtonVariant.Outline,
                        small = true,
                    )
                }
                toast?.let {
                    Spacer(Modifier.height(6.dp))
                    Text(
                        "· $it",
                        color = pc.ink3,
                        fontSize = 11.sp,
                        fontFamily = FontFamily.Monospace,
                    )
                }
            }
        }
    }

    if (editOpen) {
        var showPass by remember { mutableStateOf(false) }
        AlertDialog(
            onDismissRequest = { editOpen = false },
            containerColor = pc.surface,
            title = { Text("WebDAV 配置", color = pc.ink) },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedTextField(
                        value = url,
                        onValueChange = { url = it },
                        label = { Text("服务器 URL") },
                        placeholder = { Text("https://dav.jianguoyun.com/dav/") },
                        singleLine = true,
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedBorderColor = pc.ink,
                            unfocusedBorderColor = pc.hairlineStrong,
                        ),
                    )
                    OutlinedTextField(
                        value = user,
                        onValueChange = { user = it },
                        label = { Text("用户名") },
                        singleLine = true,
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedBorderColor = pc.ink,
                            unfocusedBorderColor = pc.hairlineStrong,
                        ),
                    )
                    OutlinedTextField(
                        value = pass,
                        onValueChange = { pass = it },
                        label = { Text("密码 / 应用密码") },
                        singleLine = true,
                        visualTransformation = if (showPass) VisualTransformation.None else PasswordVisualTransformation(),
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedBorderColor = pc.ink,
                            unfocusedBorderColor = pc.hairlineStrong,
                        ),
                    )
                    Row {
                        TextButton(onClick = { showPass = !showPass }) {
                            Text(if (showPass) "隐藏密码" else "显示密码", color = pc.ink3, fontSize = 11.sp)
                        }
                    }
                    OutlinedTextField(
                        value = path,
                        onValueChange = { path = it },
                        label = { Text("路径 (相对根目录)") },
                        placeholder = { Text("pilotty-backup.json") },
                        singleLine = true,
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedBorderColor = pc.ink,
                            unfocusedBorderColor = pc.hairlineStrong,
                        ),
                    )
                    Text(
                        "坚果云须在账户里生成\"应用密码\"而非主密码。 路径若含子目录, 云端需提前创建好。",
                        color = pc.ink4,
                        fontSize = 10.5.sp,
                    )
                }
            },
            confirmButton = {
                TextButton(onClick = {
                    prefs.url = url.trim()
                    prefs.user = user.trim()
                    prefs.password = pass
                    prefs.path = path.trim()
                    editOpen = false
                    toast = "配置已保存"
                }) { Text("保存", color = pc.accentInk, fontWeight = FontWeight.SemiBold) }
            },
            dismissButton = {
                TextButton(onClick = { editOpen = false }) { Text("取消", color = pc.ink2) }
            },
        )
    }

    if (confirmPullOpen) {
        AlertDialog(
            onDismissRequest = { confirmPullOpen = false },
            containerColor = pc.surface,
            title = { Text("下载并覆盖配置", color = pc.ink) },
            text = {
                Text(
                    "从 WebDAV ${prefs.path} 拉备份并覆盖当前所有订阅 + 路由规则 + rule-set。 继续?",
                    color = pc.ink2,
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    confirmPullOpen = false
                    runAction("下载") {
                        PilottyRepository.webDAVPull(prefs.url, prefs.user, prefs.password, prefs.path).message
                    }
                }) { Text("下载覆盖", color = pc.error, fontWeight = FontWeight.SemiBold) }
            },
            dismissButton = {
                TextButton(onClick = { confirmPullOpen = false }) {
                    Text("取消", color = pc.ink2)
                }
            },
        )
    }
}

private const val TAG = "WebDAVSync"
