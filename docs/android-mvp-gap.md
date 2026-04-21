# Android MVP Gap — v1 发布路径

> **生成日期**: 2026-04-22 · 对应战略转向:iOS 暂停,Android 单平台推到可发布状态
> **基线比对**: NekoBox for Android (`~/References/nekobox/`) 的发布形态
> **当前版本**: `versionCode=1 / versionName=0.1.0`, debug APK 60MB, 真机走流量验证通过 (Pixel 4a, 2026-04-22)

---

## 0. 本文档如何阅读

本文档把"还差什么才能把 APK 发给人用"拆成三档:

- **Must-have (M)**: 不做就不能叫 v1 发布,会被 Google Play 拒审、或会在第一次安装时就让用户装不上 / 用不了
- **Should-have (S)**: 做了明显第一印象更好,NekoBox 这类竞品默认都有。不做也能塞进一个熟人群的小范围内测
- **Won't-do (W)**: 本轮 v1 明确不做,写清楚理由避免后续反复争论

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

### M4. #M17 订阅解析器 SS URL 丢节点
- **现状**: Known Issue #M17。 `internal/subscription/parser.go` 的 SS `ss://...?type=tcp` 情况把 port 当成 `8388?type=tcp` 解析失败, 整条 node 被跳过。实际观察到用户订阅里的 SS 节点大量丢失
- **需要**: 在 parser 的 host:port 提取之前 `strings.SplitN(hostport, "?", 2)[0]`;单测覆盖 v2subscribe / clash / sing-box 三种 SS 格式
- **来源**: CLAUDE.md Known Issues #M17 (今天才识别)
- **工时**: 1h 修复 + 1h 测试
- **用户影响**: 不做 → 用户感觉"订阅里明明有 20 个节点,到 App 里只剩 3 个"。当成 bug 找我们

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

### M8. LLM API Key 配置入口
- **现状**: `mobile.NewClient(filesDir, "127.0.0.1:9090", "")` 第三个参数 apiKey 硬编码空字符串 (`PilottyApp.kt:34`)。Chat tab 能发消息但 Agent 没 key 时会报错,用户不知道怎么配
- **需要**: (a) 设置页加"LLM 配置"卡:输入硅基流动/DeepSeek Key + 保存到 EncryptedSharedPreferences;(b) 启动时读出传给 `PilottyCore.init`
- **来源**: Phase 1 任务 4 的 API 硬写在 env,移动端没有 env
- **工时**: 3-4h (含 EncryptedSharedPreferences 集成 + 首次使用引导)
- **用户影响**: 不做 → Chat tab 在用户眼里就是个报错功能, 反而砸招牌

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

---

## 4. 汇总表

| 档 | 条目 | 工时 | 关键阻塞? |
|---|---|---|---|
| M1 | 签名 + 可发布构建 | 3-4h | **是** |
| M2 | 应用图标 + 应用名 | 1-2h | **是** |
| M3 | 通知文案 + 图标 | 2h | 否 但低成本 |
| M4 | #M17 SS 解析丢节点 | 2h | **是** (数据正确性) |
| M5 | 启动空状态引导 | 2-3h | 否 但砸留存 |
| M6 | 隐私政策 + 权限说明 | 2-3h | **是** (Play policy) |
| M7 | 最小日志导出 | 3h | 否 但砸支持 |
| M8 | LLM API Key 配置 | 3-4h | **是** (Chat tab 否则废) |
| M9 | VPN 异常反馈 | 2-3h | 否 但砸信任 |
| **M 合计** | | **20-26h** | |
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
| **S 合计** | | **43-56h** | |

**建议打包**:
- v1.0 发布 = M1-M9 (~3 天专注工作)
- v1.1 "抛光" = S1 + S3 + S8 (低成本高回报 ≈ 1 天)
- v1.2 "可见性" = S2 + S7 + S11 (≈ 2 天)
- v1.3 "高级特性" = S4 + S5 + S9 (≈ 2-3 天)
- v1.4 "出海准备" = S6 + S10 + 实际翻译 (按需)

## 5. 依赖关系 / 顺序建议

```
M1 签名 ──┐
         ├─→ 必须最先,否则后面的 release 验证都做不了
M2 图标 ──┘

M4 #M17 ──→ 独立, 可并行 (Go 侧 bug)

M6 隐私 ──→ 独立, 可和 docs 团队/本人分头做

M8 API Key ──→ 先做, 因为 Chat tab 依赖它不废
       │
       ▼
M5 空状态 ──→ 在 M8 之后才能给出"Chat 需配 Key"的准确引导

M3 通知 / M7 日志 / M9 异常反馈 ──→ 独立, 穿插

之后依次 S1 → S3 → S8 → S2 → ...
```

## 6. 不在本文档管辖范围的事项

以下与 Android MVP 无直接关联, 但 **Android v1 发布之前必须同步解决**:

- **#H2 Orchestrator rollback**: Agent 三角色编排 Verify 失败路径失效。 CLI 和 Server 都受影响,但 Android 会暴露"自动修复失败时卡死" —— 如果 Chat tab 用 Orchestrator, 这是 UX bug 不是后端 bug
- **#H3 零 go test**: 订阅解析器 1648 行无单测, #M17 漏掉就是这种欠债的表现。 M4 修复时一并补 `parser_test.go`
- **#M8 aar 同步纪律**: Phase 3B 高频改 `mobile/*.go`, 忘 rebuild aar 会导致 Kotlin Unresolved reference。 Android polish 阶段触及 mobile/ API 的次数会上升, 建议加 pre-commit hook 或 Gradle task mtime check

## 7. 与 NekoBox 的对照总结

| 维度 | NekoBox 现状 | Pilotty v1 目标 | 差距 |
|---|---|---|---|
| 图标 / 品牌 | 完整 mipmap 6 档 + monochrome | M2 完成即达 | 小 |
| 主题 | Light + Dark + 20+ 语言 | S1 + S10 基础设施 | 中 (翻译不做) |
| 协议支持 | SS/Trojan/VMess/VLess/Hy/Hy2/TUIC/WG/SSH/Naive/Mieru/AnyTLS | 订阅解析侧全有 (Phase 2.5) | 对齐 |
| 节点管理 | Group 隔离 + 手工编辑 + QR 导入导出 + 扫码 | S4 + S6 + 单 overlay | 显式收窄 |
| 路由 | Route editor + 模板 + App 分流 | S5 + S9 + 已有模板 | 对齐 |
| 监控 | 流量图 + 连接列表 + logcat | S2 + S11 | 对齐 |
| 开关 | Tile + 快捷方式 + BootReceiver | S3 | 对齐 |
| 备份 | BackupFragment + 订阅 YAML | S7 | 对齐 |
| Agent / LLM | ❌ | Chat tab + 硅基流动 | Pilotty 独有 |
| Clash API 兼容 | 内置面板 + yacd 选项 | 内置 Compose UI (S2) | 显式收窄 |

**结论**: Must-have 做完后 = NekoBox 功能覆盖率 ~55%, 但 Pilotty 在 "Agent 驱动 + 中文自然语言 + tool-use 闭环" 这条差异化轴上是 NekoBox 没有的。v1 定位 **不是 NekoBox 克隆**, 是 "NekoBox 的 60% 功能 + Agent 的 100%"。
