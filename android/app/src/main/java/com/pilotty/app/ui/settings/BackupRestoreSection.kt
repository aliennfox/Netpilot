package com.pilotty.app.ui.settings

import android.net.Uri
import android.util.Log
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.AlertDialog
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
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.pilotty.app.data.PilottyRepository
import com.pilotty.app.ui.components.Kicker
import com.pilotty.app.ui.components.PilottyButton
import com.pilotty.app.ui.components.PilottyButtonVariant
import com.pilotty.app.ui.components.PilottyCard
import com.pilotty.app.ui.theme.LocalPilottyColors
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * Phase 10-E-D: 配置备份/恢复 UI。
 *
 * Export: SAF CreateDocument("application/json") 让用户选保存位置, 调 ExportBackup Go API
 *         拿 JSON → contentResolver.openOutputStream 写进去
 * Import: SAF OpenDocument 让用户挑文件, 读内容 → ImportBackup Go API 整体覆盖
 *         覆盖前弹二次确认 Dialog (所有现有 overlay + 订阅会被替换)
 *
 * 为什么走 SAF 而不是 App 私有目录: 用户切手机 / 重装 App 时私有目录会清掉, SAF 能选
 * Downloads / 网盘同步目录, 真正做到跨设备迁移。
 */
@Composable
fun BackupRestoreSection() {
    val ctx = LocalContext.current
    val pc = LocalPilottyColors.current
    val scope = rememberCoroutineScope()

    var toast by remember { mutableStateOf<String?>(null) }
    var pendingImportUri by remember { mutableStateOf<Uri?>(null) }

    val exportLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.CreateDocument("application/json"),
    ) { uri ->
        uri ?: return@rememberLauncherForActivityResult
        scope.launch {
            try {
                val data = PilottyRepository.exportBackupRaw()
                withContext(Dispatchers.IO) {
                    ctx.contentResolver.openOutputStream(uri)?.use { os ->
                        os.write(data.toByteArray(Charsets.UTF_8))
                    } ?: throw IllegalStateException("openOutputStream returned null")
                }
                toast = "备份已导出"
            } catch (t: Throwable) {
                Log.w(TAG, "export failed", t)
                toast = "导出失败: ${t.message}"
            }
        }
    }

    val importLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.OpenDocument(),
    ) { uri ->
        uri ?: return@rememberLauncherForActivityResult
        // 二次确认后才真的写覆盖
        pendingImportUri = uri
    }

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Kicker("备份 & 恢复")
        PilottyCard(modifier = Modifier.fillMaxWidth(), soft = true) {
            Column(Modifier.padding(14.dp)) {
                Text("配置导出 / 导入", color = pc.ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
                Spacer(Modifier.height(4.dp))
                Text(
                    "覆盖当前所有订阅 + 路由规则 + rule-set。 推荐存 Nut Cloud / Downloads 目录以便跨手机迁移。",
                    color = pc.ink3,
                    fontSize = 11.sp,
                )
                Spacer(Modifier.height(10.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    PilottyButton(
                        text = "导出",
                        onClick = {
                            val ts = SimpleDateFormat("yyyyMMdd-HHmmss", Locale.US).format(Date())
                            exportLauncher.launch("pilotty-backup-$ts.json")
                        },
                        variant = PilottyButtonVariant.Outline,
                        small = true,
                    )
                    PilottyButton(
                        text = "导入",
                        onClick = {
                            importLauncher.launch(arrayOf("application/json", "*/*"))
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

    pendingImportUri?.let { uri ->
        AlertDialog(
            onDismissRequest = { pendingImportUri = null },
            containerColor = pc.surface,
            title = { Text("确认导入备份", color = pc.ink) },
            text = {
                Text(
                    "当前所有订阅 / 路由规则 / rule-set 将被导入的备份完全替换。 继续?",
                    color = pc.ink2,
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    val toImport = uri
                    pendingImportUri = null
                    scope.launch {
                        try {
                            val data = withContext(Dispatchers.IO) {
                                ctx.contentResolver.openInputStream(toImport)?.use { ins ->
                                    ins.readBytes().toString(Charsets.UTF_8)
                                } ?: throw IllegalStateException("openInputStream null")
                            }
                            val resp = PilottyRepository.importBackup(data)
                            toast = resp.message.ifEmpty { "导入完成" }
                        } catch (t: Throwable) {
                            Log.w(TAG, "import failed", t)
                            toast = "导入失败: ${t.message}"
                        }
                    }
                }) {
                    Text("导入覆盖", color = pc.error, fontWeight = FontWeight.SemiBold)
                }
            },
            dismissButton = {
                TextButton(onClick = { pendingImportUri = null }) {
                    Text("取消", color = pc.ink2)
                }
            },
        )
    }
}

private const val TAG = "BackupRestore"
