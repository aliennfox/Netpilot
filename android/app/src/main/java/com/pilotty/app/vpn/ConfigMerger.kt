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

            when (snapshot.mode) {
                PerAppVpnPrefs.Mode.Off -> {
                    // UI 为 Off 时完全 no-op: 尊重 Go overlay 的 tun_override (Agent 的 set_per_app_vpn tool 写的).
                    // 若 overlay 也空, Go merger 不会写 include/exclude_package, merged.json 本就干净.
                    // 若 overlay 有值, 保留让 Agent 路径生效 (UI 没主动覆盖).
                }
                PerAppVpnPrefs.Mode.Allow -> {
                    // UI 主动选择白名单 —— UI 权威, 清 overlay 可能留下的 deny 字段再写 include.
                    tun.remove("exclude_package")
                    if (snapshot.packages.isNotEmpty()) {
                        tun.put("include_package", JSONArray(snapshot.packages.toList()))
                    } else {
                        tun.remove("include_package")
                    }
                }
                PerAppVpnPrefs.Mode.Deny -> {
                    tun.remove("include_package")
                    if (snapshot.packages.isNotEmpty()) {
                        tun.put("exclude_package", JSONArray(snapshot.packages.toList()))
                    } else {
                        tun.remove("exclude_package")
                    }
                }
            }
            Log.i(TAG, "injectPerAppRules mode=${snapshot.mode} pkgs=${snapshot.packages.size}")
            root.toString()
        } catch (t: Throwable) {
            Log.w(TAG, "injectPerAppRules failed, config unchanged", t)
            configJson
        }
    }

    /**
     * A4 注入一个 mixed (SOCKS5 + HTTP) inbound 绑 0.0.0.0:port, 供 LAN 设备当代理出口。
     *
     * 前置条件: VPN 要已开 + ListenAll 就不管自家流量回环 (自己的 TUN 已在 inbound tun-in 处理)。
     *
     * 容错:
     *  - snapshot.enabled=false → 不改
     *  - port 不在 1025-65535 范围 → 跳过 (UI 层已限制, 这里兜底)
     *  - 已存在同 tag "lan-mixed" inbound → 覆盖
     *  - JSON 异常 → 返回原字符串, 不阻塞 VPN 启动
     */
    fun injectLocalProxy(configJson: String, snapshot: LocalProxyPrefs.Snapshot): String {
        if (!snapshot.enabled) return configJson
        if (snapshot.port !in 1025..65535) {
            Log.w(TAG, "injectLocalProxy: port ${snapshot.port} out of range, skip")
            return configJson
        }
        return try {
            val root = JSONObject(configJson)
            val inbounds = root.optJSONArray("inbounds") ?: JSONArray().also { root.put("inbounds", it) }

            // 覆盖 / 追加: 先剥掉同 tag 的旧条目
            val filtered = JSONArray()
            for (i in 0 until inbounds.length()) {
                val inb = inbounds.optJSONObject(i) ?: continue
                if (inb.optString("tag") == "lan-mixed") continue
                filtered.put(inb)
            }
            val mixed = JSONObject().apply {
                put("type", "mixed")
                put("tag", "lan-mixed")
                put("listen", "0.0.0.0")
                put("listen_port", snapshot.port)
                // sing-box 1.11+ 移除了 inbound 级 sniff, 流量嗅探交给 route rules 的 action=sniff
            }
            filtered.put(mixed)
            root.put("inbounds", filtered)
            Log.i(TAG, "injectLocalProxy: mixed 0.0.0.0:${snapshot.port}")
            root.toString()
        } catch (t: Throwable) {
            Log.w(TAG, "injectLocalProxy failed, config unchanged", t)
            configJson
        }
    }

    /**
     * 注入 Sniffing 档位 + IPv6 模式偏好 (Phase P1-A)。
     *
     * 动作:
     *  - sniff=false → route.rules 里移除 action="sniff" 那条 (base 默认塞了)
     *  - ipv6Mode → dns.strategy 改写成对应值 + 按需调整 tun inbound address (去除 IPv4/IPv6 段)
     *
     * 容错: 任何 JSON 异常 → 返回原字符串, 不影响主配置加载。
     */
    fun injectSniffingIpv6(configJson: String, snapshot: SniffingIpv6Prefs.Snapshot): String {
        return try {
            val root = JSONObject(configJson)

            if (!snapshot.sniff) {
                val route = root.optJSONObject("route")
                val rules = route?.optJSONArray("rules")
                if (rules != null) {
                    val filtered = JSONArray()
                    for (i in 0 until rules.length()) {
                        val r = rules.optJSONObject(i) ?: continue
                        if (r.optString("action") == "sniff") continue
                        filtered.put(r)
                    }
                    route.put("rules", filtered)
                }
            }

            val dns = root.optJSONObject("dns")
            dns?.put("strategy", snapshot.ipv6Mode.raw)

            val inbounds = root.optJSONArray("inbounds")
            if (inbounds != null) {
                for (i in 0 until inbounds.length()) {
                    val inb = inbounds.optJSONObject(i) ?: continue
                    if (inb.optString("type") != "tun") continue
                    val addrs = inb.optJSONArray("address") ?: continue
                    val filtered = JSONArray()
                    for (j in 0 until addrs.length()) {
                        val a = addrs.optString(j)
                        val isV6 = a.contains(":")
                        when (snapshot.ipv6Mode) {
                            SniffingIpv6Prefs.Ipv6Mode.Ipv4Only -> if (!isV6) filtered.put(a)
                            SniffingIpv6Prefs.Ipv6Mode.Ipv6Only -> if (isV6) filtered.put(a)
                            else -> filtered.put(a)
                        }
                    }
                    if (filtered.length() == 0) {
                        // 避免双栈都被剥光的退化 (选了 v6Only 但 base 只给了 v4 etc), 保留原样
                        Log.w(TAG, "injectSniffingIpv6: tun address 被过滤空, 回退保留原 address")
                    } else {
                        inb.put("address", filtered)
                    }
                }
            }

            Log.i(TAG, "injectSniffingIpv6 sniff=${snapshot.sniff} ipv6=${snapshot.ipv6Mode.raw}")
            root.toString()
        } catch (t: Throwable) {
            Log.w(TAG, "injectSniffingIpv6 failed, config unchanged", t)
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
