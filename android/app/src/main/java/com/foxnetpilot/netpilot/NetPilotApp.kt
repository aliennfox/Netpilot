package com.foxnetpilot.netpilot

import android.app.Application
import android.content.Context
import android.content.pm.PackageManager
import android.net.ConnectivityManager
import android.util.Log
import libbox.Libbox
import libbox.SetupOptions

class NetPilotApp : Application() {
    override fun onCreate() {
        super.onCreate()

        // sing-box libbox 全局状态初始化 —— 必须在任何 Libbox.newCommandServer / VpnService 之前调用一次
        // 抄自 sing-box-for-android 的 Application.kt:72-82 (2026-04-17)
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

        // API key 通过 BuildConfig 或加密存储注入；先留空让 Agent fallback 到本地路由
        NetPilotCore.init(this, clashAPIAddr = "127.0.0.1:9090", apiKey = "")
    }

    override fun onTerminate() {
        NetPilotCore.shutdown()
        super.onTerminate()
    }

    companion object {
        private const val TAG = "NetPilotApp"

        // 供 NetPilotPlatformInterface 静态访问 (findConnectionOwner 要用)
        @Volatile private var instance: NetPilotApp? = null

        val connectivity: ConnectivityManager?
            get() = instance?.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager

        val packageManager: PackageManager?
            get() = instance?.packageManager
    }

    init {
        @Suppress("LeakingThis")
        instance = this
    }
}
