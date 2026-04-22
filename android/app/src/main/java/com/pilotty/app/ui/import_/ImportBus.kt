package com.pilotty.app.ui.import_

import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow

/**
 * Deep Link / 剪贴板导入的单进程广播。 MainActivity 收到 intent.data 后,
 * 把 URI 投到 [pending],Subs / Home 里的任意 Composable 可以 collect 消费它。
 *
 * 为什么不用 Activity result / nav deep link:
 *  - 我们想在"任意当前 tab"弹出确认 dialog,不绑死某条路由
 *  - Activity 在 onNewIntent 时上层 Compose 树已渲染,用 shared flow 最干净
 *
 * 为什么 StateFlow<Item?> 而不是 SharedFlow<Item>:
 *  - UI 层可能在 URI 到达时还没订阅(比如用户点 vmess:// 前 App 已被杀)
 *  - 用 StateFlow 保留 "最近一个未消费" 的 URI, Composable 起来后读到 → 处理 → 调 consume() 清空
 */
object ImportBus {
    data class Item(val uri: String)

    private val _pending = MutableStateFlow<Item?>(null)
    val pending: StateFlow<Item?> = _pending.asStateFlow()

    fun post(uri: String) {
        _pending.value = Item(uri)
    }

    fun consume() {
        _pending.value = null
    }
}
