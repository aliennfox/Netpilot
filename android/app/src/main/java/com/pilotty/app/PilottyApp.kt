package com.pilotty.app

import android.app.Application
import android.content.Context
import android.content.pm.PackageManager
import android.net.ConnectivityManager
import android.util.Log
import com.pilotty.app.agent.LLMConfigPrefs
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

        // 三件套配置: LLMConfigPrefs.get 内部会自动从老 ApiKeyPrefs 迁移单字段 apiKey, 无需手动处理。
        val cfg = LLMConfigPrefs.get(this).state.value
        Log.i(TAG, "PilottyCore.init apiKey=${if (cfg.apiKey.isBlank()) "<empty>" else "sk-***${cfg.apiKey.takeLast(4)}"} baseURL=${cfg.baseURL.ifBlank { "<default>" }} model=${cfg.model.ifBlank { "<default>" }}")
        PilottyCore.init(this, clashAPIAddr = "127.0.0.1:9090", apiKey = cfg.apiKey)
        // 把 baseURL/model 推给 Go (apiKey 已通过 init 传过)。 init 后调以保证 Client 已构造。
        if (cfg.baseURL.isNotBlank() || cfg.model.isNotBlank()) {
            runCatching { PilottyCore.setLLMConfig(cfg.baseURL, cfg.model, cfg.apiKey) }
                .onFailure { Log.w(TAG, "setLLMConfig at boot failed", it) }
        }
        // Phase 8: 让 Agent 的 start_vpn/stop_vpn/vpn_status tools 能驱动 VpnService。
        // 必须在 PilottyCore.init 之后, 这样 client 已构造好, setVpnControl 才有 receiver。
        PilottyCore.setVpnControl(com.pilotty.app.vpn.VpnControlImpl)
        // D3: Agent 写 overlay 后 Go 侧通过 reloader 让 PilottyVpnService 热加载 sing-box config.
        PilottyCore.setPlatformReloader(com.pilotty.app.vpn.PlatformReloaderImpl)
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
