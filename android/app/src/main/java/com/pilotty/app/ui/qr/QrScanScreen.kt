package com.pilotty.app.ui.qr

import android.Manifest
import android.content.pm.PackageManager
import android.util.Log
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import com.google.zxing.BinaryBitmap
import com.google.zxing.MultiFormatReader
import com.google.zxing.NotFoundException
import com.google.zxing.PlanarYUVLuminanceSource
import com.google.zxing.common.HybridBinarizer
import com.pilotty.app.ui.components.PilottyButton
import com.pilotty.app.ui.components.PilottyButtonVariant
import com.pilotty.app.ui.theme.LocalPilottyColors
import java.util.concurrent.Executors

/**
 * Phase 10-E-C: QR 扫码入口。
 *
 * 实现: CameraX (Preview + ImageAnalysis) + ZXing MultiFormatReader。 不用 ML Kit,
 * 因为 ML Kit barcode-scanning 的 unbundled 版依赖 Google Play Services, 国内 ROM
 * 上可能缺 GMS → 扫码不可用。 ZXing 是纯 Java, 体积 ~500KB, 无外部服务依赖。
 *
 * 流程: 授权 → 启动相机 → Analyzer 每帧 decode → 成功 → onResult(text) → nav back;
 * 用户取消或拒绝权限 → 空手返回。 调用方根据结果决定是 ImportNodeURI 还是 http 订阅。
 */
@Composable
fun QrScanScreen(onBack: () -> Unit, onResult: (String) -> Unit) {
    val pc = LocalPilottyColors.current
    val ctx = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current

    var hasPermission by remember {
        mutableStateOf(
            ContextCompat.checkSelfPermission(ctx, Manifest.permission.CAMERA)
                == PackageManager.PERMISSION_GRANTED,
        )
    }
    val permLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        hasPermission = granted
        if (!granted) onBack()
    }
    LaunchedEffect(Unit) {
        if (!hasPermission) {
            permLauncher.launch(Manifest.permission.CAMERA)
        }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                "←",
                color = Color.White,
                fontSize = 22.sp,
                fontWeight = FontWeight.Bold,
                modifier = Modifier
                    .clickable { onBack() }
                    .padding(end = 12.dp),
            )
            Text(
                "扫描二维码",
                color = Color.White,
                fontSize = 18.sp,
                fontWeight = FontWeight.SemiBold,
            )
        }

        if (!hasPermission) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(
                        "需要相机权限才能扫码",
                        color = Color.White,
                        fontSize = 14.sp,
                    )
                    Spacer(Modifier.height(12.dp))
                    PilottyButton(
                        text = "授予权限",
                        onClick = { permLauncher.launch(Manifest.permission.CAMERA) },
                        variant = PilottyButtonVariant.Outline,
                        small = true,
                    )
                }
            }
            return@Column
        }

        // 相机预览
        Box(modifier = Modifier.fillMaxSize()) {
            AndroidView(
                factory = { c ->
                    PreviewView(c).apply {
                        val future = ProcessCameraProvider.getInstance(c)
                        future.addListener({
                            val provider = future.get()
                            val preview = Preview.Builder().build().also {
                                it.setSurfaceProvider(surfaceProvider)
                            }
                            val analyzer = QrAnalyzer { decoded ->
                                if (decoded.isNotBlank()) {
                                    provider.unbindAll()
                                    onResult(decoded)
                                }
                            }
                            val analysis = ImageAnalysis.Builder()
                                .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                                .build()
                                .also {
                                    it.setAnalyzer(
                                        Executors.newSingleThreadExecutor(),
                                        analyzer,
                                    )
                                }
                            runCatching {
                                provider.unbindAll()
                                provider.bindToLifecycle(
                                    lifecycleOwner,
                                    CameraSelector.DEFAULT_BACK_CAMERA,
                                    preview,
                                    analysis,
                                )
                            }.onFailure {
                                Log.e(TAG, "bindToLifecycle failed", it)
                            }
                        }, ContextCompat.getMainExecutor(c))
                    }
                },
                modifier = Modifier.fillMaxSize(),
            )
            // 中心取景框提示
            Box(
                modifier = Modifier
                    .align(Alignment.Center)
                    .fillMaxWidth(0.7f)
                    .height(280.dp)
                    .background(Color.Transparent),
            )
            Text(
                "把二维码对准框内",
                color = Color.White,
                fontSize = 13.sp,
                fontFamily = FontFamily.Monospace,
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .padding(bottom = 48.dp),
            )
        }
    }

    DisposableEffect(Unit) {
        onDispose {
            runCatching {
                ProcessCameraProvider.getInstance(ctx).get().unbindAll()
            }
        }
    }
}

/**
 * QrAnalyzer: 每帧 CameraX ImageProxy (YUV_420_888) 通过 ZXing decode。
 * Backpressure=KEEP_ONLY_LATEST 保证忙不过来时不堆积, 单线程 executor 避免并发 decode。
 */
private class QrAnalyzer(
    private val onDecoded: (String) -> Unit,
) : ImageAnalysis.Analyzer {
    private val reader = MultiFormatReader()
    @Volatile private var consumed = false

    override fun analyze(image: ImageProxy) {
        if (consumed) {
            image.close()
            return
        }
        try {
            val plane = image.planes.getOrNull(0) ?: return
            val buffer = plane.buffer
            val bytes = ByteArray(buffer.remaining())
            buffer.get(bytes)
            val source = PlanarYUVLuminanceSource(
                bytes,
                image.width,
                image.height,
                0,
                0,
                image.width,
                image.height,
                false,
            )
            val bitmap = BinaryBitmap(HybridBinarizer(source))
            val result = runCatching { reader.decodeWithState(bitmap) }.getOrNull()
            if (result != null) {
                val text = result.text
                if (!text.isNullOrBlank()) {
                    consumed = true
                    onDecoded(text)
                }
            }
        } catch (_: NotFoundException) {
            // 未识别到二维码, 继续下一帧
        } catch (t: Throwable) {
            Log.w(TAG, "decode failed", t)
        } finally {
            image.close()
        }
    }
}

private const val TAG = "QrScan"
