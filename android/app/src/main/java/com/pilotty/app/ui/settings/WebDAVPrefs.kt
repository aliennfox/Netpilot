package com.pilotty.app.ui.settings

import android.content.Context
import android.content.SharedPreferences

/**
 * WebDAV 连接配置持久化 (Phase 10-E-E)。
 *
 * ⚠️ 安全债同 ApiKeyPrefs: 明文存密码。 v1.x 必须改 EncryptedSharedPreferences。
 * 已在 docs/android-mvp-gap.md 标注。 自用场景可接受。
 */
class WebDAVPrefs private constructor(private val sp: SharedPreferences) {

    var url: String
        get() = sp.getString(KEY_URL, "").orEmpty()
        set(v) { sp.edit().putString(KEY_URL, v).apply() }

    var user: String
        get() = sp.getString(KEY_USER, "").orEmpty()
        set(v) { sp.edit().putString(KEY_USER, v).apply() }

    var password: String
        get() = sp.getString(KEY_PASS, "").orEmpty()
        set(v) { sp.edit().putString(KEY_PASS, v).apply() }

    var path: String
        get() = sp.getString(KEY_PATH, "pilotty-backup.json").orEmpty()
        set(v) { sp.edit().putString(KEY_PATH, v.ifBlank { "pilotty-backup.json" }).apply() }

    /** 快捷脱敏展示 (用户 + 密码脱掉): "https://dav.jianguoyun.com/dav/ · user@***" */
    fun summary(): String {
        val u = url
        if (u.isBlank()) return "未配置"
        val userPart = if (user.isBlank()) "?" else user.take(4) + (if (user.length > 4) "***" else "")
        return "$u · $userPart"
    }

    fun isConfigured(): Boolean = url.isNotBlank() && user.isNotBlank() && password.isNotBlank()

    companion object {
        private const val FILE = "webdav"
        private const val KEY_URL = "url"
        private const val KEY_USER = "user"
        private const val KEY_PASS = "pass"
        private const val KEY_PATH = "path"

        @Volatile private var instance: WebDAVPrefs? = null

        fun get(ctx: Context): WebDAVPrefs {
            instance?.let { return it }
            return synchronized(this) {
                instance ?: WebDAVPrefs(
                    ctx.applicationContext.getSharedPreferences(FILE, Context.MODE_PRIVATE),
                ).also { instance = it }
            }
        }
    }
}
