package com.pilotty.app.ui.subs

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.pilotty.app.PilottyApp
import com.pilotty.app.PilottyCore
import com.pilotty.app.data.PilottyRepository
import com.pilotty.app.data.SubscriptionDto
import com.pilotty.app.vpn.PilottyVpnService
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class SubsUi(
    val loading: Boolean = false,
    val subscriptions: List<SubscriptionDto> = emptyList(),
    val toast: String? = null,
    val error: String? = null,
)

class SubsViewModel : ViewModel() {
    private val _state = MutableStateFlow(SubsUi())
    val state: StateFlow<SubsUi> = _state.asStateFlow()

    init { refresh() }

    fun refresh() = viewModelScope.launch {
        _state.value = _state.value.copy(loading = true, error = null)
        try {
            _state.value = _state.value.copy(
                loading = false,
                subscriptions = PilottyRepository.subscriptions(),
            )
        } catch (e: Throwable) {
            _state.value = _state.value.copy(loading = false, error = e.message)
        }
    }

    fun addSubscription(name: String, url: String) = viewModelScope.launch {
        val cleanName = name.trim().ifEmpty { url.take(20) }
        val cleanUrl = url.trim()
        if (cleanUrl.isEmpty()) {
            _state.value = _state.value.copy(error = "URL 不能为空")
            return@launch
        }
        // http(s) → 订阅 fetch; 其他 scheme (vmess/ss/vless/...) → 单节点导入
        val isHttpSub = cleanUrl.startsWith("http://", ignoreCase = true) ||
            cleanUrl.startsWith("https://", ignoreCase = true)
        _state.value = _state.value.copy(loading = true, error = null)
        try {
            val r = if (isHttpSub) {
                PilottyRepository.addSubscription(cleanName, cleanUrl)
            } else {
                PilottyRepository.importNodeURI(cleanUrl)
            }
            _state.value = _state.value.copy(
                loading = false,
                toast = r.message.ifEmpty { if (isHttpSub) "订阅已导入" else "节点已导入" },
            )
            refresh()
            requestVpnReloadIfRunning()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(loading = false, error = e.message)
        }
    }

    fun removeSubscription(id: String) = viewModelScope.launch {
        try {
            PilottyRepository.removeSubscription(id)
            refresh()
            requestVpnReloadIfRunning()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(error = e.message)
        }
    }

    /**
     * P1 · SAF 本地文件导入: 用户从文件系统选 .yaml/.json/.txt, Kotlin 读字节 → Go 侧 ParseSubscription
     * 三格式自动探测 (Clash / sing-box JSON / base64-URI list), 落成一条 URL="local://<name>" 的订阅。
     */
    fun importFromData(name: String, data: ByteArray) = viewModelScope.launch {
        if (data.isEmpty()) {
            _state.value = _state.value.copy(error = "文件为空")
            return@launch
        }
        _state.value = _state.value.copy(loading = true, error = null)
        try {
            val r = PilottyRepository.importSubscriptionFromData(name, data)
            _state.value = _state.value.copy(
                loading = false,
                toast = r.message.ifEmpty { "订阅已导入" },
            )
            refresh()
            requestVpnReloadIfRunning()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(loading = false, error = e.message)
        }
    }

    /** M16 Deep Link: 用户点 vmess://... 链接后, Go 侧 ImportNodeURI 把节点写到 overlay。 */
    fun importNodeURI(uri: String) = viewModelScope.launch {
        val cleanUri = uri.trim()
        if (cleanUri.isEmpty()) {
            _state.value = _state.value.copy(error = "URI 不能为空")
            return@launch
        }
        _state.value = _state.value.copy(loading = true, error = null)
        try {
            val r = PilottyRepository.importNodeURI(cleanUri)
            _state.value = _state.value.copy(
                loading = false,
                toast = r.message.ifEmpty { "节点已导入" },
            )
            refresh()
            requestVpnReloadIfRunning()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(loading = false, error = e.message)
        }
    }

    fun updateAll() = viewModelScope.launch {
        _state.value = _state.value.copy(loading = true, error = null)
        try {
            val r = PilottyRepository.updateAllSubscriptions()
            _state.value = _state.value.copy(
                loading = false,
                toast = r.message.ifEmpty { "已更新全部订阅" },
            )
            refresh()
            requestVpnReloadIfRunning()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(loading = false, error = e.message)
        }
    }

    /** 订阅/overlay 变更后若 VPN 运行中则触发 libbox 热重载。未运行时 no-op。 */
    private fun requestVpnReloadIfRunning() {
        if (!PilottyCore.tunRunning.value) return
        PilottyApp.appContext?.let { PilottyVpnService.requestReload(it) }
    }

    fun dismissToast() { _state.value = _state.value.copy(toast = null) }
    fun dismissError() { _state.value = _state.value.copy(error = null) }
}
