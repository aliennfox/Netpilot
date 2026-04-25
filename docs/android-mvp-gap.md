# Android MVP Gap — v1 发布路径

> **生成日期**: 2026-04-22 · **最后扩充**: 2026-04-22(加入 Karing / Hiddify / v2rayNG / Clash Meta 四方对照 + 协议/订阅格式基线)
> **战略对照**: iOS 暂停,Android 单平台推到可发布状态
> **基线比对**:
>
> - **NekoBox for Android** (`~/References/nekobox/`) —— sing-box 同核权威功能全集,UI 发布形态参考
> - **Karing** ([karing.app](https://karing.app)) —— **用户重合度最高的直接竞品**(sing-box + 消费级 UX + 多核 + 多订阅格式)
> - **Hiddify-Next** (`~/References/hiddify-app/`) —— sing-box 跨平台中间态
> - **Clash Meta / Mihomo** —— 中文主流,走自研内核,占据大量机场默认订阅格式
> - **v2rayNG** —— Xray-core 老牌,很多老机场唯一出的就是这个订阅
>
> **当前版本**: `versionCode=1 / versionName=0.1.0`, debug APK 60MB, 真机走流量验证通过 (Pixel 4a, 2026-04-22)

---

## 0. 本文档如何阅读

本文档把"还差什么才能把 APK 发给人用"拆成三档:

- **Must-have (M)**: 不做就不能叫 v1 发布,会被 Google Play 拒审、或会在第一次安装时就让用户装不上 / 用不了 / 关键场景(订阅导入、主流协议握手)就失败
- **Should-have (S)**: 做了明显第一印象更好,NekoBox 这类竞品默认都有。不做也能塞进一个熟人群的小范围内测
- **Won't-do (W)**: 本轮 v1 明确不做,写清楚理由避免后续反复争论

> **协议 / 订阅格式是 must-have 的原生一部分**(2026-04-22 扩充)。 原先 M1-M9 偏 Android 外观 + 合规,没有正视"用户订阅粘贴进来能解析吗 / 节点拨出能握手吗"这条更基础的门槛。 新增 M10-M13 把这条补齐。
>
> **非协议功能基线也是 must-have 的一部分**(2026-04-22 二次扩充,见 Section 10)。 Per-App VPN / Kill Switch / 深链 / 配额 UI / 规则 UI 这 5 项是竞品(NekoBox / Hiddify / Karing / Clash Meta / v2rayNG)**普遍都有**的消费级 table-stakes, Pilotty 当前一个没做。

### v1 可发布总工时(2026-04-22 修订版)

| 类别 | 工时 | 工作日(单人) | 说明 |
|---|---|---|---|
| 协议基线补齐 (M10-M13) | ~25-31h | ~3 天 | TUIC / AnyTLS / ShadowTLS / Hy1 / Clash YAML 订阅 / sing-box JSON 订阅 / 回归矩阵 |
| UI / 合规基线 (M1-M9) | ~20-26h | ~2.5 天 | 签名 / 图标 / 通知 / 隐私 / API Key / 日志 |
| 非协议功能基线 (M14-M18, 见 Section 10) | ~24-40h | ~3-5 天 | Per-App VPN / Kill Switch / 深链 + QR / 配额 UI / 规则 UI |
| AI 护城河 UI 强化 (见 Section 11) | ~8-16h | ~1-2 天 | 自然语言输入 Hero / 快照回滚 Safety card / Chat tool-call timeline |
| **Android v1 可发布总和** | **~77-113h** | **~7-10 天** | 单人全职冲刺,含真机回归 |

> 原计划 (M1-M9 only, 20-26h) **严重低估**了真正的缺口。真实的可发布基线按上表, 是原估计的 3-4 倍。

每项条目的元数据字段:
- **来源**: Google Play Policy / Android 平台限制 / NekoBox 对照 / 已知 Issue (#M17 等) / 用户 UX 直觉
- **工时**: 估算的独立实施工时 (不含联调),除非注明"含联调"
- **用户影响**: 不做时用户能感知到的具体症状 (不是抽象的"体验下降")

---

## 1. Must-Have (v1 发布阻塞项)

### M1. 签名 + 可发布构建配置
- **现状**: `build.gradle.kts:20-28` 的 release 块 `isMinifyEnabled = false`,没有 `signingConfigs`, 没有 `debuggable false`。当前只能 `assembleDebug` 产 60MB debug APK, 无法产 release APK
- **需要**: (a) 新建 keystore + 加 signingConfigs.release + 环境变量取密码;(b) `isMinifyEnabled=true` + `isShrinkResources=true` + proguard rules 保 gomobile 桥接 (已有基础 `-keep class go.** / mobile.**`, 还需要加 libbox.** 和 kotlinx.serialization 的 keep);(c) `-checklinkname=0` 是编 aar 时加的,不影响 APK 侧;(d) release 变体关闭日志宏
- **来源**: Android 平台硬限 (unsigned APK 在大部分 Android 版本上装不上)
- **工时**: 3-4h (keystore 生成 + proguard 规则调试 + 验证代理功能不被 obfuscate 搞崩)
- **用户影响**: 不做 → 根本没法把包发给朋友。装不上

### M2. 应用图标 + 应用名
- **现状**: `AndroidManifest.xml:13-14` 使用 `@android:drawable/sym_def_app_icon` (Android 内置默认图标) 和 `android:label="Pilotty"` (不是本地化资源)。桌面图标是灰色默认机器人
- **需要**: 最小: 1 套 adaptive icon (foreground + background xml 两个 drawable + 6 个 mipmap 尺寸)。最大: 加夜色变体 + notification 小图标。品牌身份不需要美术定稿,先用纯色块+字母也比默认机器人强
- **来源**: NekoBox 有完整 mipmap-*dpi 六档;Google Play 上架 硬要求图标
- **工时**: 1-2h (设计 30min + 生成 adaptive assets 用 Android Studio Image Asset Studio + 替换 manifest)
- **用户影响**: 不做 → 用户把图标当系统默认,找不到 App;发给朋友时第一眼就觉得是半成品

### M3. 通知文案 + 通知图标
- **现状**: `PilottyVpnService.kt:274-280` 通知 channel "Pilotty VPN", title "Pilotty", text "VPN 已连接" 写死。小图标用 `android.R.drawable.ic_lock_lock` (系统自带锁图标, 和品牌无关)
- **需要**: (a) notification 小图标用 app monochrome 版本;(b) text 带当前节点名 + ⬆⬇ 最近分钟流量;(c) 通知 action 加"停止" 按钮,免得用户为了停 VPN 还要打开 App
- **来源**: NekoBox `ServiceNotification.kt` 有完整示例;Android 8+ 前台服务通知可见性是合规要求
- **工时**: 2h
- **用户影响**: 不做 → 用户在通知栏看到"VPN 已连接" 但不知道节点是哪个,想停 VPN 必须掏 App

### M4. #M17 订阅解析器 SS URL 丢节点 ✅ 已修(2026-04-22 晚)
- **状态**: fix 已落在 `internal/subscription/parser.go:209-291` —— 进 `parseHostPort` 之前先 `strings.Index(body, "?")` 剥 query,query 参数回填 `node.Extra` / `node.Network`。`parser_test.go` 已加 5 个 case(sip002 basic / `?type=tcp` 回归 / `?plugin=obfs-local;obfs=tls` / legacy all-base64 / malformed)覆盖 SIP002 三种 SS 格式
- **剩余**: (a) 其他 parser(vmess/vless/trojan/hysteria2/wg)同步写表驱动单测,同一类 bug 不要靠"下次 logcat 见"才发现; (b) CLAUDE.md #M17 条目应更新为"已修,已覆盖测试"
- **工时**: fix+单测 ~ 1h 已花;其它 parser 单测补齐另计 2-3h
- **用户影响**: 已消除"订阅 20 个节点只显示 3 个"的问题

### M10. 协议解析器扩充 (TUIC v5 / AnyTLS / ShadowTLS / Hysteria v1 / VLESS Reality full) ✅ 已完成 (2026-04-22, Known Issue #M18)
- **现状**: `internal/subscription/parser.go:182-198` 的 `parseLine` switch 只识别 6 个 scheme:`ss / trojan / vmess / vless / hysteria2|hy2 / wireguard|wg`。机场常见但我们**直接丢弃**的 URI 有:
  - `tuic://` —— TUIC v5,基于 QUIC,2024 年后新机场标配(Karing / Hiddify / NekoBox 全支持,NekoBox 位置:`~/References/nekobox/app/src/main/java/io/nekohasekai/sagernet/fmt/tuic/` + `TuicSettingsActivity.kt`)
  - `anytls://` —— AnyTLS,mRIYIlTv 等小众但上升中协议(NekoBox:`~/References/nekobox/app/src/main/java/moe/matsuri/nb4a/proxy/anytls/AnyTLSFmt.kt` + `AnyTLSBean.java`;sing-box 官方 outbound 支持)
  - `shadowtls://` —— ShadowTLS,专门对抗 SNI 嗅探的抗审查协议(NekoBox:`~/References/nekobox/app/src/main/java/moe/matsuri/nb4a/proxy/shadowtls/ShadowTLSFmt.kt`)
  - `hysteria://` —— Hysteria v1(**非** hysteria2),仍部署在大量旧机场,不等于 hy2 向后兼容(NekoBox:`~/References/nekobox/app/src/main/java/io/nekohasekai/sagernet/fmt/hysteria/HysteriaBean.kt` 单 bean 同时承载 v1/v2,scheme 分别是 `hysteria://` / `hysteria2://`)
  - **VLESS Reality 全量字段**:我们的 `parseVLess` (parser.go:401-474) 已捕获 `pbk / sid / fp / flow / spx / security=reality`,converter.go:127-135 映射到 sing-box `tls.reality.{public_key, short_id}`,converter.go:116-117 已把 `node.Extra["flow"]` 写入 outbound `flow` 字段。**解析 + 映射已齐**,剩下的只是:(a) 表驱动单测覆盖 `flow=xtls-rprx-vision`;(b) 真机回归(目前真机走通的是 VLESS+Reality 香港-1,需确认该节点原订阅有 `flow` 字段才证完整链);(c) `fp / spx` 两个字段当前只进 Extra,未见 converter 写入 outbound(需 verify sing-box 是否期望这些在 `tls.reality.fingerprint` 等位置)
- **需要**:
  - [ ] `parseTuic()` + `convertTuic()` + sing-box outbound `type=tuic`(字段:uuid, password, congestion_control, udp_relay_mode, alpn, sni)
  - [ ] `parseAnyTLS()` + `convertAnyTLS()` + sing-box outbound `type=anytls`
  - [ ] `parseShadowTLS()` + `convertShadowTLS()` + sing-box outbound `type=shadowtls`(注意:ShadowTLS 常作为 SS 的前置,可能要两段 outbound 链)
  - [ ] `parseHysteria1()` + `convertHysteria()` + sing-box outbound `type=hysteria`(注意参数和 hy2 不一样:`auth_str` vs hy2 的 `password`,`up_mbps/down_mbps` 必填)
  - [ ] `convertVLess` 加 `flow` 字段透传 + 单测覆盖 `flow=xtls-rprx-vision`
- **来源**:
  - Karing 官网 feature list 声明 "All sing-box protocols"(含 TUIC / AnyTLS / ShadowTLS / Hysteria)
  - NekoBox 协议全集:上述 `fmt/` 各子目录 + `moe/matsuri/nb4a/proxy/`
  - sing-box 官方 outbound 类型清单:[sing-box.sagernet.org/configuration/outbound](https://sing-box.sagernet.org/configuration/outbound/)(我们已 aar 嵌入 libbox 支持这些 outbound type,只差订阅侧 URI 解析)
- **工时**: 5 个 parser × 1-2h URI 解析 + converter 映射 + 表驱动单测 ≈ 8-12h,集中做可压到 1.5 天
- **用户影响**: 不做 → 用户粘贴机场订阅,**每条不支持的 URI 都被 `parseLine` 静默 skip**,跟 #M17 表现一模一样("明明 20 节点只剩 8 个")。在 TUIC 普及的机场上更严重,可能从"丢 20%"变成"丢 70%"
- **验证方式**: (a) 每个 parser 加表驱动测试;(b) 找 3 家已知机场做真机粘贴回归,比对"机场网页公布节点数"与"Pilotty 列表节点数"

### M11. 订阅格式扩充 — Clash / Clash.Meta YAML ✅ 已完成 (2026-04-22, Known Issue #M19)
- **现状**: `internal/subscription/parser.go:127-154` 的 `ParseSubscription` **只接受 base64-encoded URI 列表**。`tryBase64Decode` 失败就返回"Base64 解码失败"。 我们当前连 `proxies:` 开头的纯 YAML 订阅(Clash 家族)都不尝试解析,更别说对应的节点 map → `NodeConfig`
- **为什么关键**:国内机场生态是"Clash 优先,v2ray/sing-box 其次"。 很多机场**只**给 Clash 订阅链接,连 base64 URI 列表都不给。NekoBox `group/RawUpdater.kt:227-243` 的做法是:HTTP 拿原文后 `if (text.contains("proxies:"))` 走 SnakeYAML → 遍历 `yaml["proxies"]` 的 List<Map<String, Any?>>,逐个映射到内部 Bean。 Karing 的公开 feature list 把"支持 Clash/Clash.Meta 订阅"列在首位就是因为这是消费级用户的**入场门票**。粘贴进来被拒 = 用户流失
- **需要**:
  - [ ] `ParseSubscription` 入口先探测:文本 TrimSpace 后 `HasPrefix("proxies:")` 或 `Contains("\nproxies:")`(YAML 顶层 key 检测)→ 走 Clash 路径;否则走现有 base64
  - [ ] 新增 `internal/subscription/clash.go`:`ParseClashYAML([]byte) ([]NodeConfig, error)`,用 `gopkg.in/yaml.v3` 把 `proxies` 数组反序列化成 `[]map[string]any`,分 type(`type: ss / vmess / vless / trojan / hysteria2 / tuic / anytls / wireguard...`)映射到 `NodeConfig`
  - [ ] Clash 特有字段覆盖: `cipher → Method`, `plugin-opts` (ss), `ws-opts.path/headers.Host` (vmess/vless), `reality-opts.{public-key,short-id}` (vless reality)
  - [ ] 表驱动单测:挑 3 家主流机场的 sample yaml(脱敏后)回归
  - [ ] `go.mod` 增 `gopkg.in/yaml.v3` 依赖;注意 rebuild aar (#M8 纪律)
- **来源**:
  - NekoBox `~/References/nekobox/app/src/main/java/io/nekohasekai/sagernet/group/RawUpdater.kt:231-243`(SnakeYAML 方案)
  - Clash.Meta 订阅示例:[wiki.metacubex.one/config/proxies](https://wiki.metacubex.one/config/proxies/)
  - sing-box 官方没有"clash 订阅"概念(只有自家 JSON 订阅),但嵌入式 libbox 不关心 URI 层,parser 输出 `NodeConfig` → 现有 converter 即可
- **工时**: 8-10h(依赖接入 + 映射 + 测试 + 真机回归)≈ 1 天
- **用户影响**: 不做 → 10 个机场里可能 5 个粘贴进来立刻报"Base64 解码失败",完全无法使用。这是 **Karing 相对我们的最大 UX 护城河**,不抹平就别谈消费化

### M12. sing-box 原生订阅格式(自家 JSON)回归验证 ✅ 已完成 (2026-04-22, Known Issue #M19)
- **现状**: `ParseSubscription` 走 base64 → 按行拆 → 每行当 URI 解析。实际上 sing-box 官方订阅格式是"一个 JSON 顶层含 `outbounds` 数组",不是 URI 列表。我们从没验证过粘贴一个纯 sing-box JSON 订阅会发生什么(预期:base64 解码失败就退出)
- **需要**:
  - [ ] `ParseSubscription` 格式探测里加分支:`HasPrefix("{")` 且能 Unmarshal 出 `{"outbounds":[...]}` → 走 sing-box native 路径
  - [ ] 新增 `ParseSingBoxJSON`:直接把每个 outbound 元素映射回 `NodeConfig`(含 vless/vmess/tuic/anytls/shadowtls 所有 sing-box 原生支持类型)
  - [ ] 边界:多 outbound / selector 嵌套 / detour 链该怎么展开成扁平节点列表,需要设计决策
- **来源**: sing-box 官方 [Configuration Format](https://sing-box.sagernet.org/configuration/)
- **工时**: 4-6h
- **用户影响**: 不做 → 少数 sing-box 原生机场粘贴失败。比 Clash 影响小,但排到 must 是因为"我们是 sing-box 客户端"这件事用户有心理期待

### M13. 协议 + 订阅格式真机回归矩阵 ✅ 已完成 (2026-04-22, scripts/subscription-matrix-test.sh)
- **现状**: M10-M12 做完后,缺系统回归:给定(3 家真实机场)× (2 种订阅格式 Clash / base64)× (支持的 10+ 种 outbound type),实际能跑通的组合有多少?目前只靠 `scripts/smoke.sh` 跑一条路径,盲点大
- **需要**:
  - [ ] 在 `scripts/` 下加 `subscription-matrix-test.sh`:对每个脱敏 fixture 文件跑 `parse → convert → sing-box config validate → 启动 → ping` 全链
  - [ ] 失败 case 自动归档到 `testdata/failed/`
  - [ ] 把通过率写进 release checklist
- **来源**: 没有直接参考,自造。理由:M10 M11 引入的代码面太大,不自动化回归 = 下一个 #M17 在路上
- **工时**: 3-4h 搭架子 + 收 fixture 1-2h + 持续维护
- **用户影响**: 不做 → "发布时测过 OK,两周后新机场订阅格式微调就挂"

### M5. 启动空状态引导
- **现状**: `SettingsScreen.kt:62-66` 只有一句"还没有订阅。点右上「添加」粘贴 URL 导入"。 Dashboard 首屏在没订阅时显示节点名 "—",模式 "rule",连接数 0。 没有强引导告诉新用户第一步是什么
- **需要**: Dashboard 首屏若 `nodeCount=0` 则显示引导卡: "先添加订阅 → 跳到设置页 / 粘贴单条节点 URL"。Chat tab 若未配置 LLM API Key 应明示"配置 Key 后 Agent 才能用"
- **来源**: UX 直觉 + NekoBox ConfigurationFragment 的空状态处理
- **工时**: 2-3h (含设计简单文案)
- **用户影响**: 不做 → 新用户打开 App 看到一屏"——",以为是 bug;留存率崩

### M6. 隐私政策 + 权限说明
- **现状**: 无 privacy policy 文件,无 `data safety` 声明
- **需要**: (a) 写一份最小 privacy policy markdown 说明"应用读写本地文件用于代理配置 / 不收集 PII / LLM API Key 明文存本地";(b) 放一个 GitHub Pages URL;(c) Google Play Data Safety 表要声明 (Network Extension/VPN 类 App 必填);(d) 应用内"关于"页加链接
- **来源**: Google Play Policy (VPN 类 App 强制要求 data safety + privacy policy URL)
- **工时**: 2-3h (草稿 + 部署到 Pages + 表格填写)
- **用户影响**: 不做 → Google Play 审核直接拒。即使只在小圈子分发 APK, 没有 privacy 声明也容易被举报 "这 App 是不是在偷数据"

### M7. 最小日志导出 / 复制
- **现状**: Go 侧日志走 stderr, libbox 日志走 Android Log。用户一旦遇到"连不上",开发者没任何手段复现 —— adb logcat 对外分发用户没法用
- **需要**: (a) 一个"导出最近日志"按钮, 产出一个 zip 或 txt 到用户 Downloads;(b) 可选:内嵌 logcat viewer (NekoBox `LogcatFragment` 模式)。MVP 用 (a) 足够
- **来源**: 支持/bug 回报流程硬需求
- **工时**: 3h (含权限适配 Android 10+ scoped storage)
- **用户影响**: 不做 → 收到"我连不上"的反馈,我们只能说"重装试试"

### M8. LLM API Key 配置入口 ✅ 已完成(2026-04-22 ~ 2026-04-25 验证)
- **✅ 实际交付**: Settings UI `AgentApiKeySection.kt` + `ApiKeyPrefs` 持久化 + `PilottyApp.kt:35-37` 启动注入 + `PilottyCore.setApiKey → Client.SetAPIKey` 热重载。无需重启即可切换 Key。2026-04-25 Explore 核实链路贯通
- **⚠️ 剩余安全债(v1.x,非阻塞 v1 发布)**: `ApiKeyPrefs` 用明文 `SharedPreferences`,未升级到 `androidx.security-crypto` 的 `EncryptedSharedPreferences`;`AgentApiKeySection.kt:28` KDoc 还写"需重启",与实际热重载行为有文档漂移
- **原需求**(供历史参考): 设置页加"LLM 配置"卡输入硅基流动/DeepSeek Key + 加密存储 + 启动时读出传给 `PilottyCore.init`。原估 3-4h,实际已做但跳过加密存储那一步

### M9. VPN 异常退出 / 系统回收的用户反馈
- **现状**: VpnService 被系统杀掉后 `onRevoke()` 或 `onDestroy()` 调 stopService, 但 UI 没有"VPN 意外中断" 的提示。#M13 修了 UI 跟随 tunRunning,但"突然从 true 掉到 false"的语义和"用户主动停"混在一起
- **需要**: 记录最后一次 stop 的原因 (user / revoke / crash),Dashboard 显示"上次意外中断于 X 时 · 查看日志"。配合 M7 让用户能自救
- **来源**: NekoBox `BaseService.kt` 有 STATE 机器 + 错误透传
- **工时**: 2-3h
- **用户影响**: 不做 → 用户看到 VPN 突然就不工作了, 不知道是自己点错了还是系统杀了,会反复点重启

---

## 2. Should-Have (更好第一印象)

### S1. Dark Theme + Material You
- **现状**: `AndroidManifest.xml` 用 `Theme.Material.Light.NoActionBar` 系统主题。Compose `MaterialTheme {}` 裸用默认配色。 `res/values-night/` 不存在
- **需要**: `values/colors.xml` + `values-night/colors.xml`, Compose 端 `darkColorScheme()/lightColorScheme()`。可选:dynamic color (`Build.VERSION_CODES.S+` 吃系统取色)
- **来源**: Material3 标准;NekoBox 有 `values-night/` 分支
- **工时**: 2-3h
- **用户影响**: 不做 → 晚上用 App 白屏刺眼;Android 12+ 用户看不到和系统一致的配色,感觉廉价

### S2. 流量实时图 / 连接列表
- **现状**: Dashboard 只显示累计 upload/download 字节数 (`StatusDto.upload/download`)。没实时曲线,没"当前正在连什么"的连接列表
- **需要**: (a) Dashboard 加 sparkline 曲线 (最近 60s 每秒采一点);(b) 单独 tab 或页面显示 `/connections` 列表 (hostname, process, bytes, via proxy-group)。Clash API 已暴露,Go 侧加 `connections()` gRPC/HTTP 包装
- **来源**: NekoBox 用 TrafficUpdater 做实时图;3B-3 debrief 里 `/connections` 是调试关键证据
- **工时**: 6-8h (含 Compose Canvas 画图 + WebSocket/轮询)
- **用户影响**: 不做 → 用户不知道流量在不在走代理,无法 debug "为啥 X 网站慢"

### S3. Quick Settings Tile (一键开关)
- **现状**: 无
- **需要**: `PilottyTileService : TileService` + manifest 声明 + bind VpnService。用户下拉通知栏就能点开关
- **来源**: NekoBox `TileService.kt` (~100 行)
- **工时**: 3h
- **用户影响**: 不做 → 开关 VPN 每次都要打开 App,流量场景下的使用门槛变高

### S4. 订阅二维码扫码 / 剪贴板粘贴增强
- **现状**: 只有 `OutlinedTextField` 输入 URL。不读剪贴板,不扫码
- **需要**: (a) 添加订阅对话框检测剪贴板有 `http://|https://|ss://|vmess://` 时弹"粘贴?"按钮;(b) 扫码 → 用 CameraX + ZXing
- **来源**: NekoBox ScannerActivity + QRCodeDialog;小白用户分享节点主要渠道
- **工时**: (a) 1h · (b) 4-5h
- **用户影响**: 不做 → 机场老板的 v2rayN 订阅给过来,用户必须切换到浏览器复制 URL 再切回粘贴,断流

### S5. Per-App VPN UI
- **现状**: `PilottyVpnService.kt:91-103` 会读 libbox 的 includePackage/excludePackage,但 libbox 配置里这个字段没 UI 编辑,实际等同"全局接管"。只硬写了 exclude 自己 packageName 防回环
- **需要**: 设置页加 "按 App 代理" 开关 + 应用列表 multi-select → 写入 overlay.ProxySettings → `libbox.TunOptions.includePackage/excludePackage`
- **来源**: NekoBox AppManagerActivity + AppListActivity (~200 行);国内场景"微信直连,浏览器代理"的高频诉求
- **工时**: 6-8h (含列表 lazy-load + 图标缓存)
- **用户影响**: 不做 → 想分流的用户改不了, 只能"要么全走要么全不走"

### S6. 节点 QR 导出 / 复制 URL
- **现状**: 节点只能导入,不能分享
- **需要**: 节点列表长按菜单加"复制 URL" + "显示二维码"
- **来源**: NekoBox 标准功能
- **工时**: 3h
- **用户影响**: 不做 → 用户想把某个节点发给朋友 / 换到另一台手机时没法做

### S7. Backup / Restore UI
- **现状**: `PilottyCore.exportBackup()/importBackup()` Go 侧 API 已实现 (Phase 2.5 T12);Kotlin 侧没 UI wire up。 Settings 底部有一行 "故障自动切换 / 备份导入导出 / API Token (TODO)"
- **需要**: Settings 页加 "备份 / 恢复" 两个按钮, 调用 `SAF` 选 / 保存 JSON 文件
- **来源**: 后端已有,纯前端 wire up;NekoBox `BackupFragment`
- **工时**: 3h
- **用户影响**: 不做 → 换手机 / 重装 APK 全部订阅要重新粘贴。但小众场景, should- 不是 must-

### S8. 自动订阅更新 (WorkManager)
- **现状**: `SubscriptionDto.autoUpdate + intervalMinutes` 字段存在,Go 侧参数有但 Android 没有 WorkManager 任务驱动实际更新
- **需要**: 开 WorkManager PeriodicWorkRequest 每 N 分钟跑一次 `updateAllSubscriptions`;失败 backoff
- **来源**: 订阅 TTL 常见;NekoBox SubscriptionUpdater
- **工时**: 3-4h
- **用户影响**: 不做 → 用户需要手动点"全部更新"才能吃到机场的新节点。小众,但无痛

### S9. 路由规则编辑器 (当前只能套模板)
- **现状**: Rules tab 能列出规则 + 应用模板,不能手工加减规则。 #M4 (订阅节点不走 `_agent:` 前缀) 间接增加了编辑风险
- **需要**: 规则列表每项右侧 "编辑 / 删除",底部"+ 自定义规则" dialog (domain_suffix / process_name / outbound)
- **来源**: Phase 1 任务 5 声称"Config Overlay + 路由规则增删 + 4 个内置模板" 但 Android 只做了读 + 模板应用
- **工时**: 6-8h
- **用户影响**: 不做 → 用户想加一条"公司内网走直连",只能求 Agent 帮忙。小众

### S10. i18n 基础设施
- **现状**: 中文字符串硬编码在 Compose 里 (e.g., `Text("启动 VPN")`,`MainActivity.kt:50-55` 的 tab label "首页/对话/节点/规则/设置")。 `res/values/strings.xml` 不存在
- **需要**: 不一定要加英文翻译,但至少把所有中文字符串迁到 `strings.xml`,这样以后加英文 / 繁体只需要加 `values-*/` 文件夹,不改 Kotlin。 NekoBox 支持 20+ 语言
- **来源**: Android 惯例;发海外必备
- **工时**: 3-4h
- **用户影响**: 不做 → v1 只能给中文用户。海外朋友看不懂

### S11. In-App 日志 Viewer
- **现状**: M7 做到"导出"已够 MVP。in-app viewer 是 nicer
- **需要**: 读 `filesDir/log.txt` + TailF 轮询 + 滚到底
- **来源**: NekoBox LogcatFragment
- **工时**: 3h
- **用户影响**: 不做 → 用户想自己看日志要 `adb logcat`,几乎不可能

### S12. 基础协议补完 — SOCKS 4/5 & HTTP(S)
- **现状**: `parseLine` switch 不识别 `socks://` / `socks5://` / `http://` / `https://`。NekoBox 的 `fmt/socks/SOCKSBean` + `fmt/http/HttpBean` 都有,是"一切代理的起点"
- **为什么不是 Must-have**:现代机场很少以 SOCKS/HTTP URL 形式派发订阅,通常是"主协议 + 回退 SOCKS 本地链";用户遇到的情境多是手动配置跳板机
- **需要**: (a) `parseSocks` / `parseHttp` 两个 parser + 同名 converter(sing-box outbound `type: socks / http`,几乎零映射成本,直接 host/port/user/password);(b) 对 `socks5+tls` / `https` 握手 TLS 字段透传
- **来源**: NekoBox `~/References/nekobox/app/src/main/java/io/nekohasekai/sagernet/fmt/socks/` + `fmt/http/`;sing-box 官方 [socks outbound](https://sing-box.sagernet.org/configuration/outbound/socks/) + [http outbound](https://sing-box.sagernet.org/configuration/outbound/http/)
- **工时**: 3-4h(两个协议合做,大量代码可共享 `parseHostPort`)
- **用户影响**: 不做 → 企业用户想配"走公司跳板机然后再去机场"的链式分流,当前只能用 Agent `create_chain` + 手改 overlay,消费级用户就放弃了

### S13. 节点手工单条粘贴(非订阅 URL)
- **现状**: Settings 只接受 http(s):// 订阅 URL;不接受直接粘贴 `vless://...#HK` 单条
- **需要**: 添加订阅对话框文本变化监听:若是 `ss:// / vmess:// / vless:// / trojan:// / tuic://...` 等 scheme,直接当单节点 import(复用 parser 层)
- **来源**: Karing / NekoBox / v2rayNG 都支持。 零星分享节点场景(朋友丢一条节点给你)
- **工时**: 1h(UI 分支)
- **用户影响**: 不做 → 用户必须找地方挂个 http server 把单节点包成订阅再粘,根本没人会这样做 → 放弃本 App

---

## 3. Won't-Do for v1 (显式延期)

### W1. iOS 客户端
- **理由**: 本次战略转向 —— 付费开发者账号 / NE Entitlement 审批 / 双端心智成本 全部留到 Android v1 发布并稳定后重评估
- **参考**: `docs/phase-3b-4-plan.md` 整本已 DEFERRED,不删除
- **触发重启信号**: Android v1 发布 + 30 天内无 P0 崩溃 + 有 >50 DAU + 用户明确要 iOS

### W2. Profile / Group 多配置隔离
- **理由**: Pilotty 设计走 "单 overlay + Agent 自动编排",不做 NekoBox 的 ConfigurationFragment / GroupFragment 那套"机场 A / 机场 B" 多配置切换。 产品定位差异,不是工时问题
- **触发重启信号**: 用户反复要求"我想同时管多个订阅但它们不混"

### W3. 协议手工编辑器 (per-protocol ProfileSettingsActivity)
- **理由**: NekoBox 给每个协议 (SS/Trojan/VMess/Hysteria/WireGuard/...) 一个独立编辑 Activity。Pilotty 的路线是"订阅导入 + Agent 创建链式代理 (`create_chain` tool)", 不暴露手工字段编辑
- **触发重启信号**: 用户需要把订阅里的节点做微调 (改 UUID / 换传输层);当下用 Agent chat 可以覆盖

### W4. STUN / 诊断工具页
- **理由**: NekoBox `StunActivity` + `ToolsFragment` 是 debug 向功能,v1 用户不需要
- **触发重启信号**: 网络诊断成为高频 support 问题

### W5. 插件系统
- **理由**: NekoBox 的 SS-Rust / NaïveProxy 等插件是因为它要支持老版本协议,Pilotty 走 sing-box 一个 kernel 搞定所有, 无需外部 plugin
- **触发重启信号**: 出现 sing-box 原生不支持的协议,且用户量大

### W6. Yacd / 内嵌 Web 面板
- **理由**: 相比 Yacd,内建 Compose 页面 (连接列表、流量图) 性能和主题更统一
- **触发重启信号**: S2 做不过 Yacd,或外部有极简洁的 Webview 方案

### W7. 手动节点编辑器 (非从订阅)
- **理由**: S6 搞定"复制分享",W3 关掉"手动编辑",留一个"手动粘贴单条 URL 导入"走 M5 的空状态引导就够了
- **触发重启信号**: 机场停服 / 用户要从头搭自己的节点

### W8. TV / 平板 / Wear OS 适配
- **理由**: 手机单一尺寸优先
- **触发重启信号**: 有用户明确要

### W9. 云同步 / 账号系统
- **理由**: 只做本地数据 + S7 手动备份。云同步牵扯后端 + 加密密钥管理 + 法务,成本大
- **触发重启信号**: 用户反复要

### W10. 多语言翻译本体
- **理由**: S10 只做基础设施 (字符串抽到 resources)。英文 / 繁体 / 日文翻译不做
- **触发重启信号**: 海外分发准备

### W11. SSR (ShadowsocksR)
- **理由**: 纯 legacy 协议,核心机场 2023 年后基本弃用。sing-box **不原生支持** SSR outbound(需要 SS-Rust + ssr-local 插件),抄实现意味着引入外部二进制/插件系统(见 W5)。NekoBox 也是靠插件支持,不是内核直支持
- **来源**: sing-box issue 列表 + [sing-box outbound 官方清单](https://sing-box.sagernet.org/configuration/outbound/)不含 ssr
- **触发重启信号**: 出现 >10 用户反馈"只有 SSR 订阅可用"

### W12. Mieru
- **理由**: 2024 年出现的新协议,绝对用户量极小;sing-box v1.13 支持 `mieru` outbound 但稳定性存疑(社区反馈较少);NekoBox 给了 `MieruSettingsActivity.kt` 专门适配其配置复杂度。投 ROI 不合算
- **触发重启信号**: sing-box 社区确认 mieru 稳定 + 机场开始默认分发

### W13. NaiveProxy
- **理由**: NaiveProxy 走 Chrome 网络栈,sing-box 原生不支持(NekoBox 用独立二进制插件:`naive_preferences.xml`);绝对用户量极小,主要是老 clash 生态残留
- **来源**: sing-box outbound 清单不含 naive;NekoBox `PluginManager.kt` 走插件路径
- **触发重启信号**: 需求量 > TUIC

### W14. SSH as proxy
- **理由**: Hiddify 独有特性,主要服务伊朗等绕过场景;sing-box 虽有 `ssh` outbound,但中文消费级市场几乎零需求。投入 ROI 最差
- **来源**: sing-box [ssh outbound](https://sing-box.sagernet.org/configuration/outbound/ssh/)
- **触发重启信号**: Pilotty 进入 Hiddify 主用户区(伊朗 / 俄语区)

### W15. Trojan-Go
- **理由**: 基于 Trojan 的延伸协议,sing-box 不原生支持(NekoBox 用 `fmt/trojan_go/` + 独立插件进程);标准 Trojan 已覆盖 95% 场景,Trojan-Go 的 ws-fallback 等差异化对用户不可见
- **触发重启信号**: 大面积 Trojan-Go only 机场出现(极不可能)

---

## 4. 竞争定位(2026-04-22 扩充)

Pilotty 的目标用户画像**重合度最高的是 Karing**(消费级 UX / 多订阅格式容忍 / 主流 GPT-aware 用户),而不是 NekoBox(幂级用户)或 Hiddify(抗审查重灾区用户)。

**匹配 Karing 的协议 + 订阅基线是"入场门票"**(M10-M13 覆盖)。不抹平这条基线,Pilotty 永远是 NekoBox 的功能子集 + 一个看起来很酷但实际没法用的 Chat tab。

**AI Agent 层(自然语言指令 / tool-call 可观测性 / 自动快照 + 回滚)是真正的护城河**,5 家竞品(NekoBox / Hiddify / Karing / Clash Meta / v2rayNG)无一具备:

| 能力 | NekoBox | Hiddify | Karing | Clash Meta | v2rayNG | Pilotty |
|---|---|---|---|---|---|---|
| sing-box 内核 | ✅ | ✅ | ✅(多核) | ❌(自研) | ❌(Xray) | ✅ |
| Clash YAML 订阅 | ✅ | ✅ | ✅ | ✅ | ❌ | **❌(M11)** |
| sing-box JSON 订阅 | ✅ | ✅ | ✅ | ❌ | ❌ | **部分(M12)** |
| TUIC v5 | ✅ | ✅ | ✅ | ✅ | ❌ | **❌(M10)** |
| AnyTLS | ✅ | ❌ | ✅ | ⚠️(部分) | ❌ | **❌(M10)** |
| ShadowTLS | ✅ | ✅ | ✅ | ✅ | ❌ | **❌(M10)** |
| Hysteria v1 | ✅ | ✅ | ✅ | ✅ | ❌ | **❌(M10)** |
| VLESS Reality Vision | ✅ | ✅ | ✅ | ✅ | ✅ | **部分(M10)** |
| 自然语言 Agent | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| Tool-call 可审计 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| 自动快照 + 回滚 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| 中文 LLM 原生接入 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |

**战略推论**:

1. **3B-4 阶段 必须包含协议 + 订阅格式基线**(M10-M13),不只是 UI 抛光。不补这一条,v1 发布即"比 Karing 少 4 种协议、少 1 种订阅格式",用户看完 protocol 列表就劝退
2. 补完基线的额外成本约 **3 天**(见 section 5 新增项),是 must-have 总工时的 ~50% 增量。收益是把"无法使用"变成"功能对齐 + Agent 差异化"
3. 未来所有"我们要不要加 XX 协议"的决策,先查 Karing 是否支持;Karing 有 = 用户会问,必须排进路线;Karing 无 = 延后

---

## 5. 汇总表

| 档 | 条目 | 工时 | 关键阻塞? |
|---|---|---|---|
| M1 | 签名 + 可发布构建 | 3-4h | **是** |
| M2 | 应用图标 + 应用名 | 1-2h | **是** |
| M3 | 通知文案 + 图标 | 2h | 否 但低成本 |
| M4 | #M17 SS 解析丢节点 ✅ 已修 | 已花 ~1h | (数据正确性,已消) |
| M5 | 启动空状态引导 | 2-3h | 否 但砸留存 |
| M6 | 隐私政策 + 权限说明 | 2-3h | **是** (Play policy) |
| M7 | 最小日志导出 | 3h | 否 但砸支持 |
| M8 | LLM API Key 配置 | 3-4h | **是** (Chat tab 否则废) |
| M9 | VPN 异常反馈 | 2-3h | 否 但砸信任 |
| **M10** | **协议解析器 TUIC/AnyTLS/ShadowTLS/Hy1/VLESS-flow** | **8-12h** | **是(消费级对齐)** |
| **M11** | **Clash / Clash.Meta YAML 订阅格式** | **8-10h** | **是(消费级入场券)** |
| **M12** | **sing-box native JSON 订阅** | **4-6h** | 是(品牌一致性) |
| **M13** | **订阅 × 协议真机回归矩阵** | **4-6h** | 否(自我保护) |
| **M14** | **Per-App VPN UI** | **6-8h** | **是(竞品普遍都有)** |
| **M15** | **Kill Switch(方式 A 引导版)** ✅ 已完成 | ~1h 实际 | **是(隐私用户门槛)** |
| **M16** | **Deep Link + 剪贴板导入** ✅ (QR 延期 v1.1) | ~2.5h 实际 | **是(分享路径断流)** |
| **M17(feat)** | **订阅流量配额 UI** ✅ 已完成 | 0.5h 实际 | 是(to-avoid-surprise) |
| **M18(feat)** | **路由规则 CRUD UI** ✅ 已完成 | ~2h 实际 | 是(普通用户找不到入口) |
| **M 合计(v2)** | | **~70-96h(原 45-57h + 非协议基线 ~25-33h)** | |
| S1 | Dark Theme | 2-3h | |
| S2 | 流量图 / 连接列表 | 6-8h | |
| S3 | Quick Settings Tile | 3h | |
| S4 | 扫码 / 剪贴板粘贴 | 5-6h | |
| S5 | Per-App VPN UI | 6-8h | |
| S6 | 节点 QR 导出 | 3h | |
| S7 | Backup / Restore UI | 3h | |
| S8 | 自动订阅更新 | 3-4h | |
| S9 | 路由规则编辑器 | 6-8h | |
| S10 | i18n 基础设施 | 3-4h | |
| S11 | In-App 日志 Viewer | 3h | |
| **S12** | **SOCKS 4/5 + HTTP(S) 基础协议** | **3-4h** | |
| **S13** | **单条节点 URI 粘贴导入** | **1h** | |
| **S14** | **订阅到期提醒** | **2-3h** | |
| **S15** | **Nodes 页面 Auto-failover 开关** | **2h** | |
| **S16** | **连接时自动选最快节点** | **2-3h** | |
| **S 合计** | | **53-69h** | |
| D1 | 自然语言指令 Dashboard 顶部入口 ✅ 已完成 | ~1h 实际 | **AI 护城河 UI** |
| D2 | Safety card(Snapshot 一键回滚) ✅ 已完成 | ~1h 实际 | **AI 护城河 UI** |
| D3 | Chat tool-call inline timeline | 4-6h | **AI 护城河 UI** |
| **D 合计** | | **10-16h** | |

**建议打包**(2026-04-22 二次扩充版):
- **v1.0 可发布 = M1-M18 + D1-D3 (~7-10 天单人全职)**
  - Day 1-3: M10-M13 协议 + 订阅格式(追平消费级基线)
  - Day 4-5: M14-M18 非协议功能基线(追平竞品 table-stakes)
  - Day 6-7: M1-M9 UI / 合规(可发布条件)
  - Day 8-9: D1-D3 AI 护城河 UI 强化(差异化点)
  - Day 10: 真机端到端 + buffer
  - **关键认识: 原先以为 20-26h(3 天)就能发布,真实基线是其 3-4 倍**
- v1.1 "抛光" = S1 + S3 + S8 + S13 + S14 + S15 (低成本高回报 ≈ 1.5 天)
- v1.2 "可见性" = S2 + S7 + S11 + S16 (≈ 2 天)
- v1.3 "分流能力" = S4 + S5 + S9 + S12 (≈ 3 天)
- v1.4 "出海准备" = S6 + S10 + 实际翻译 (按需)

## 6. 依赖关系 / 顺序建议

```
M1 签名 ──┐
         ├─→ 必须最先,否则后面的 release 验证都做不了
M2 图标 ──┘

M10 协议 ──┐
M11 Clash ─┼─→ 基础层,都是 Go 侧 + 单测, Android 侧零动
M12 JSON ──┘   └─→ M10 和 M11 可并行 (不同 parser 文件), M12 依赖 M10
                  └─→ 完成后 M13 串起来做回归矩阵

M4 #M17 ──→ 已完成 ✅

M6 隐私 ──→ 独立, 可和 docs 团队/本人分头做

M8 API Key ──→ 先做, 因为 Chat tab 依赖它不废
       │
       ▼
M5 空状态 ──→ 在 M8 之后才能给出"Chat 需配 Key"的准确引导

M3 通知 / M7 日志 / M9 异常反馈 ──→ 独立, 穿插

S12 SOCKS/HTTP ←── 推荐搭 M10 一起做 (同一代码区,一次 review 省事)

之后依次 S1 → S3 → S8 → S13 → S2 → ...
```

**推荐推进顺序(单人 6 天冲刺版)**:

| Day | 上午 | 下午 |
|---|---|---|
| 1 | M10 TUIC + Hysteria1 parser | M10 AnyTLS + ShadowTLS parser |
| 2 | M10 VLESS flow 映射 + 单测齐 | M11 Clash YAML parser (含 yaml.v3 依赖) |
| 3 | M11 Clash 协议映射 + 真机 3 家机场回归 | M12 sing-box JSON 订阅 |
| 4 | M13 回归脚本 + S12 SOCKS/HTTP 搭车 | M1 签名 + M2 图标 |
| 5 | M6 隐私政策 + M8 API Key UI | M5 空状态 + M3 通知 |
| 6 | M7 日志导出 + M9 异常反馈 | 真机端到端 + buffer |

## 7. 不在本文档管辖范围的事项

以下与 Android MVP 无直接关联, 但 **Android v1 发布之前必须同步解决**:

- **#H2 Orchestrator rollback**: Agent 三角色编排 Verify 失败路径失效。 CLI 和 Server 都受影响,但 Android 会暴露"自动修复失败时卡死" —— 如果 Chat tab 用 Orchestrator, 这是 UX bug 不是后端 bug
- **#H3 零 go test**: 订阅解析器 1648 行无单测, #M17 漏掉就是这种欠债的表现。 M4 修复时一并补 `parser_test.go`
- **#M8 aar 同步纪律**: Phase 3B 高频改 `mobile/*.go`, 忘 rebuild aar 会导致 Kotlin Unresolved reference。 Android polish 阶段触及 mobile/ API 的次数会上升, 建议加 pre-commit hook 或 Gradle task mtime check

## 8. 与 NekoBox 的对照总结

| 维度 | NekoBox 现状 | Pilotty v1 目标 | 差距 |
|---|---|---|---|
| 图标 / 品牌 | 完整 mipmap 6 档 + monochrome | M2 完成即达 | 小 |
| 主题 | Light + Dark + 20+ 语言 | S1 + S10 基础设施 | 中 (翻译不做) |
| 协议支持(URI 解析层) | SS/Trojan/VMess/VLess/Hy/Hy2/TUIC/WG/SSH/Naive/Mieru/AnyTLS/ShadowTLS/Trojan-Go | SS/Trojan/VMess/VLess/Hy2/WG + **M10 补齐 TUIC/AnyTLS/ShadowTLS/Hy1/VLESS-flow** | **原报告错写"对齐",实际差 5 个协议(W11-W15 显式不做)** |
| 订阅格式 | Base64-URI / Clash YAML / sing-box JSON / 纯文本 | 当前**仅 Base64-URI**;M11 补 Clash YAML;M12 补 sing-box JSON | **消费级入场券** |
| 节点管理 | Group 隔离 + 手工编辑 + QR 导入导出 + 扫码 | S4 + S6 + S13 + 单 overlay | 显式收窄 |
| 路由 | Route editor + 模板 + App 分流 | S5 + S9 + 已有模板 | 对齐 |
| 监控 | 流量图 + 连接列表 + logcat | S2 + S11 | 对齐 |
| 开关 | Tile + 快捷方式 + BootReceiver | S3 | 对齐 |
| 备份 | BackupFragment + 订阅 YAML | S7 | 对齐 |
| Agent / LLM | ❌ | Chat tab + 硅基流动 | Pilotty 独有 |
| Clash API 兼容 | 内置面板 + yacd 选项 | 内置 Compose UI (S2) | 显式收窄 |

**结论(2026-04-22 修正)**: 原结论"NekoBox 功能覆盖率 ~55%"**过于乐观** —— 协议 URI 解析和订阅格式**没补齐前**实际覆盖不到 35%。M10-M13 落地后才回到 55%。
v1 定位仍是 "**NekoBox / Karing 的 60% 功能 + Agent 的 100%**",但那 60% 里有相当一部分是本轮(M10-M13)必须现场补完的欠账,不是"已有"。

## 9. 与 Karing 的对照总结(2026-04-22 新增)

Karing 是**本项目真正的对手**(同 sing-box 核、同消费级定位、同多订阅格式容忍)。 NekoBox 对比更多是"功能全集参考"。

| 维度 | Karing | Pilotty v1 目标 | 差距 |
|---|---|---|---|
| 协议覆盖 | sing-box 全量 + 多核 | sing-box(单核)+ M10 补齐 | 近平(我们不做多核) |
| 订阅格式 | Clash / sing-box / Surge / URI | **Clash + sing-box(M11 M12 后)** | 近平(不做 Surge) |
| UX 抛光度 | 成熟,图标/主题/多语言完整 | M1-M9 补齐后,50% 水准 | 中 |
| 订阅管理 | 多订阅 + 分组 + 自动更新 | 多订阅 + S8 自动更新 + **不做分组(W2)** | 显式收窄 |
| 节点分享 | QR + URI 全套 | S6 + S13 | 近平 |
| 分流 | 规则模板 + App 分流 + Process | 模板 + S5 App 分流 | 近平 |
| 内核切换 | sing-box / Clash / Xray 可选 | sing-box 独占 | 显式收窄 |
| 自然语言 Agent | ❌ | ✅ | **我们独有** |
| Tool-call 可审计 | ❌ | ✅ | **我们独有** |
| 自动快照 + 回滚 | ❌ | ✅ | **我们独有** |

**Karing 的护城河**:多核 + Surge 兼容 + UX 成熟度
**我们的护城河**:Agent 层(三项)

**Must-have 缺的不是特性,是门槛** —— 不进门,Agent 层用户根本看不到。

---

## 10. Feature Baseline Gap (non-protocol)

协议 / 订阅格式之外,还有一组"用户打开 App 第 5 分钟就会问'怎么没有'"的非协议 table-stakes。 NekoBox / Hiddify / Karing / Clash Meta / v2rayNG 全部都有; Pilotty 当前**一个没做**。

### 10.1 Must-have for v1 release (~3-5 天)

#### M14. Per-App VPN UI
- **现状**: `PilottyVpnService.kt:91-103` 已读取 libbox `TunOptions.includePackage/excludePackage`,但 overlay 里这两个字段无 UI 编辑入口;实际等同"全局接管所有 App 流量"。代码只硬写了 `excludePackage = [self packageName]` 防 VPN 回环
- **竞品做法**:
  - NekoBox: `~/References/nekobox/app/src/main/java/io/nekohasekai/sagernet/ui/AppManagerActivity.kt` + `AppListActivity.kt` —— 完整"应用列表 + 多选 + 搜索" UI,写入 `DataStore.bypassApps`,下发到 VpnService.Builder.addDisallowedApplication
  - Hiddify: `per_app_proxy_settings.dart` 同款选人模式
  - Karing / Clash Meta:App 列表 + 模式切换(白名单/黑名单/关闭)
- **需要**:
  - (a) Settings 页加 "按 App 代理" 开关 + 模式选择 (全局 / 白名单 / 黑名单)
  - (b) 应用列表页:`PackageManager.getInstalledApplications(MATCH_UNINSTALLED_PACKAGES)` + 图标 + label + 多选,过滤掉系统 package
  - (c) 写入 overlay `_agent:perapp.includes / excludes`,下发到 libbox TunOptions 的 `include_package / exclude_package`
  - (d) `PilottyVpnService.onStartCommand` 读 overlay 后用 Android 原生 `VpnService.Builder.addAllowedApplication / addDisallowedApplication` 对 TUN 接口本身做裁剪(libbox 侧 include/exclude 是上层匹配, VpnService.Builder 是系统层路由, 两层互补)
- **工时**: 6-8h(含列表 lazy-load + 图标缓存)
- **用户影响**: 不做 → 想"微信 QQ 直连, 浏览器和 GCM 走代理"的国内用户只能"全走"或"全不走";小众场景但高频诉求

#### M15. Kill Switch (断网保护) ✅ 方式 A 已完成(2026-04-22 真机验证通过)
- **实现**: `android/.../ui/settings/KillSwitchSection.kt` — 新卡片 "防泄漏 · Kill Switch",双层说明文案(为什么需要 / 为什么是引导型)+ 4 步指引 + "打开系统 VPN 设置" 按钮 → `Intent(Settings.ACTION_VPN_SETTINGS)`;挂在 `SettingsScreen` 中 `PerAppVpnSection` 之后
- **真机验证**: Pixel 4a 点按钮 → `topResumedActivity=com.android.settings/.Settings$VpnSettingsActivity`,系统 VPN 列表页显示 Pilotty 条目 + 齿轮图标(用户点齿轮即可进 always-on + lockdown 勾选页)
- **路径选择说明**:
  - 方式 A(已实施): 引导跳转系统设置页,App 不持有非 VPN 流量的阻断权(Android 不允许普通 App)
  - 方式 B(v1.x 延期): BlockingTunService 接管 onDestroy 时挂 0.0.0.0/0 → reject 的 TUN;工程量 10-14h,遗留给 v1.x
- **实际工时**: ~1h(纯 UI Section + Intent,无 Go 改动,无 reload 链,一次 gradle assemble 通过)

#### M16. Deep Link + 剪贴板导入 ✅ 已完成(2026-04-22 真机验证)· QR 延期到 v1.1
- **实现**:
  - Go 侧新 API: `SubscriptionManager.ImportNodeURI(uri)` + `mobile.Client.ImportNodeURI(uri)`(manager.go:91 / netpilot.go 对应新增),直接走 `ParseSubscription + convertNodes + overlay.AddOutboundsBatch + Apply`,不创建 Subscription 条目
  - AndroidManifest.xml MainActivity 加 intent-filter 覆盖 14 个 scheme:`ss / vmess / vless / trojan / tuic / hysteria / hysteria2 / hy2 / wireguard / anytls / shadowtls / sub / clash / pilotty`;`launchMode="singleTask"` 配合 `onNewIntent` 复用实例
  - `ui/import_/ImportBus` StateFlow 单例:MainActivity.handleIncomingUri 收到 intent.data → post,SubsScreen 订阅 pending 弹 `ImportConfirmDialog`(scheme 大写标题 + URI 全文 + 导入/忽略)
  - `SubsViewModel.addSubscription` 分流:`startsWith("http://"|"https://")` → 订阅拉列表;否则 → `importNodeURI` 单节点
  - AddSubscriptionDialog 加剪贴板检测:`pickImportableFromClipboard` 同步读 ClipboardManager 匹配 14 种 scheme 或 http(s),命中时顶部显示"使用剪贴板: xxx"按钮
- **真机验证**(Pixel 4a):
  - `adb shell am start -a android.intent.action.VIEW -d "ss://..."` → logcat `MainActivity: deep-link received: ss://...#TestDeepLink`
  - Subs tab 弹 `ImportConfirmDialog` 标题 "导入节点 · SS",URI 全文显示
  - 点"导入" → `overlay.json` outbounds 数组 15→16, 新增 `{tag:TestDeepLink, type:shadowsocks}`
  - Nodes tab 显示 16/16,TestDeepLink `203.0.113.99:8388` 可见
- **QR 扫码延期**: CameraX + ZXing 依赖较重(~3h 含权限 UI),v1.1 再做。Deep Link + 剪贴板已覆盖 TG/浏览器/手动复制 3 大分享路径
- **实际工时**: ~2.5h(Go 20min + AAR 10min + Kotlin 1h + 真机回归 40min;AAR 重建两次因字段名微调)

#### M17(feat). 订阅流量配额 UI ✅ 已完成(2026-04-22 真机验证通过)
- **实现**:
  - Go 侧 `Subscription.UserInfo` 已随 `Subscriptions()` JSON 透出(store.go:88)
  - Kotlin `SubscriptionDto.userInfo: UserInfoDto?`(Dto.kt:56, 59-65)
  - `SubsScreen.QuotaRow` 渲染横向进度条(accent 填充)+ 已用/总量 + 到期日期(SubsScreen.kt:215-242)
  - Fallback: `sub.userInfo?.let { QuotaRow(it) }` 无 header 时不渲染,UI 不崩
- **真机验证**: 本地 fixture server 在 `/clash-mixed.yaml` 响应附加 `subscription-userinfo: upload=5GB; download=10GB; total=85GiB; expire=2026-05-15` header → "全部更新" → Subs tab 第二卡渲染进度条 ~18% + `已用 15.00 GB / 85.00 GB · 到期 2026-05-15`;第一卡(relaycore.cc 无 header)正确不渲染 QuotaRow
- **原预估** 3-4h,实际 ~0.5h(发现 UI + dto 代码此前已入库,本次只是补了 fixture header 回归验证)

#### M18(feat). 路由规则 CRUD UI ✅ 已完成(2026-04-22 真机验证)
- **实现**:
  - Go: `mobile.Client.AddRule(ruleJSON)` / `RemoveRule(tag)`(netpilot.go 新增),内部 `overlay.AddRule/RemoveRule + Apply(adapter)`;`source` 强制覆盖为 `"user"`,避免客户端伪造成 `"agent"`
  - Kotlin: Rules tab(内嵌在 Settings)Kicker 右侧加 "+ 自定义规则" 按钮,规则卡右侧加红色 "×" 删除按钮 + 二次确认 AlertDialog
  - `AddRuleDialog`: 描述(可选)+ 4 种 matcher 类型 chip(domain_suffix / domain / ip_cidr / process_name)+ 值输入(支持逗号分隔多值)+ 4 种 outbound chip(direct / proxy / proxy-group / reject)
  - Tag 由 `"user-" + System.currentTimeMillis().toString(36)` 自动生成,避免冲突
  - `buildRuleJSON` 本地拼 JSON,按 matcher 类型动态选 `domain_suffix` / `domain` / `ip_cidr` / `process_name` 字段名
- **真机验证**(Pixel 4a):添加 `domain_suffix=test-rule.example.com → direct` → overlay.json `route_rules[0] = {tag:"user-mo9xd7yq", outbound:"direct", source:"user", domain_suffix:["test-rule.example.com"]}` → UI "ACTIVE RULES · 1" + 规则卡显示 `→ direct  suffix×1` + source="user" 标 → 点 × → 二次确认 → 删除 → "ACTIVE RULES · 0"
- **延期项**: 编辑现有规则(当前是只读列表,要编辑需先删再加);outbound 下拉动态填充当前可用 proxy-group 列表(当前是硬编码 4 项);IP CIDR / domain 格式校验(当前后端不报错就算过)
- **实际工时**: ~2h(Go 15min + AAR 10min + Kotlin UI 1h + 真机回归 20min)

### 10.2 Should-have (第一印象,~1.5 天)

#### S14. 订阅到期提醒
- 检测 `expireAt - now < 72h` 时发本地 notification
- 依赖 WorkManager + M17(feat) 的 expire 数据
- 工时: 2-3h

#### S15. Nodes 页面的 Auto-failover 开关
- 后端 `internal/failover/monitor.go` 已实现连续失败 Cooldown 自动切换
- Nodes tab 顶部缺一个开关 toggle + "当前策略" 指示
- 工时: 2h

#### S16. 连接时自动选最快节点
- 非开关, 行为默认值: 用户点"启动 VPN" 且 selector 为"自动"时, 先跑一次 `test_latency_all` 再选最低 latency
- 工时: 2-3h
- 风险: 机场节点数量大时首次启动会卡 5-10s,需要 loading 遮罩

### 10.3 Won't-do for v1

#### W16. WebDAV 云同步
- 理由: v2rayN / Karing 有,nice 但非阻塞;S7 本地备份已覆盖"换设备"场景
- 触发重启: 多设备用户明确要

#### W17. Web dashboard (Yacd 式外部面板)
- 理由: NekoBox / Clash Meta 有 Yacd 选项;我们的 Clash API server 已在 `127.0.0.1:9090`,技术上用户自己跑 Yacd 可以连,但 v1 手机端 Compose 页面足够(S2 流量图 + S11 日志),不专门做
- 触发重启: 手机端 UI 信息密度撑不住

#### W18. Material You 动态取色
- 理由: Android 12+ 独占,S1 Dark/Light 主题已够; 动态取色是 polish 不是 blocker
- 触发重启: v1.1+ 抛光

#### W19. Apple TV / 大屏适配
- 理由: Karing 的独特卖点,我们是 Android-only,没这个战场
- 触发重启: 永不(iOS 都 DEFERRED 了, TV 更别谈)

#### W20. 插件系统
- 理由: NekoBox 有 5 个插件(SSR-Rust / NaïveProxy / Hysteria / Xray-Core / Mieru),是"sing-box 不原生支持协议"的应对。Pilotty 走 sing-box 一个内核到底(见原则 #5 内核可替换 + W5 不自营插件)
- 触发重启: sing-box 原生缺的协议有大量用户

---

## 11. Differentiators to Prominently Showcase

Pilotty 有 3 项**无任一竞品提供**的能力。 这 3 项**必须在 UI 里显眼**(不是藏在 Settings 里),否则 Pilotty 就变成"NekoBox 的 60% 功能 + Karing 的 50% UX", 死在基线战里。

### D1. 自然语言指令入口(Raycast 式体验)✅ 已完成(2026-04-22 真机验证)
- **实现**:
  - Home tab 顶部已有 "Ask the agent" hero 输入框 + 3 个 quick-action chip(最快节点 / Netflix 模式 / 诊断卡顿),原来只是跳 Chat tab 不传入内容
  - 新增 `ui/agent/AgentQueryBus` StateFlow<String?> 单例(同 ImportBus 模式),Home ↑ 按钮 / chip 点击 `post(query)` + nav 跳 Chat
  - `ChatScreen` `LaunchedEffect(pendingQuery)` collect → `vm.send(query)` + `consume()` 自动发起一次 Agent 调用
  - Chip query 文案特意对齐 `internal/router/keywords.go` 的 substring 关键词:"切换节点 找个快的"(→ switch_best_node)、"netflix 分流"(→ apply_template)、"当前状态"(→ show_status),保证 LLM 未配置 apiKey 时本地 IntentRouter 也能闭环执行
- **真机验证**(Pixel 4a,apiKey 空):点"诊断卡顿" → 跳 Chat tab → 用户气泡"当前状态" → Agent pipeline 命中 `show_status` → `localEngine.Execute` → Clash API call(VPN 未启所以 127.0.0.1:9090 connection refused 是预期,证明 pipeline 真跑而非空壳)
- **延期**: "/" 前缀命令选单 + 单行折叠展开动效保留 v1.1;D1 核心 "自然语言 → 真执行" 闭环已达成
- **实际工时**: ~1h(AgentQueryBus 一个文件 + Home 两处 onClick 改动 + Chat LaunchedEffect + chip 文案调整)
- **为什么是关键**: Raycast 的护城河就是"唤起即指令"。 我们的 Agent 藏在 tab 里,用户永远不会意识到它存在

### D2. Safety Card — 写操作随时可回滚 ✅ 已完成(2026-04-22 真机验证)
- **实现**:
  - Go: `mobile.Client.Snapshots()` (返回 `SnapshotStore.List()` JSON) + `Rollback(id)` (走 `pipeline.ManualRollback(id)`,id 空即回滚最新)
  - Kotlin: `SnapshotDto(id, timestamp, activeProxies)` + `PilottyRepository.snapshots() / rollback(id)`
  - `HomeViewModel.refresh` 现在并行拉 status + snapshots,snapshot 失败 fallback 空列表不阻塞;`rollbackLatest()` 发起回滚 + toast
  - Home Safety card 改造:显示 `🛡 Safety · {N} snapshots` + 最近快照 id + 时间戳(`formatSnapTs` 格式化 RFC3339 → `MM-DD HH:MM`)+ activeProxies 键值对 + Rollback 按钮 enabled 仅当有快照
- **真机验证**(Pixel 4a):Home 显示 `🛡 Safety · 3 snapshots` + `最近快照: snap-20260422-002 · 04-22 12:53` + `proxy-group=direct-out` → 点 Rollback → `pipeline.ManualRollback` 找到最新快照 → 尝试通过 Clash API 恢复 `proxy-group → RN-San-Jose-VLESS` → VPN 未启所以 API unreachable 错误正确透传至 UI 红色错误行(VPN 运行时会真回滚)
- **延期**: "每条快照带撤销按钮" 的完整列表页面(当前只有最新快照一键);haptic feedback
- **实际工时**: ~1h(Go mobile 20 行 + AAR 重建 + Dto/Repo wire + Home Card 重写)
- **2026-04-25 升级 (commit efc8a23)**: `Snapshot.Tool` 字段 + `pipeline.Save(adapter, toolName)` + `SnapshotDto.tool` Kotlin 字段; AgentLogCard 优先 `friendlyToolName(snap.tool)` 显示中文友好名 ("切换节点" / "Per-App VPN 配置"),老 snapshot tool 空时回落 `inferActionFromSnapshot` 差分推断兼容

### D3. Agent Action Trace — Chat 内联工具调用时间线 ✅ 已完成 (2026-04-25 真机验证)
- **交付**:
  - Chat 气泡内 timeline: ▸ 折叠头 + ✓/✗ badge + duration + 友好 tool 名 (`set_per_app_vpn` → `Per-App VPN 配置`),21 个 raw tool 名 zh+en 双语映射 (`AgentToolMeta.kt`)
  - Running ▸ 指示: durationMs=0 + 无 output/error 时显示 "…运行中" 占位; ToolEnd 后转 ✓/✗
  - Settings "最近 AGENT 活动" 卡片: 5 条预览 + Dialog 50 条 (跨重启磁盘 telemetry,`AgentActivitySection.kt`); friendlyToolName 共用
  - mobile.Client.RecentTelemetry / TelemetryLogger.LoadRecentFromDisk: 读 telemetry.jsonl tail (内存 ring 进程内才有效, UI 跨重启走磁盘)
- **真机验证 09211JEC204960**: Settings 卡片 5 条预览 friendly 名生效; Dialog 含跨日条目; Chat 历史气泡展开 timeline ✓ + 输出预览
- **commit 4b367de**: 11 文件 +420/-8

### 11.1 总战略: UI 权重分配

| 区域 | 当前权重 | 目标权重 |
|---|---|---|
| Dashboard 状态卡(节点/流量) | 60% | 30% |
| Dashboard 自然语言输入 (D1) | 0% | 30% |
| Dashboard Safety card (D2) | 0% | 20% |
| Dashboard 流量图 (S2) | 40% | 20% |
| Chat tab 内联 tool timeline (D3) | 0% | 必加 |

**Phase 3B-4 的 UI 抛光必须按此目标权重重新设计 Dashboard, 而不是纯粹打磨现有组件**。 抛光方向错,补再多细节也没用。
