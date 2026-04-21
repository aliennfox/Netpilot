package com.pilotty.app.perapp

import android.content.Context
import android.content.pm.ApplicationInfo
import android.content.pm.PackageManager
import android.graphics.drawable.Drawable
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * 扫描已安装 App, 产出可选列表 (M14)。 主线程不跑, 所有调用必须在 IO 上下文。
 */
object InstalledAppsLoader {

    data class Entry(
        val packageName: String,
        val label: String,
        val icon: Drawable,
        val isSystem: Boolean,
    )

    /**
     * 返回排序后的 App 列表。
     * 排序: 用户 App 在前 (按 label 字母序), 系统 App 在后 (按 label 字母序)。
     * 自家 package 从结果里剔除: VpnService 必须始终排除自己, 用户选它没意义。
     */
    suspend fun load(ctx: Context): List<Entry> = withContext(Dispatchers.IO) {
        val pm = ctx.packageManager
        val selfPkg = ctx.packageName
        val flags = PackageManager.GET_META_DATA
        val apps = runCatching {
            @Suppress("DEPRECATION")
            pm.getInstalledApplications(flags)
        }.getOrElse { emptyList() }

        apps.asSequence()
            .filter { it.packageName != selfPkg }
            .filter { canLaunch(pm, it) }
            .map { info ->
                Entry(
                    packageName = info.packageName,
                    label = runCatching { pm.getApplicationLabel(info).toString() }.getOrDefault(info.packageName),
                    icon = runCatching { pm.getApplicationIcon(info) }.getOrElse { pm.defaultActivityIcon },
                    isSystem = (info.flags and ApplicationInfo.FLAG_SYSTEM) != 0,
                )
            }
            .sortedWith(compareBy({ it.isSystem }, { it.label.lowercase() }))
            .toList()
    }

    /**
     * 过滤掉"完全无 UI 的系统 package" (纯服务 / provider 等)。
     * 用户看到它们也没意义, 且数量庞大会拖慢首次加载。
     * 保留有 launcher intent 或用户可见的系统应用 (比如浏览器, 设置, Play 商店)。
     */
    private fun canLaunch(pm: PackageManager, info: ApplicationInfo): Boolean {
        if ((info.flags and ApplicationInfo.FLAG_SYSTEM) == 0) return true
        val launch = runCatching { pm.getLaunchIntentForPackage(info.packageName) }.getOrNull()
        return launch != null
    }
}
