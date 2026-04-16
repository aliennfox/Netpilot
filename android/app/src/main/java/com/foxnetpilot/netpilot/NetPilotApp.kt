package com.foxnetpilot.netpilot

import android.app.Application

class NetPilotApp : Application() {
    override fun onCreate() {
        super.onCreate()
        // API key 通过 BuildConfig 或加密存储注入；先留空让 Agent fallback 到本地路由
        NetPilotCore.init(this, clashAPIAddr = "127.0.0.1:9090", apiKey = "")
    }

    override fun onTerminate() {
        NetPilotCore.shutdown()
        super.onTerminate()
    }
}
