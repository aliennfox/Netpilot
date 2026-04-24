package com.pilotty.app.ui.qr

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.graphics.Bitmap
import android.graphics.Color
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.google.zxing.BarcodeFormat
import com.google.zxing.qrcode.QRCodeWriter
import com.pilotty.app.data.NodeUriDto
import com.pilotty.app.data.PilottyRepository
import com.pilotty.app.ui.components.PilottyButton
import com.pilotty.app.ui.components.PilottyButtonVariant
import com.pilotty.app.ui.theme.LocalPilottyColors
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * A2 节点 QR 分享 Dialog。
 * 流程: nodeTag → Go NodeURI(tag) → URI → QRCodeWriter → Bitmap → Image。
 * 失败分两种:
 *   1. 协议不支持 (wg/ssh/naive/anytls/shadowtls): 显示 "暂不支持导出"
 *   2. 节点不存在 / 字段缺失: 显示错误消息
 * 成功时同时给"复制 URI"兜底, 方便粘贴到 TG/邮件。
 */
@Composable
fun NodeQrDialog(
    nodeTag: String,
    onDismiss: () -> Unit,
) {
    val pc = LocalPilottyColors.current
    val ctx = LocalContext.current
    var state by remember { mutableStateOf<QrState>(QrState.Loading) }

    LaunchedEffect(nodeTag) {
        state = try {
            val dto = PilottyRepository.nodeURI(nodeTag)
            val bmp = withContext(Dispatchers.Default) { generateQr(dto.uri, 512) }
            if (bmp == null) QrState.Error("生成 QR 失败") else QrState.Ok(dto, bmp)
        } catch (e: Throwable) {
            QrState.Error(e.message ?: "未知错误")
        }
    }

    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = pc.surface,
        title = {
            Text(
                "分享节点 · $nodeTag",
                color = pc.ink,
                fontSize = 15.sp,
                fontWeight = FontWeight.SemiBold,
            )
        },
        text = {
            Column(
                modifier = Modifier.fillMaxWidth(),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                when (val s = state) {
                    is QrState.Loading -> {
                        Box(
                            modifier = Modifier
                                .size(240.dp)
                                .clip(RoundedCornerShape(12.dp))
                                .background(pc.surface2),
                            contentAlignment = Alignment.Center,
                        ) {
                            Text("生成中...", color = pc.ink3, fontSize = 12.sp)
                        }
                    }
                    is QrState.Ok -> {
                        // QR 放在白底上保证扫码稳定 (dark 主题下深色 QR 贴近背景扫不出)
                        Surface(
                            color = androidx.compose.ui.graphics.Color.White,
                            shape = RoundedCornerShape(12.dp),
                        ) {
                            Image(
                                bitmap = s.bitmap.asImageBitmap(),
                                contentDescription = "Node QR",
                                modifier = Modifier
                                    .padding(8.dp)
                                    .size(224.dp),
                            )
                        }
                        Spacer(Modifier.height(12.dp))
                        Text(
                            s.dto.uri,
                            color = pc.ink3,
                            fontSize = 10.5.sp,
                            fontFamily = FontFamily.Monospace,
                            lineHeight = 14.sp,
                            maxLines = 3,
                        )
                        Spacer(Modifier.height(10.dp))
                        PilottyButton(
                            text = "复制 URI",
                            onClick = { copyToClipboard(ctx, s.dto.uri) },
                            variant = PilottyButtonVariant.Mono,
                            small = true,
                        )
                    }
                    is QrState.Error -> {
                        Text(
                            s.message,
                            color = pc.error,
                            fontSize = 13.sp,
                            lineHeight = 19.sp,
                        )
                    }
                }
            }
        },
        confirmButton = {
            PilottyButton(
                text = "关闭",
                onClick = onDismiss,
                variant = PilottyButtonVariant.Mono,
                small = true,
            )
        },
    )
}

private sealed class QrState {
    object Loading : QrState()
    data class Ok(val dto: NodeUriDto, val bitmap: Bitmap) : QrState()
    data class Error(val message: String) : QrState()
}

private fun generateQr(text: String, size: Int): Bitmap? {
    if (text.isEmpty()) return null
    return try {
        val matrix = QRCodeWriter().encode(text, BarcodeFormat.QR_CODE, size, size)
        val bmp = Bitmap.createBitmap(size, size, Bitmap.Config.ARGB_8888)
        for (x in 0 until size) {
            for (y in 0 until size) {
                bmp.setPixel(x, y, if (matrix[x, y]) Color.BLACK else Color.WHITE)
            }
        }
        bmp
    } catch (_: Throwable) {
        null
    }
}

private fun copyToClipboard(ctx: Context, text: String) {
    val cm = ctx.getSystemService(Context.CLIPBOARD_SERVICE) as? ClipboardManager ?: return
    cm.setPrimaryClip(ClipData.newPlainText("node-uri", text))
}
