package com.foxnetpilot.netpilot.vpn

import android.util.Log
import org.json.JSONArray
import org.json.JSONObject

/**
 * 把 Go 侧 overlay.Apply() 生成的 `merged.json` 与 Android 基础 tun 配置 (`android_tun_base.json`)
 * 再合并一次,确保关键的 tun inbound / route / dns / dns-out outbound 不缺。
 *
 * 动机 (Known Issue #H4): Go 侧 overlay 的 base 是 CLI 用的 minimal.json (mixed inbound),
 * 合出来的 merged.json 没有 tun inbound, sing-box libbox 永不会回调 openTun。
 * 选"Kotlin 二次合并"而非改 Go overlay 层是为了不触发 aar 重建,避开 #M8 同步纪律问题。
 *
 * 合并策略:
 *  - inbounds: 若无 type=tun, 从 base 拷一份 tun inbound 追加到末尾
 *  - outbounds: 若无 type=dns (dns-out), 从 base 拷一份
 *  - route.auto_detect_interface: 若 merged 未设, 取 base 值
 *  - route.rules: base 的系统级规则 (protocol=dns, ip_is_private) prepend 到 merged.rules 前
 *    —— 必须 prepend 而非 append, 否则用户规则 "抖音 direct" 会拦截 DNS 流量导致 dns-out 失效
 *  - dns: 若 merged 无 dns 字段, 整段拷 base
 */
object ConfigMerger {
    private const val TAG = "ConfigMerger"

    fun ensureTunInbound(mergedJson: String, tunBaseJson: String): String = try {
        val m = JSONObject(mergedJson)
        val b = JSONObject(tunBaseJson)

        mergeInbounds(m, b)
        mergeOutbounds(m, b)
        mergeRoute(m, b)
        mergeDns(m, b)

        m.toString()
    } catch (t: Throwable) {
        Log.w(TAG, "merge failed, using merged.json as-is", t)
        mergedJson
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

    private fun mergeOutbounds(merged: JSONObject, base: JSONObject) {
        val mOutbounds = merged.optJSONArray("outbounds") ?: JSONArray().also { merged.put("outbounds", it) }
        val hasDnsOut = (0 until mOutbounds.length()).any {
            mOutbounds.optJSONObject(it)?.optString("type") == "dns"
        }
        if (hasDnsOut) return
        val bOutbounds = base.optJSONArray("outbounds") ?: return
        for (i in 0 until bOutbounds.length()) {
            val ob = bOutbounds.optJSONObject(i) ?: continue
            if (ob.optString("type") == "dns") {
                mOutbounds.put(ob)
                Log.i(TAG, "injected dns outbound tag=${ob.optString("tag")}")
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

        // base 的系统级规则必须先匹配, prepend 到前面
        val bRules = bRoute.optJSONArray("rules") ?: JSONArray()
        val mRules = mRoute.optJSONArray("rules") ?: JSONArray()
        if (bRules.length() == 0) return
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
