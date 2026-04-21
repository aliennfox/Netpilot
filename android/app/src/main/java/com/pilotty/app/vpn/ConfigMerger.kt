package com.pilotty.app.vpn

import android.util.Log
import com.pilotty.app.perapp.PerAppVpnPrefs
import org.json.JSONArray
import org.json.JSONObject

/**
 * 把 Go 侧 overlay.Apply() 生成的 `merged.json` 与 Android 基础 tun 配置 (`android_tun_base.json`)
 * 再合并一次,确保关键的 tun inbound / route / dns 不缺。
 *
 * 动机 (Known Issue #H4): Go 侧 overlay 的 base 是 CLI 用的 minimal.json (mixed inbound),
 * 合出来的 merged.json 没有 tun inbound, libbox 永不会回调 openTun。
 * 选"Kotlin 二次合并"而非改 Go overlay 层是为了不触发 aar 重建,避开 #M8 同步纪律问题。
 *
 * sing-box 1.13 schema:
 *  - 移除了 `outbounds[type=dns]`, 改用 route rule `{action:"hijack-dns"}`
 *  - 移除了 `outbounds[type=block]`, 改用 route rule `{action:"reject"}`
 *  - 移除了 inbound 级 `sniff:true`, 改用 route rule `{action:"sniff"}`
 *
 * 合并策略:
 *  - inbounds: 若无 type=tun, 从 base 拷一份 tun inbound 追加到末尾
 *  - route.auto_detect_interface: 若 merged 未设, 取 base 值
 *  - route.default_domain_resolver: 若 merged 未设, 取 base 值 (sing-box 1.12+ 必需)
 *  - route.rules 系统规则 (action=sniff / action=hijack-dns / ip_is_private direct) 确保存在
 *    - 若 merged 的首条不是 sniff, 把 base 的系统规则 prepend 到 merged.rules 前
 *    - **必须 prepend**, 否则用户规则 "抖音 direct" 会拦截 DNS 流量, DNS 永远不到 hijack-dns
 *  - dns: 若 merged 无 dns 字段, 整段拷 base
 */
object ConfigMerger {
    private const val TAG = "ConfigMerger"

    fun ensureTunInbound(mergedJson: String, tunBaseJson: String): String = try {
        val m = JSONObject(mergedJson)
        val b = JSONObject(tunBaseJson)

        mergeInbounds(m, b)
        mergeRoute(m, b)
        mergeDns(m, b)

        m.toString()
    } catch (t: Throwable) {
        Log.w(TAG, "merge failed, using merged.json as-is", t)
        mergedJson
    }

    /**
     * 把 Per-App VPN 偏好 (M14) 写到 tun inbound 的 include_package / exclude_package 字段。
     *
     * 说明:
     *  - Off 模式 → 清掉两个字段 (全量代理)
     *  - Allow(白名单) → 写 include_package = [...], 清空 exclude_package
     *  - Deny(黑名单) → 写 exclude_package = [...], 清空 include_package
     *
     * libbox 走 sing-box 配置 → TunOptions → VpnService.Builder.addAllowedApplication
     * /addDisallowedApplication 兜底。 `excludePackage` 里不加 "自家 package" 由
     * `PilottyVpnService.openTun` 兜底处理 (行 97-98 硬写 packageName), 保证 TUN 流量
     * 不回环。
     */
    fun injectPerAppRules(configJson: String, snapshot: PerAppVpnPrefs.Snapshot): String {
        return try {
            val root = JSONObject(configJson)
            val inbounds = root.optJSONArray("inbounds") ?: return configJson
            var tun: JSONObject? = null
            for (i in 0 until inbounds.length()) {
                val inb = inbounds.optJSONObject(i) ?: continue
                if (inb.optString("type") == "tun") {
                    tun = inb
                    break
                }
            }
            if (tun == null) {
                Log.w(TAG, "injectPerAppRules: no tun inbound, per-app 规则写不进去")
                return configJson
            }

            tun.remove("include_package")
            tun.remove("exclude_package")
            when (snapshot.mode) {
                PerAppVpnPrefs.Mode.Off -> {
                    // 两个字段都已 remove, 完事
                }
                PerAppVpnPrefs.Mode.Allow -> if (snapshot.packages.isNotEmpty()) {
                    tun.put("include_package", JSONArray(snapshot.packages.toList()))
                }
                PerAppVpnPrefs.Mode.Deny -> if (snapshot.packages.isNotEmpty()) {
                    tun.put("exclude_package", JSONArray(snapshot.packages.toList()))
                }
            }
            Log.i(TAG, "injectPerAppRules mode=${snapshot.mode} pkgs=${snapshot.packages.size}")
            root.toString()
        } catch (t: Throwable) {
            Log.w(TAG, "injectPerAppRules failed, config unchanged", t)
            configJson
        }
    }

    private fun mergeInbounds(merged: JSONObject, base: JSONObject) {
        val mInbounds = merged.optJSONArray("inbounds") ?: JSONArray().also { merged.put("inbounds", it) }
        val hasTun = (0 until mInbounds.length()).any {
            mInbounds.optJSONObject(it)?.optString("type") == "tun"
        }
        if (hasTun) return
        val bInbounds = base.optJSONArray("inbounds") ?: return
        for (i in 0 until bInbounds.length()) {
            val inb = bInbounds.optJSONObject(i) ?: continue
            if (inb.optString("type") == "tun") {
                mInbounds.put(inb)
                Log.i(TAG, "injected tun inbound tag=${inb.optString("tag")}")
                return
            }
        }
    }

    private fun mergeRoute(merged: JSONObject, base: JSONObject) {
        val bRoute = base.optJSONObject("route") ?: return
        val mRoute = merged.optJSONObject("route") ?: JSONObject().also { merged.put("route", it) }

        if (!mRoute.has("auto_detect_interface") && bRoute.has("auto_detect_interface")) {
            mRoute.put("auto_detect_interface", bRoute.optBoolean("auto_detect_interface", true))
        }
        if (!mRoute.has("default_domain_resolver") && bRoute.has("default_domain_resolver")) {
            mRoute.put("default_domain_resolver", bRoute.opt("default_domain_resolver"))
        }

        val bRules = bRoute.optJSONArray("rules") ?: JSONArray()
        val mRules = mRoute.optJSONArray("rules") ?: JSONArray()
        if (bRules.length() == 0) return

        // 检测 merged 是否已有系统级 action 规则 (sniff / hijack-dns)
        val hasSystemAction = (0 until mRules.length()).any { idx ->
            val r = mRules.optJSONObject(idx) ?: return@any false
            val act = r.optString("action")
            act == "sniff" || act == "hijack-dns"
        }
        if (hasSystemAction) return

        // prepend base 的系统规则到 merged.rules 前
        val newRules = JSONArray()
        for (i in 0 until bRules.length()) newRules.put(bRules.opt(i))
        for (i in 0 until mRules.length()) newRules.put(mRules.opt(i))
        mRoute.put("rules", newRules)
        Log.i(TAG, "merged route rules: base=${bRules.length()} + merged=${mRules.length()}")
    }

    private fun mergeDns(merged: JSONObject, base: JSONObject) {
        if (merged.has("dns")) return
        val bDns = base.optJSONObject("dns") ?: return
        merged.put("dns", bDns)
        Log.i(TAG, "injected dns block from base")
    }
}
