package com.pilotty.app.perapp

import android.content.Context
import android.content.SharedPreferences
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

/**
 * Per-App VPN 偏好设置 (M14)。
 *
 * 存两件事:
 *   1. 模式 (Off / Allow / Deny) —— 对应 libbox TunOptions.include_package(白)/exclude_package(黑)
 *   2. 选中的 package 名集合
 *
 * 不需要加密: package 列表不是秘密, 且 EncryptedSharedPreferences 会拉 androidx.security
 * 依赖, 本轮避开。 LLM API Key (M8) 才上 EncryptedSharedPreferences。
 */
class PerAppVpnPrefs(ctx: Context) {

    enum class Mode(val raw: String) {
        Off("off"),     // 全局代理,忽略 package 列表
        Allow("allow"), // 白名单: 只代理选中的 App
        Deny("deny");   // 黑名单: 代理除选中外的所有 App

        companion object {
            fun fromRaw(s: String?): Mode = values().firstOrNull { it.raw == s } ?: Off
        }
    }

    private val sp: SharedPreferences =
        ctx.applicationContext.getSharedPreferences(FILE, Context.MODE_PRIVATE)

    private val _state = MutableStateFlow(currentSnapshot())
    val state: StateFlow<Snapshot> = _state

    data class Snapshot(val mode: Mode, val packages: Set<String>)

    fun mode(): Mode = Mode.fromRaw(sp.getString(KEY_MODE, Mode.Off.raw))

    fun packages(): Set<String> = sp.getStringSet(KEY_PKGS, emptySet()) ?: emptySet()

    fun setMode(mode: Mode) {
        sp.edit().putString(KEY_MODE, mode.raw).apply()
        _state.value = currentSnapshot()
    }

    fun setPackages(pkgs: Set<String>) {
        // SharedPreferences getStringSet 对同一 Set 实例有 mutation 风险, 显式拷贝
        sp.edit().putStringSet(KEY_PKGS, pkgs.toHashSet()).apply()
        _state.value = currentSnapshot()
    }

    private fun currentSnapshot() = Snapshot(mode(), packages())

    companion object {
        private const val FILE = "perapp_vpn"
        private const val KEY_MODE = "mode"
        private const val KEY_PKGS = "packages"

        @Volatile private var INSTANCE: PerAppVpnPrefs? = null

        fun get(ctx: Context): PerAppVpnPrefs =
            INSTANCE ?: synchronized(this) {
                INSTANCE ?: PerAppVpnPrefs(ctx).also { INSTANCE = it }
            }
    }
}
