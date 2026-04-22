package com.pilotty.app.vpn

import kotlinx.coroutines.channels.BufferOverflow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow

/**
 * ViewModel → MainActivity 的"请启动 VPN"事件总线。
 *
 * 为什么必要: `VpnService.prepare()` 要求 Activity-scoped `ActivityResultLauncher` 处理
 * 系统 consent 对话框, ViewModel 不能直接调。 Nodes 测速若 VPN 未启动, ViewModel 通过这个
 * bus 广播一次 request, MainActivity 里 `collect { onStartVpn() }` 消费, 走现有 VpnController。
 *
 * 用 SharedFlow (非 StateFlow) 避免粘滞: 同一个 request 不应在 Activity 重建时重放。
 */
object StartVpnBus {
    private val _requests = MutableSharedFlow<Unit>(
        extraBufferCapacity = 1,
        onBufferOverflow = BufferOverflow.DROP_OLDEST,
    )
    val requests: SharedFlow<Unit> = _requests.asSharedFlow()

    fun request() {
        _requests.tryEmit(Unit)
    }
}
