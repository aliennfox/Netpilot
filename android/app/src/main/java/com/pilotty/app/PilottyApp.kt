package com.pilotty.app

import android.app.Application
import android.content.Context
import android.content.pm.PackageManager
import android.net.ConnectivityManager
import android.util.Log
import libbox.Libbox
import libbox.SetupOptions
import java.io.File

class PilottyApp : Application() {
    override fun onCreate() {
        super.onCreate()
        instance = this

        try {
            val baseDir = filesDir.apply { mkdirs() }
            val workingDir = (getExternalFilesDir(null) ?: filesDir).apply { mkdirs() }
            val tempDir = cacheDir.apply { mkdirs() }
            Libbox.setup(SetupOptions().apply {
                basePath = baseDir.path
                workingPath = workingDir.path
                tempPath = tempDir.path
                logMaxLines = 3000
            })
            Log.i(TAG, "Libbox.setup ok, base=${baseDir.path} working=${workingDir.path}")
        } catch (t: Throwable) {
            Log.e(TAG, "Libbox.setup failed", t)
        }

        bootstrapConfigAssets()

        PilottyCore.init(this, clashAPIAddr = "127.0.0.1:9090", apiKey = "")
    }

    /**
     * 把 assets/android_tun_base.json 拷到 filesDir/configs/, 首次启动或 asset 更新后生效。
     *
     * 同时写成 `minimal.json` —— Go mobile.NewClient 用 `filesDir/configs/minimal.json` 作 overlay base;
     * 在 Android 上两者内容一致 (都是 tun 版), 这样 Go overlay.MergeConfigs 能正确产出含 tun inbound 的 merged.json。
     * merged.json 由 Go overlay.Apply() 生成; 当其不存在时, VpnService 回退到 minimal.json。
     */
    private fun bootstrapConfigAssets() {
        val configsDir = File(filesDir, "configs").apply { mkdirs() }
        runCatching {
            val bytes = assets.open(TUN_BASE_NAME).use { it.readBytes() }
            for (name in arrayOf(TUN_BASE_NAME, MINIMAL_NAME)) {
                val dst = File(configsDir, name)
                dst.outputStream().use { it.write(bytes) }
                Log.i(TAG, "asset copied: ${dst.path} (${dst.length()} bytes)")
            }
        }.onFailure {
            Log.e(TAG, "asset copy failed: $TUN_BASE_NAME", it)
        }
    }

    override fun onTerminate() {
        PilottyCore.shutdown()
        super.onTerminate()
    }

    companion object {
        private const val TAG = "PilottyApp"
        const val TUN_BASE_NAME = "android_tun_base.json"
        const val MINIMAL_NAME = "minimal.json"

        @Volatile private var instance: PilottyApp? = null

        val connectivity: ConnectivityManager?
            get() = instance?.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager

        val packageManager: PackageManager?
            get() = instance?.packageManager

        val appContext: Context? get() = instance
    }
}
