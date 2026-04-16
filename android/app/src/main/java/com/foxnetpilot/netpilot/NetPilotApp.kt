package com.foxnetpilot.netpilot

import android.app.Application
import android.content.Context
import android.content.pm.PackageManager
import android.net.ConnectivityManager
import android.util.Log
import libbox.Libbox
import libbox.SetupOptions
import java.io.File

class NetPilotApp : Application() {
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

        NetPilotCore.init(this, clashAPIAddr = "127.0.0.1:9090", apiKey = "")
    }

    /**
     * 把 assets/android_tun_base.json 拷到 filesDir/configs/, 首次启动或 asset 更新后生效。
     * merged.json 由 Go overlay.Apply() 生成; 当其不存在时, VpnService 回退到这份基础 tun config。
     */
    private fun bootstrapConfigAssets() {
        val configsDir = File(filesDir, "configs").apply { mkdirs() }
        val dst = File(configsDir, TUN_BASE_NAME)
        runCatching {
            assets.open(TUN_BASE_NAME).use { input ->
                dst.outputStream().use { input.copyTo(it) }
            }
            Log.i(TAG, "asset copied: ${dst.path} (${dst.length()} bytes)")
        }.onFailure {
            Log.e(TAG, "asset copy failed: $TUN_BASE_NAME", it)
        }
    }

    override fun onTerminate() {
        NetPilotCore.shutdown()
        super.onTerminate()
    }

    companion object {
        private const val TAG = "NetPilotApp"
        const val TUN_BASE_NAME = "android_tun_base.json"

        @Volatile private var instance: NetPilotApp? = null

        val connectivity: ConnectivityManager?
            get() = instance?.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager

        val packageManager: PackageManager?
            get() = instance?.packageManager
    }
}
