package com.pilotty.app.util

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withPermit
import kotlinx.coroutines.withContext
import java.net.InetSocketAddress
import java.net.Socket

/**
 * VPN 未启动时的节点测速回退。 libbox Clash API `/proxies/<tag>/delay` 要求 VpnService 运行,
 * 我们在 Kotlin 直接做 TCP connect probe 作为兜底 —— 精度够"判断节点是否在线",
 * 不保证 TLS handshake 和协议握手成功, 但够自用。 参考 NekoBox 的 URLTest 本地回退路径。
 *
 * 返回毫秒; 失败 / 超时返回 -1 (调用方可据此映射到 UI "--")。
 */
object TcpLatencyProbe {
    suspend fun probe(host: String, port: Int, timeoutMs: Int = 5000): Int = withContext(Dispatchers.IO) {
        if (host.isBlank() || port <= 0) return@withContext -1
        val t0 = System.currentTimeMillis()
        try {
            Socket().use { sock ->
                sock.connect(InetSocketAddress(host, port), timeoutMs)
                (System.currentTimeMillis() - t0).toInt().coerceAtLeast(1)
            }
        } catch (_: Throwable) {
            -1
        }
    }

    /**
     * 并发 probe 多个 (host, port), 最多 [concurrency] 个 socket 在途, 避免 fd 爆。
     * 结果按输入顺序返回, 一一对应。
     */
    suspend fun probeAll(
        targets: List<Pair<String, Int>>,
        timeoutMs: Int = 5000,
        concurrency: Int = 10,
    ): List<Int> = coroutineScope {
        val sem = Semaphore(concurrency)
        targets.map { (host, port) ->
            async { sem.withPermit { probe(host, port, timeoutMs) } }
        }.map { it.await() }
    }
}
