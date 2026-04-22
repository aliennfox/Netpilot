package com.pilotty.app.ui.agent

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/**
 * Home "Ask the agent" 输入 / 快捷 chip → Chat tab 的单进程广播。
 *
 * 设计同 [com.pilotty.app.ui.import_.ImportBus]:
 *  - Home 点 ↑ / chip 后 `post(query)`,然后 nav 到 Chat
 *  - ChatScreen 起来时 collect 到 pending,调用 vm.send(query) 并 consume()
 *  - 中断路径(用户切 tab 回来又切走): Compose lifecycle 会重新触发 collect,但 consume 确保只处理一次
 *
 * 这是 D1 "自然语言 Dashboard 入口" 最小闭环:让 Home 上的 hero input 真能落到 Agent pipeline,
 * 而不只是跳 tab 让用户再打一遍字。
 */
object AgentQueryBus {
    private val _pending = MutableStateFlow<String?>(null)
    val pending: StateFlow<String?> = _pending.asStateFlow()

    fun post(query: String) {
        val q = query.trim()
        if (q.isNotEmpty()) _pending.value = q
    }

    fun consume() {
        _pending.value = null
    }
}
