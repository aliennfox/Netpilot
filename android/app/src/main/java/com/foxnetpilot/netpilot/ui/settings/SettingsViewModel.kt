package com.foxnetpilot.netpilot.ui.settings

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.foxnetpilot.netpilot.NetPilotApp
import com.foxnetpilot.netpilot.NetPilotCore
import com.foxnetpilot.netpilot.data.NetPilotRepository
import com.foxnetpilot.netpilot.data.SubscriptionDto
import com.foxnetpilot.netpilot.vpn.NetPilotVpnService
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class SettingsUi(
    val loading: Boolean = false,
    val subscriptions: List<SubscriptionDto> = emptyList(),
    val toast: String? = null,
    val error: String? = null,
)

class SettingsViewModel : ViewModel() {
    private val _state = MutableStateFlow(SettingsUi())
    val state: StateFlow<SettingsUi> = _state.asStateFlow()

    init { refresh() }

    fun refresh() = viewModelScope.launch {
        _state.value = _state.value.copy(loading = true, error = null)
        try {
            _state.value = _state.value.copy(
                loading = false,
                subscriptions = NetPilotRepository.subscriptions(),
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
        _state.value = _state.value.copy(loading = true, error = null)
        try {
            val r = NetPilotRepository.addSubscription(cleanName, cleanUrl)
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

    fun removeSubscription(id: String) = viewModelScope.launch {
        try {
            NetPilotRepository.removeSubscription(id)
            refresh()
            requestVpnReloadIfRunning()
        } catch (e: Throwable) {
            _state.value = _state.value.copy(error = e.message)
        }
    }

    fun updateAll() = viewModelScope.launch {
        _state.value = _state.value.copy(loading = true, error = null)
        try {
            val r = NetPilotRepository.updateAllSubscriptions()
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
        if (!NetPilotCore.tunRunning.value) return
        NetPilotApp.appContext?.let { NetPilotVpnService.requestReload(it) }
    }

    fun dismissToast() { _state.value = _state.value.copy(toast = null) }
    fun dismissError() { _state.value = _state.value.copy(error = null) }
}
