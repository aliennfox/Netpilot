package com.pilotty.app.vpn

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.net.VpnService
import androidx.activity.result.ActivityResultLauncher
import androidx.activity.result.contract.ActivityResultContracts

/**
 * VPN 启停辅助：
 *  - [registerLauncher] 在 Activity 创建时调用一次，注册系统授权回调
 *  - [start] 检查授权 → 启动 VpnService；未授权时弹系统对话框
 *  - [stop] 通过 ACTION_STOP intent 通知 service 自行结束
 */
class VpnController(private val activity: Activity) {
    private lateinit var launcher: ActivityResultLauncher<Intent>

    fun registerLauncher() {
        launcher = (activity as androidx.activity.ComponentActivity)
            .registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
                if (result.resultCode == Activity.RESULT_OK) startServiceInternal()
            }
    }

    fun start() {
        val prepare = VpnService.prepare(activity)
        if (prepare != null) launcher.launch(prepare) else startServiceInternal()
    }

    fun stop() {
        val intent = Intent(activity, PilottyVpnService::class.java).apply {
            action = PilottyVpnService.ACTION_STOP
        }
        activity.startService(intent)
    }

    private fun startServiceInternal() {
        val intent = Intent(activity, PilottyVpnService::class.java)
        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.O) {
            activity.startForegroundService(intent)
        } else {
            activity.startService(intent)
        }
    }
}
