# CLAUDE.md — Pilotty

> LLM-Agent 驱动的智能网络代理客户端。sing-box 内核 + Go 后端 + Android 端 (iOS DEFERRED)。
>
> **最后更新日期**: 2026-04-22 · 详见本文末尾 Known Issues 节

## 项目概述

Pilotty 是一个能听懂用户意图、实时感知流量状态、自动编排复杂链路的智能网络代理客户端。核心差异:Agent 深度驱动,不只是代理 GUI。

## 命名约定 (2026-04-22 rebrand 后)

项目经历过一次 rebrand: **"NetPilot" → "Pilotty"**。为减少重构爆炸半径,以下 naming 刻意保留不一致:

| 层 | 值 | 改过? |
|---|---|---|
| 品牌 / App 名 / Play 上架名 | **Pilotty** | ✅ 新值 |
| Android 包名 / Bundle ID | `com.pilotty.app` | ✅ 新值 |
| Kotlin root package | `com.pilotty.app` | ✅ 新值 |
| Kotlin 主要类名 | `PilottyApp` / `PilottyCore` / `PilottyVpnService` / `PilottyPlatformInterface` / `PilottyRepository` | ✅ 新值 |
| Go module path | `github.com/foxnetpilot/netpilot` | ❌ 保留旧值 |
| 本地 repo 目录 | `/Users/fox/Netpilot` | ❌ 保留旧值 |
| AAR artifact 文件名 | `android/app/libs/netpilot.aar` | ❌ 保留旧值 |
| Go 源文件名 | `mobile/netpilot.go` etc. | ❌ 保留旧值 |
| CLI 二进制名 | `netpilot` / `netpilot-server` | ❌ 保留旧值 |

**原则**: 用户面 (App 标签、通知、UI 字符串、商店 listing) 必须是 Pilotty;开发者面 (文件路径、Go module、CLI 名) 保留 netpilot 以避免 churn。Go 侧 rename 留到未来用户有需求或准备 1.x 大重构时再做。

**历史档保留**: `docs/phase-3b-plan.md` 和 `docs/phase-3b-4-plan.md` 里剩余的 "NetPilot" 字样**故意保留**,因为它们是 Phase 3B 调研 / 执行的历史凭证(且 phase-3b-4 整本已 DEFERRED)。后续做仓库 grep+replace 时若看到这两份文档里还有 NetPilot,**不要"修正"**,那是刻意留下的时间戳。

---

## 技术栈

- **网络内核**: sing-box
  - Phase 1-3A: 通过 Clash API HTTP 控制**外部 sing-box 二进制**
  - Phase 3B: 目标切换为嵌入式 libbox(gomobile bind)+ gRPC 控制
- **后端语言**: Go(module `github.com/foxnetpilot/netpilot`)
- **LLM**: DeepSeek V3 via 硅基流动(OpenAI 兼容 API,裸 HTTP 调用,无 SDK 依赖)
- **目标平台**: Android (v1 发布目标);iOS 已 DEFERRED,见设计原则 #7
- **Android**: Jetpack Compose + Kotlin + Material3,Go 后端通过 gomobile 编译为 `.aar`
- **iOS**: [DEFERRED] 当前为 SwiftUI + APIClient(HTTP 连本地 server);原 Phase 3B 目标 NEPacketTunnelProvider + 嵌入式 libbox 已冻结
- **架构模式**: 借鉴 Claude Code 的 tool-use 闭环 + Agent 角色分工

## 核心架构(四层)

```
UI Layer (Android Compose / iOS SwiftUI / CLI REPL)
    ↓
Intent Router → 关键词命中走 Local Engine(零 LLM 成本)
              → 未命中 / 含指代词 走 LLM Agent
    ↓
Tool Pipeline (Permission → Pre-Hook → Snapshot → Execute → Post-Hook → Telemetry)
    ↓
EngineAdapter interface → SingBoxAdapter (Clash API HTTP)
    ↓
sing-box 内核(当前:外部进程;Phase 3B:嵌入式 libbox)
```

## 设计原则

1. **Agent 不是黑箱**: 每次操作可审计、可解释、可回滚
2. **用户配置不可侵犯**: Agent 只在 `_agent:` 前缀的 Overlay 区域操作
   - 例外:订阅导入的节点 tag 当前使用节点原名,见 Known Issue #M4
3. **失败安全**: 异常 → 自动回滚 → 上报,不继续尝试
4. **本地优先**: 能本地处理的绝不调 LLM
5. **内核可替换**: 所有 sing-box 依赖收敛到 EngineAdapter 接口(已验证合规,仅 `internal/engine/` 内部允许直连 Clash API)
6. **参考优先**: 涉及 sing-box 内核集成、gomobile bind、VpnService、NEPacketTunnelProvider 等"行业有成熟解法"的工程问题,**先查 `~/References/` 下的 NekoBox / libneko / Hiddify 现有实现,再决定自己怎么做**。每个重要技术决策在 commit message 或 docs 里注明参考来源。若某决策无参考源,显式标注"自造",避免隐性自造轮子。
7. **单平台优先**: 2026-04-22 战略转向 —— Android v1 发布前不投入任何 iOS 工程资源。iOS 相关文档 (`docs/phase-3b-4-plan.md`、`docs/ios-platform.md`)、调研成果、xcframework 都冻结保留,但禁止在本阶段开 iOS 工作分支、加 iOS 相关 Go tag、或让 `mobile/` 层为 iOS 二次抽象。重启信号:Android v1 发布 + 30 天无 P0 崩溃 + 有 >50 DAU + 用户明确要 iOS。Android v1 缺口清单见 `docs/android-mvp-gap.md`。

---

## ⚙️ Self-Planning Protocol(自规划协议)

> **核心目的**:让 Claude Code 在完成任务后自主做规划决策,不把规划工作外包给用户。用户只在关键选型和优先级排序时拍板,不做搬运工。

### 触发条件(满足任一即触发)

- 新建 `docs/*.md` 且 > 100 行
- 代码实现任务完成(编译通过 + 人工可验证的交付)
- 重大重构完成
- 用户明确说 "这个任务结束了" / "done" / "完成了"
- 连续 2+ 个 commit 后仍无阶段汇报

### 执行流程

完成任务后,**立即按以下 4 个 Step 输出**,不等待用户下指令。用 Markdown 分段,每一 Step 标清楚。

---

#### Step 1:交付自评

按以下格式输出:

```
✅ 本次产出: <简述,1-2 句>

📋 DoD 清单:
- [ ] / [x] 每一条 DoD 的完成状态

🎯 质量自评: X/10
扣分理由:
- <具体原因 1>
- <具体原因 2>

⚠️ 隐性未完成:
- 标 ❓ 的假设(列出哪几处)
- 被跳过的步骤(列出原因)
- 未验证的断言(哪些结论没有实证支撑)
```

**硬性要求**:如果质量自评 < 7,Step 2 的第一优先级必须是"先补齐到 7 分",不是推进下一步。

---

#### Step 2:阻塞点识别

列出下一步启动前所有的 unknown / 风险点,**按 ROI 排序**:

```
🚧 阻塞点清单:

#1 [严重性: 高/中/低] <标题>
   问题: <具体描述>
   若卡住损失: <X 小时/天>
   验证成本: <Y 小时>
   ROI: 损失/验证成本 = Z
   是否必须预研: 是/否(ROI > 5 必须是)

#2 ...
```

**硬性规则**:任何 ROI > 5 的阻塞点,必须在 Step 3 里作为候选方向之一出现,不能跳过。

---

#### Step 3:决策建议

提供 2-3 个候选下一步方向:

```
🎯 候选方向:

方向 A(推荐度 ⭐⭐⭐): <标题>
  动作: <具体做什么>
  预估工时: <时间>
  预期产出: <可验证的交付物>
  风险: <有无风险,什么风险>
  推荐理由: <为什么选这个>

方向 B(推荐度 ⭐⭐): <标题>
  ...

方向 C(推荐度 ⭐): <标题>
  ...

💡 个人推荐: 方向 X,理由 <一句话>
```

**硬性规则**:
- 至少 2 个方向,最多 3 个
- 推荐度必须明确(不许全部 ⭐⭐⭐)
- "继续下一个已规划任务" 不是默认方向,要和其他方向同台 PK

---

#### Step 4:等待用户拍板

在 Step 3 后加:

```
⏸️ 等待决策

用户可选:
- "按推荐" / "A" → 执行方向 A
- "B" / "C" → 执行其他方向
- "停" → 暂停,讨论
- "改计划" → 重新评估
```

**硬性规则**:
- 只在 Step 4 后等待。Step 1-3 是自主输出,不等用户
- 用户回复简短决策词后,立即启动新任务,新任务完成后继续循环 Step 1-4
- 如果用户说"按推荐",且推荐动作耗时 < 2 小时 + 低风险,可以合并两个循环(即做完了再一起汇报 Step 1-4)

---

### 协议例外

以下情况**不触发 Self-Planning Protocol**:

1. 纯信息查询 / 简单问答(用户问 "现在的状态是什么")
2. 明确的短任务链(用户说 "做 A 后直接做 B")
3. 用户正在质疑 / 追问细节(这时要答问题,不要切规划模式)

### 协议纪律

- 绝不跳过 Step 1 直接到 Step 3
- 绝不在 Step 1 里注水好评 —— 自评扣分要诚实
- 绝不在 Step 2 里遗漏阻塞点以求"看起来一切顺利"
- 绝不把 "继续按原计划" 当默认方向,每次都要重新评估

---

## 当前进度

### Phase 1 — CLI 原型 ✅ 已完成

| # | 任务 | 状态 | 实现位置 |
|---|------|------|---------|
| 1 | EngineAdapter + SingBoxAdapter + CLI REPL | ✅ | `internal/engine/`, `cmd/netpilot/` |
| 2 | Intent Router + Local Engine(实际 22 个 Action,超原计划 7 个) | ✅ | `internal/router/`, `internal/local/` |
| 3 | Tool Pipeline + Hook + Snapshot/Rollback + Telemetry + Permission | ✅ | `internal/tool/` |
| 4 | LLM Agent(DeepSeek via 硅基流动, tool-use 闭环) | ✅ | `internal/agent/` |
| 5 | Config Overlay + 路由规则修改 + 4 个内置模板 | ✅ | `internal/overlay/`, `internal/template/` |
| 6 | 修复 Agent 二轮调用 bug(纯文本消息格式绕过硅基流动校验) | ✅ | `internal/agent/single_agent.go:40-46, 123-135` |
| 7 | Agent 角色分工(Diagnose/Configure/Verify Orchestrator) | ⚠️ | `internal/agent/roles.go`, `orchestrator.go`(自动回滚路径失效,见 Issue #H2) |
| 8 | 对话历史 + 多轮上下文(摘要注入 system prompt) | ✅ | `internal/agent/history.go`, `prompt.go` |
| 9 | 订阅解析(SS/Trojan/VMess/VLess+Reality/Hysteria2/WireGuard)+ overlay 集成 | ⚠️ | `internal/subscription/`(selector 注册依赖 base config,见 Issue #M6) |
| 10 | 自动化测试 | 🟡 部分 | `scripts/smoke.sh` 端到端冒烟已接通;go test 仍空(#H3) |

### Phase 2 — 超出原计划的交付物

| 模块 | 说明 | 位置 |
|------|------|------|
| HTTP Server | 给移动端 UI 提供控制面 API | `cmd/netpilot-server/` (~900 行) |
| Backup | Overlay + 订阅的 JSON 导出/导入,带 version 兜底 | `internal/backup/backup.go` |
| Failover | 节点健康探测 + 连续失败自动切换(Cooldown) | `internal/failover/monitor.go` |
| Monitor | 终端连接监控(darwin raw termios) | `internal/monitor/` |
| Mobile Binding | gomobile 绑定层(JSON in/out) | `mobile/netpilot.go` (474 行) |
| Permission | Agent 写操作的 ask/auto/strict 信任模式 + CLI `/trust` | `internal/tool/permission.go` |
| Chain Proxy | detour 链式代理 `create_chain` tool | `internal/tool/overlay_tools.go:toolCreateChain` |

### Phase 2.5 — 真机前功能补齐 ✅ 已完成(2026-04-21)

在 3B-3 真机测试前一次性补齐的功能矩阵(对应 `docs/roadmap.md` 真机测试前功能补全计划第一/第三批):

| # | 任务 | 位置 |
|---|------|------|
| 1 | VLESS+Reality(spider_x)URI 解析 | `internal/subscription/parser.go:441` + `converter.go:127-135` |
| 2 | WireGuard URI 解析 + sing-box outbound 产出 | `internal/subscription/parser.go:parseWireGuard` + `converter.go:convertWireGuard` |
| 3 | 链式代理 detour chain(`create_chain` tool) | `internal/tool/overlay_tools.go:toolCreateChain` |
| 4 | Permission 系统 + CLI `/trust` 命令 | `internal/tool/permission.go`, `pipeline.go`, `cmd/netpilot/main.go` |
| 5 | 订阅流量配额 `subscription-userinfo` 头解析 | `internal/subscription/parser.go:ParseUserInfoHeader`, `store.go:UserInfo` |
| 11 | 故障自动切换(URLTest + Cooldown) | `internal/failover/monitor.go` + server endpoints |
| 12 | 配置备份/导出(JSON + version) | `internal/backup/backup.go` + server endpoints |
| 13 | 端到端冒烟脚本 | `scripts/smoke.sh` |

第二批(Android 侧)由 Phase 3A 工程化 + 3B-1/3B-3 代码硬化合并消化。

### Phase 3A — Android/iOS 控制面 🟡 冻结中

**决策**: 继续 UI 细化对用户价值有限(App 还不能真走流量),冻结 Phase 3A 的 UI 迭代,优先推进 Phase 3B。Phase 3A 保留"能编译、5 tab 基本齐"的干净状态,Phase 3B 完成后回来激活。

已完成部分:
- `mobile/` gomobile 绑定层 474 行 ✅
- `android/app/libs/netpilot.aar` 已构建(22.5 MB,仅控制面,**不含 libbox**)✅
- Android Gradle 工程 + 5 tab 底部导航,工程可编译 APK ✅ (2026-04-17)
- Android `PilottyVpnService.kt` 骨架(fd 已通到 Go,待接 libbox)⚠️
- iOS SwiftUI 5 页面 + APIClient ✅(无 VPN 能力)
- iOS NEPacketTunnelProvider ❌ Phase 3B 任务

### Phase 3B — libbox 嵌入式集成 🟡 **3B-1~3 完成,3B-4 已改为 Android 抛光**

**这是 Android 从"壳"变"可发布 App"的关键**。 iOS 已根据原则 #7 暂停。

里程碑拆解:
- [x] **3B-1** 引入 sing-box + 编出含 libbox 的 aar ✅ (2026-04-17, 0.5 天含两轮阻塞预研)
- [x] **3B-2** ~~改造 SingBoxAdapter,新增"嵌入式模式"~~ → **显式接受双轨**,见 Known Issue #M12 (2026-04-17)
- [x] **3B-3** Android VpnService 接入 libbox ✅ (2026-04-22 真机验证通过)
  - [x] Kotlin CommandServer + PlatformInterface 代码硬化 (2026-04-17)
  - [x] #H4 修复:Kotlin `ConfigMerger.ensureTunInbound` 方案 c(1c6e942)
  - [x] 真机前功能补齐(Phase 2.5,见上节)+ 文档对齐 (2026-04-21)
  - [x] 真机 `09211JEC204960` 走流量验证:Chrome/GCM 等 UID 经 TUN → libbox → 香港-1 VLESS → 上游;/connections 观测到 `api.ipify.org:443 via ['香港-1','proxy-group']` 等真连接;延迟测 香港-1 2919ms、日本-1 4786ms (2026-04-22)
  - [x] **Android 端"真能用"闭环**(2026-04-22 同日): 订阅 UI(Settings tab 管理 add/remove/update-all) + 节点列表真数据(type/alive/latency) + 切换节点 + VpnService `ACTION_RELOAD` 热重载 + 自动延迟测速。ipdata.co 真机验证走 San-Jose VLESS 出口。
- [ ] **3B-4** **Android 抛光到可发布 (Polish-to-Shippable)** — 原 iOS NE 任务已 DEFERRED
  - 范围(2026-04-22 二次扩充):`docs/android-mvp-gap.md` 中的 M1-M18 Must-have + D1-D3 AI 护城河 UI (~77-113h 工时 ≈ **7-10 天单人**)
    - M1-M9 (UI/合规/日志):~20-26h
    - **M10-M13 (协议 + 订阅格式基线):~25-31h** — Known Issues #M18 #M19
    - **M14-M18 (非协议功能基线):~25-33h** — Known Issues #M20-#M24(Per-App VPN / Kill Switch / Deep Link+QR / 配额 UI / 规则 CRUD UI)
    - **D1-D3 (AI 护城河 UI):~10-16h** — 自然语言 Dashboard 入口 / Safety card / Chat tool-call timeline
  - 交付:签名 release APK + Google Play Data Safety 就绪 + 消费级协议覆盖 + Clash/sing-box 订阅格式解析 + 5 项非协议 table-stakes(Per-App/Kill Switch/深链/配额/规则)+ AI 护城河 3 项差异化 UI
  - **原计划 M1-M9 only (~3 天) 严重低估**,真实基线按上表 7-10 天;对标不做 = "又一个 NekoBox 克隆"
  - 原 iOS NEPacketTunnelProviderExtension 实现计划见 `docs/phase-3b-4-plan.md` (整本冻结, 顶部已标注 DEFERRED)
- [ ] **3B-5** **iOS (DEFERRED, 无排期)** — Android v1 发布并观察稳定性 30 天后重评估
  - 触发信号:v1 无 P0 崩溃 + DAU > 50 + 用户明确要 iOS
  - 现存资产:`~/References/ios-frameworks/Libbox_std.xcframework` 88MB + `docs/phase-3b-4-plan.md` 调研 + Phase 3A SwiftUI 壳
  - 重启时最小工作量估算:3-5 天 (按原 3B-4 计划) + 苹果开发者账号 $99/年

**参考计划文档**: `docs/phase-3b-plan.md`(含完整 4 合 1 预研踩坑史 + 工具链"sweet spot"决策);`docs/android-mvp-gap.md`(3B-4 实际任务清单)

**3B-1 交付物**:
- `libcore/` package(box.go + platform.go + gomobile_deps.go), 封装 sing-box libbox
- `mobile/netpilot.go` StartTun/StopTun/TunRunning + 新增 SetPlatformInterface
- `scripts/build-aar.sh` 一键产出 `android/app/libs/netpilot.aar` (40MB, 4 架构)
- `go.mod` 降到 go 1.25 + 引入 `github.com/sagernet/sing-box v1.13.8`
- Android `./gradlew :app:assembleDebug` 成功 (APK 60MB debug, release 瘦后预计 ~20-25MB)
- Kotlin `PilottyCore.kt` / `PilottyVpnService.kt` 同步 API 契约
- sing-box libbox 的真启停逻辑挂了 `TODO(3B-3)`, 3B-3 用 `libbox.NewCommandServer` 接入

**战略建议**: ~~3B-3 完成后(Android 能真跑)先停,消化经验,再做 3B-4 iOS。~~ 已按原则 #7 执行:3B-3 完成后 Android 进入 polish-to-shippable (3B-4 范围), iOS 进入 DEFERRED。

## 验证里程碑总表

| 里程碑 | 状态 |
|--------|------|
| sing-box Clash API 端到端可控 | ✅ |
| 中文自然语言 → 本地执行 | ✅ |
| 写操作自动快照 + 失败回滚 | ✅ |
| LLM Agent tool-use 闭环 | ✅ |
| Agent 三角色分工 + Orchestrator 编排 | ⚠️ Verify 失败自动回滚路径失效(#H2) |
| Config Overlay 路由规则增删 + 模板 | ✅ |
| 多轮对话上下文 + /clear /history 命令 | ✅ |
| 订阅解析 + 节点导入(SS/Trojan/VMess/VLess+Reality/Hysteria2/WireGuard) | ✅ (2026-04-21) |
| 订阅流量配额展示(subscription-userinfo) | ✅ (2026-04-21) |
| 链式代理 detour chain(`create_chain` tool) | ✅ (2026-04-21) |
| 写操作 permission 确认(ask/auto/strict) | ✅ (2026-04-21) |
| 节点故障自动切换(URLTest + Cooldown) | ✅ (2026-04-21) |
| 用户数据备份/导出/导入(JSON) | ✅ (2026-04-21) |
| 端到端冒烟脚本(导订阅 → 切节点 → 走代理 → ping) | ✅ (2026-04-21) |
| selector 动态注册导入节点 | ❓ 待验证(#M6) |
| Android 工程能 gradle build 出 APK | ✅ (2026-04-17) |
| netpilot.aar 含嵌入式 sing-box libbox + gvisor/quic/wg/utls/clash_api | ✅ 3B-1 (2026-04-17, 40MB 4-arch aar) |
| Android APK (含 libbox) 能 `assembleDebug` | ✅ 3B-1 (60MB debug) |
| Android 真机走代理 | ✅ 3B-3 (2026-04-22, Pixel 4a `09211JEC204960`, 香港-1 VLESS 走通) |
| Android v1 可发布 (签名 + Play 合规 + 数据正确) | ⏳ 3B-4 (M1-M9, `docs/android-mvp-gap.md`) |
| iOS 真机走代理 | [DEFERRED] 原 Phase 3B-4 整本冻结, 待 Android v1 稳定 30 天后重评估 |

## Known Issues(已知缺陷)

### 🔴 高优先级

**#H4 — overlay merged.json 在 Android 上缺 tun inbound** ✅ **已修(方案 c,1c6e942)**
- 问题: `mobile/netpilot.go:79` NewClient 用 `configs/minimal.json` 作 base(mixed inbound),`internal/overlay/overlay.go:270` Apply() 合出的 `merged.json` 没有 tun inbound,libbox 永不回调 openTun
- 修复: `android/.../vpn/ConfigMerger.kt:ensureTunInbound` 采用**方案 (c) Kotlin 二次合并**
  - `loadConfigJson()` 读 merged.json 后再与 `android_tun_base.json` 合并
  - 注入缺失的 `inbounds[type=tun]` / `outbounds[type=dns]` / `route.auto_detect_interface`
  - **关键细节**:base 的系统级 route rules(`protocol=dns`、`ip_is_private`)prepend 到 merged.rules 前,否则用户规则("抖音 direct")会拦截 DNS 流量,导致 dns-out 失效
  - 选 (c) 而非 (a)(Go NewClient 加 platform hint)的理由:不触发 aar 重建,避开 #M8 同步纪律
- 遗留:[DEFERRED] iOS 重启后需要复刻同等合并逻辑(或改走方案 a 统一收敛)

**#H1 — libbox 数据面真机验证** ✅ **已完成 (2026-04-22)**
- Kotlin `PilottyVpnService` + `PilottyPlatformInterface` + `DefaultNetworkMonitor` 全量接通 libbox
- 关键踩坑: Android P+ `registerDefaultNetworkCallback` 会把 VPN 自己当默认网络返回, 导致 sing-box 上游回环 (表现: UI 已连接但 ping 超时)。修复: API 31+ 用 `registerBestMatchingNetworkCallback(NetworkRequest)`, API 28-30 用 `requestNetwork`, request 不含 VPN capability (需 `CHANGE_NETWORK_STATE` 权限)。抄作业来源: NekoBoxForAndroid `DefaultNetworkListener.kt`
- `getInterfaces()` / `startDefaultInterfaceMonitor` / `closeDefaultInterfaceMonitor` 改为真实现 (原 stub 导致同样的回环)
- 验证数据: `/proxies/香港-1/delay` 2919ms、日本-1 4786ms;`/connections` 观测到 Chrome / GCM / Safe Browsing 全部经 proxy-group
- 遗留: [DEFERRED] iOS 重启后需复刻同等 PlatformInterface 真实现 (NE 侧用 `nw_path_monitor`)

**#H2 — Orchestrator 自动回滚路径名存实亡**
- 位置: `internal/agent/orchestrator.go:86` 调用 `pipeline.Execute(ctx, "rollback", nil)`,但 `internal/tool/tools.go:282-291` 中 rollback tool 是占位实现
- 影响: Verify 阶段失败时,以为系统会自动回滚,实际不会
- 修复方向: 改为 `o.pipeline.ManualRollback("")`,并加集成测试覆盖

**#H3 — 零 go test**(已部分缓解,仍成立)
- 状态: `scripts/smoke.sh` 已接通端到端关键路径(status/sub/switch/latency/proxy-ping/failover/backup),真机前回归有兜底
- 剩余缺口: 全仓库仍无 `_test.go` 文件,订阅解析器(1648 行)格式兼容性回归无保护
- 修复方向: 优先给 `subscription/parser.go`、`overlay/merger.go`、`tool/snapshot.go`、`router/intent_router.go` 写表驱动单测

### 🟡 中优先级

**#M2 — SnapshotStore.Rollback 返回值被忽略**
- 位置: `internal/tool/pipeline.go:106, 132`

**#M3 — Reload() 使用 `pkill -x sing-box`**
- 位置: `internal/engine/singbox_adapter.go:274`
- 注: Phase 3B 切嵌入式 libbox 后该问题自动消失

**#M4 — 订阅节点 outbound 不走 `_agent:` 前缀**
- 位置: `internal/subscription/converter.go`

**#M5 — ask 模式下缺失 approver 时静默拒绝**(文档化,非 bug)
- 位置: `internal/tool/permission.go:79-82`
- 现状: CLI 已注入 `CLIApprover`;server 侧显式使用默认 `TrustAuto` 全放行。真正会撞到的只有"未接 approver 的 ask 模式"边界,目前代码注释已说明"安全失败"语义
- 修复方向: 若未来暴露为 SDK,需在构造函数校验 approver,避免误用

**#M6 — selector 节点注册是静态依赖,不是动态**
- 位置: `internal/subscription/manager.go`
- 验证方法: 打开 `configs/minimal.json`,看 selector 是静态列 tag 还是通配符

**#M7 — 前 9 个任务对 References 利用不足**
- 现象: Agent 循环、VpnService 骨架、mobile/ 绑定层等都是"看过参考项目一眼,自己摸"的水平,没把现成工程经验充分吸收
- 影响: Phase 3B 开始前若不修正方法,会重复撞坑
- 修复: "参考优先原则"(设计原则第 6 条),Phase 3B 每一步都抄作业

**#M8 — aar 与 mobile/*.go 同步纪律依赖人工**(本次修订新增)
- 现象: 修改 `mobile/*.go` 后必须重跑 `scripts/build-aar.sh`,否则 Kotlin 侧会报 Unresolved reference
- 影响: Phase 3B 期间会高频改动 mobile/,这个坑每天都会撞
- 修复方向: 考虑 Gradle task 自动检测 aar mtime,或 pre-commit hook。**Phase 3B 启动后再评估是否优先**,避免过早优化

**#M9 — GOPROXY 经 sing-box TUN 卡死**(Phase 3B-1 预研发现)
- 现象: 本机跑 sing-box/Netpilot 作 TUN 网关时, `go get` / `go list` 访问 `proxy.golang.org` 会被 TUN 阻塞, 实测卡 6 分钟 0% CPU
- 根因: `198.18.0.1` TUN 网关转发的 HTTPS 握手不稳
- 影响: `scripts/build-aar.sh` 在开发机上直接跑会 hang
- 修复: `scripts/build-aar.sh` 开头硬编码 `export GOPROXY="https://goproxy.cn,https://goproxy.io,direct"; export GOSUMDB=off`

**#M10 — gomobile CLI 需要 golang.org/x/mobile/bind 但 go mod tidy 会清掉**(Phase 3B-1 预研发现)
- 现象: `gomobile bind` 报 `"golang.org/x/mobile/bind" is not found`
- 根因: gomobile 工具内部依赖 `bind` 子包, 但用户代码不直接 import, `go mod tidy` 将其视为无用并删除
- 修复: 在 `libcore/gomobile_deps.go` 放 `package libcore` + `import _ "golang.org/x/mobile/bind"`, 显式锁定依赖

**#M11 — sing-box 要求 Go `-ldflags=-checklinkname=0`**(Phase 3B-1 预研发现)
- 现象: `link: github.com/sagernet/sing-box/experimental/libbox: invalid reference to os.checkPidfdOnce`
- 根因: sing-box `experimental/libbox/pidfd_android.go:12` 用 `//go:linkname` 单向访问 Go 标准库 unexported 符号 `os.checkPidfdOnce`; Go 1.23+ 收紧 linkname 规则, 要求双向声明才通过
- 修复: 构建时必须加 `-ldflags='-s -w -checklinkname=0'` 绕过新 linker 检查 (Go 官方提供的逃生舱)
- 关联: sing-box issue #3233, golang issue #70508

**#M12 — 3B-2 显式接受双轨,Go adapter 不做嵌入式改造**(Phase 3B-2 决策, 2026-04-17)
- 背景: 原计划 3B-2 让 `SingBoxAdapter` 在 CLI/Server 模式下走外部 sing-box 二进制, 在移动端走嵌入式 libbox。3B-3 实现过程中发现 Kotlin 直接操作 `libbox.CommandServer` 更自然, Go 侧 `libcore.BoxInstance` 包 libbox 反而是多余中间层。
- 决策: **接受双轨为显式设计** — CLI/Server 继续 Clash API HTTP 外部进程模式; Android/iOS 由平台语言 (Kotlin/Swift) 直接持有 libbox CommandServer 生命周期。Go `libcore.BoxInstance` 保留为占位但不调用, `mobile.Client.StartTun/StopTun` 签名保留但移动端不再使用。
- 影响: 
  - CLI 和移动端代码路径不共享 sing-box 启停逻辑 —— 可接受, 因为语义本来就不同 (CLI 面对开发者/服务器; 移动端面对 VpnService/NE Extension 生命周期)
  - 无需维护 Go 侧嵌入式 adapter 代码, 减 2-3 天工作量
  - 风险: CLI 路径与移动端在 sing-box 行为上可能漂移, 需用一致的 `configs/*.json` + `merged.json` 作为"配置层契约"兜底
- 位置: `internal/engine/singbox_adapter.go` (保持原样); `mobile/netpilot.go:197-252` StartTun/StopTun/TunRunning 空壳; `libcore/box.go` BoxInstance stub; `android/.../PilottyVpnService.kt` Kotlin 直接驱动 libbox

**#M13 — Android UI 连接状态不随 tunRunning 刷新** ✅ **已修 (2026-04-22)**
- 根因: `PilottyCore.tunRunningFlag` 原为 `@Volatile var Boolean`, 不是 StateFlow, Compose 无法订阅;`DashboardViewModel.refresh()` 只在 init 和手动刷新时读一次
- 修复: `PilottyCore` 改 `MutableStateFlow<Boolean>`, `DashboardViewModel.init` 里 launch 一个 collect 协程把值透到 UI state
- 验证: 真机点"启动 VPN" 后 `TUN: 未启动` 立即变 `运行中`, 无需按刷新

**#M14 — Clash API baseURL 不接受裸 host:port** ✅ **已修 (2026-04-22)**
- 根因: Android 从 Kotlin 传 `"127.0.0.1:9090"`(无 scheme), Go `url.Parse` 按 scheme:opaque 解析, 所有 `http.Get(base+"/proxies")` 失败报 `first path segment in URL cannot contain colon`
- 修复: `NewSingBoxAdapter` 检测若不含 `"://"` 自动前置 `http://`
- 位置: `internal/engine/singbox_adapter.go:22-36`

**#M15 — Android 嵌入模式下 `overlay.Apply()` 调 exec("sing-box") 报错** ✅ **已修 (2026-04-22)**
- 根因: CLI 模式下 `SingBoxAdapter.Reload()` 通过 `pkill sing-box && exec sing-box run -c merged.json` 重启外部进程;Android 上没有该二进制(sing-box 嵌在 libbox 里,由 Kotlin `libbox.CommandServer.startOrReloadService()` 驱动),每次订阅导入 / 更新都撞 `exec: "sing-box": executable file not found`
- 修复: `Reload()` 先用 `exec.LookPath("sing-box")` 探测;找不到视为嵌入模式, merged.json 已经写盘, 返回 nil。Kotlin 端 `PilottyVpnService.ACTION_RELOAD` intent 负责通知运行中的 libbox 吃新配置
- 位置: `internal/engine/singbox_adapter.go:269-280`, `android/.../vpn/PilottyVpnService.kt` `reloadIfRunning()`

**#M16 — Clash API `/proxies/<group>` 不含成员延迟 / 类型** ✅ **已修 (2026-04-22)**
- 根因: 该 endpoint 只返回 `{type, now, all:[tag]}`, 无 per-proxy 的 `type`/`alive`/`history`;Nodes 列表 UI 永远显示 `—`
- 修复: `GetProxyGroup` 额外拉一次 `/proxies` 把每个成员的 history 末尾 delay 取出来填 `ProxyInfo.Latency`;`mobile.Client.Nodes()` 再和 `overlay.ListOutbounds()` 合并拿 server/port。Kotlin 端 `NodesViewModel` 首次进页面若发现所有节点 latency=0 且 VPN 在跑, 自动触发 `test_latency_all` 然后刷新(sing-box 不 autoprobe, history 靠 `/delay` 显式填)
- 位置: `internal/engine/singbox_adapter.go:92-155`, `mobile/netpilot.go:327-385`, `android/.../ui/Screens.kt:NodesViewModel.refresh`

**#M17 — 订阅解析器对 `ss://...?type=tcp` 静默跳过** ✅ **已修 + 已加测试(2026-04-22 当日)**
- 修复: `internal/subscription/parser.go:209-291` 进 `parseHostPort` 之前先 `strings.Index(body, "?")` 剥 query,query 参数回填 `node.Extra` / `node.Network`
- 测试: `internal/subscription/parser_test.go` 加 5 个 case(sip002 basic / `?type=tcp` 回归 / `?plugin=obfs-local;obfs=tls` / legacy all-base64 / malformed),覆盖 SIP002 主要格式
- 遗留: 其他 parser(vmess/vless/trojan/hysteria2/wg)同类单测尚未写 —— 归入 `docs/android-mvp-gap.md` M4 的"剩余"项,不在 #M17 本身

**#M18 — 协议 URI 解析器覆盖低于消费级基线** ✅ **已修(2026-04-22 当晚 M10 完成)**
- 修复: `parseLine` switch 扩到 10 个 scheme,新增 `tuic` / `hysteria` / `anytls` / `shadowtls` 四个 parser + 对应 `convertTuic` / `convertHysteria1` / `convertAnyTLS` / `convertShadowTLS` converter;VLESS `flow` (converter.go:116-117)和 `fingerprint` → `tls.utls.fingerprint` (converter.go:137-142) 已 write-through;`spider_x` 不写(sing-box v1.13.8 `option/tls.go:OutboundRealityOptions` 无该字段,验证后刻意 skip,test 锁死)
- 覆盖: `internal/subscription/parser.go:201-470` 新增 4 个 parser, `converter.go:47-200` 新增 4 个 converter + 扩 SummarizeNodes/protocolSuffixRegex
- 测试: `parser_test.go` 新增 9 个 case(TestParseTuic / TestConvertTuic / TestParseHysteria1 / TestConvertHysteria1 / TestParseAnyTLS / TestConvertAnyTLS / TestParseShadowTLS / TestConvertShadowTLS / TestParseVLessReality_Vision)
- 遗留: 真实世界脏数据变体回归还需靠 M13 矩阵脚本 + 真实机场 fixture 补充
- 修复: `docs/android-mvp-gap.md` M10 (~8-12h),在 Phase 3B-4 内完成。参考 `~/References/nekobox/app/src/main/java/io/nekohasekai/sagernet/fmt/` 各子目录 + `moe/matsuri/nb4a/proxy/anytls|shadowtls/`
- 关联: 明确延期的 SSR/Mieru/Naive/SSH/Trojan-Go 见 gap 文档 W11-W15

**#M19 — 订阅格式仅支持 Base64-URI 列表, 不识别 Clash YAML** ✅ **已修(2026-04-22 当晚 M11/M12/M13 完成)**
- M11 Clash / Clash.Meta YAML: `internal/subscription/clash.go` 430 行, `IsClashYAML` 探测(容忍顶层先出现 `mixed-port:` 等 Clash 全局 key) + `ParseClashYAML` + 10 种协议映射 + ws-opts/grpc-opts/reality-opts 回填;参考 NekoBox `RawUpdater.kt:227-243`;依赖 `gopkg.in/yaml.v3`
- M12 sing-box native JSON: `internal/subscription/singbox.go` 200 行, `ParseSingBoxJSON` 反向映射 outbound → NodeConfig,跳过 direct/block/selector/urltest 控制流
- Dispatch: `ParseSubscription` 按优先级 `Clash → sing-box JSON → base64-URI` 探测(parser.go:138-184)
- M13 回归矩阵: `scripts/subscription-matrix-test.sh` + `internal/subscription/matrix_test.go` + `testdata/fixtures/{clash-mixed.yaml, singbox-mixed.json, base64-uri-mixed.txt}`, `--deep` 模式额外用 sing-box binary `check` 验证 (binary 版本需对齐 go.mod)
- 测试: `clash_test.go` (IsClashYAML 6 case + ParseClashYAML 映射全协议 + 端到端 dispatch) + `matrix_test.go` 断言每个 fixture 的 MinNodes/RequiredTypes/RequiredTags
- Android: 不需要 rebuild aar —— 订阅解析走 subscription 包, mobile/netpilot.go 只持有 SubscriptionManager 句柄,新 parser 自动生效
- 遗留: Clash `proxy-providers:` 外部 URL 拉取暂不解析(W 档,非 must);真实机场 fixture 应持续补充 testdata/fixtures/

**#M20 — Per-App VPN UI 缺失**(2026-04-22 对照识别)
- 现象: `android/.../vpn/PilottyVpnService.kt:91-103` 已读取 libbox `TunOptions.includePackage/excludePackage`,但 overlay 这两个字段无 UI 编辑入口;实际"全局接管"
- 影响: 国内高频诉求"微信直连 / 浏览器走代理"完全做不到;NekoBox / Hiddify / Karing / Clash Meta / v2rayNG 均有此功能
- 修复: `docs/android-mvp-gap.md` M14 (~6-8h)。 参考 `~/References/nekobox/app/src/main/java/io/nekohasekai/sagernet/ui/AppManagerActivity.kt` + `AppListActivity.kt`

**#M21 — Kill Switch 缺失**(2026-04-22 对照识别)
- 现象: VPN 断开 / 崩溃时无阻断,流量明文回到物理网络
- 影响: 隐私敏感用户直接劝退。Android 系统自带 `Settings > 网络 > VPN > 始终开启` (lockdown), App 不能直接开但可引导
- 修复: `docs/android-mvp-gap.md` M15 方式 A (~3-4h 引导型) · 方式 B (~10-14h 实现型 BlockingTunService) v1 用方式 A

**#M22 — Deep Link / QR Code 订阅导入缺失**(2026-04-22 对照识别)
- 现象: `AndroidManifest.xml` 无任何 `<intent-filter scheme="ss|vmess|vless|trojan|tuic|..." />`;用户在 TG / 微信点节点链接不会路由给 Pilotty
- 影响: 节点分享路径完全断流,用户必须"长按复制 → 切回 Pilotty → 粘贴",80% 场景直接流失
- 修复: `docs/android-mvp-gap.md` M16 (~7-9h, 含扫码)。参考 NekoBox AndroidManifest.xml:96/108/119 + ScannerActivity

**#M23 — 订阅流量配额 UI 缺失(后端已实现)**(2026-04-22 对照识别)
- 现象: `internal/subscription/parser.go:ParseUserInfoHeader` + `store.go:UserInfo` 解析 upload/download/total/expire 齐了, Kotlin StatusDto + Subscription UI 无透传
- 影响: 用户不知道"还剩多少流量 / 何时到期",被动撞"突然连不上"
- 修复: `docs/android-mvp-gap.md` M17(feat) (~3-4h),纯 UI wire up

**#M24 — 用户侧路由规则 CRUD UI 缺失(仅 Agent 可改)**(2026-04-22 对照识别)
- 现象: Rules tab 能列规则 + 应用 4 个内置模板,但用户无法手工 加 / 删 / 改 单条规则。 后端 Overlay `AddRoutingRule / RemoveRoutingRule` 有,Agent 能用, 普通用户找不到入口
- 影响: "加一条公司内网直连" 这种诉求, Agent 固然能做, 但要求用户先懂 Agent 存在
- 修复: `docs/android-mvp-gap.md` M18(feat) (~6-8h)。参考 NekoBox RouteSettingsActivity

**#M25 — WireGuard 在 sing-box 1.13.8+ 从 outbound 迁到 endpoint, convertWireGuard 过时**(2026-04-22 M13 --deep 真机 validate 捕获)
- 现象: `internal/subscription/converter.go:convertWireGuard` 产出 `{"type":"wireguard", "local_address":[...], "peer_public_key":"..."}` 形态的 outbound 对象。 sing-box 1.13.8+ 把 WireGuard 搬去 endpoints registry (`protocol/wireguard/endpoint.go:33` 用 `option.WireGuardEndpointOptions` 注册),schema 改成 `{"address":[...], "peers":[{"address":...,"public_key":...,"reserved":[...]}]}` 且顶层没有 server/server_port。 当前 converter 产物被 1.13.9 binary `sing-box check` 直接拒 `json: unknown field "local_address"`
- 影响: 订阅里含 WG 节点 → 生成的 merged.json 在 sing-box 1.13.8+ 启动时拒绝加载 → 整份配置挂掉,不光 WG 节点,所有节点都不工作。 严重度中等因为用户订阅里 WG 节点本就稀少
- 修复方向: `convertWireGuard` 改为产出 endpoint 对象而非 outbound;overlay/merger 需要区分 outbounds 和 endpoints 两段,新增 endpoint 写入路径。 范围影响到 `internal/overlay/` 的 merger 层,不只是 subscription。 预估 4-6h。 暂定 Phase 3B-4 M10 增量修复,本轮 fixture 先把 WG 从 clash-mixed 移除绕过
- 遗留预防: `scripts/subscription-matrix-test.sh --deep` 已跑绿,未来 sing-box 继续变 schema 时会第一时间被 binary check 逮住

### 🟢 轻微

- **#L1** ✅ 已修(2026-04-22 commit 8dc4576): `gofmt -w .` 清零了最后 5 个违规文件
- **#L2** `SingBoxAdapter.IsRunning()` 永远返回 false
- **#L3** `history.go` 40 条上限、2000 字符摘要限制硬编码
- **#L4** `classifier.go:78` 未分类请求默认走完整三角色流水线,成本未必合理
- **#L5** `manager.go:337-383` 的 `containsAny`/`findSubstring`/`trimPrefix` 是重造标准库
- **#L6** `Icons.Filled.Rule` 已 deprecated,应迁移到 `Icons.AutoMirrored.Filled.Rule`

## 详细设计文档

各模块完整实现方案,按需引用:

| 文档 | 内容 | 何时引用 |
|------|------|---------|
| `docs/architecture.md` | 四层架构详细设计、Intent Router/Local Engine 实现 | 修改架构时 |
| `docs/agent-runtime.md` | Agent 运行时、Tool Pipeline、Hook、角色分工、Orchestrator | 改 Agent / Pipeline 时必读 |
| `docs/ios-platform.md` | [DEFERRED] iOS NE 实现、进程架构、IPC、Entitlement(含 Hiddify 验证数据) | iOS 重启后再用 |
| `docs/phase-3b-4-plan.md` | [DEFERRED] 原 iOS NE 执行计划,整本冻结 | iOS 重启后再用 |
| `docs/android-mvp-gap.md` | **Android v1 发布缺口分析 (M/S/W 三档)** | 3B-4 抛光期间持续参考 |
| `docs/intent-pipeline.md` | 自然语言 → sing-box 配置的四层转译 | 做意图解析时 |
| `docs/traffic-engine.md` | 流量感知引擎、Sniffing metadata | 流量感知特性开发时 |
| `docs/engine-evolution.md` | 内核演进路线、EngineAdapter 完整接口、三阶段替换计划 | Phase 3B 必读 |
| `docs/phase-3b-plan.md` | **Phase 3B 执行计划(3B-1 调研阶段产出,当前推进依据)** | 整个 Phase 3B 期间 |
| `docs/business-model.md` | LLM 成本估算、收费模型、商业模型 | 产品决策时 |
| `docs/roadmap.md` | 完整开发路线图(Phase 0-4) | 规划时 |

## 代码规范

- Go 标准项目布局:`cmd/` 入口、`internal/` 业务逻辑、`mobile/` gomobile 绑定
- 所有 sing-box 交互通过 EngineAdapter 接口,禁止直接调用 Clash API
  - 例外豁免:`internal/engine/` 内部实现、`internal/subscription/parser.go`(下载订阅)、`internal/agent/llm_client.go`(LLM API)
- 写操作必须经过 Tool Pipeline(Permission → Pre-Hook → Snapshot → Execute → Post-Hook → Telemetry)
- Agent 生成的配置资源使用 `_agent:` 前缀
- 错误处理:不 panic,返回友好错误信息;rollback 返回值必须 check
- 提交前执行 `gofmt -w .` 和 `go vet ./...`
- 中文注释 / 英文代码 / 英文 commit message
- **参考优先**: sing-box 集成相关工程问题必须抄 `~/References/` 的作业,commit 里注明参考来源
- **自规划协议**: 完成里程碑任务后,按 Self-Planning Protocol 自主输出 Step 1-4,不等用户下指令

## Reference 资源清单

本机 `~/References/` 目录下的参考项目,**Phase 3B 期间必读**:

| 路径 | 用途 | 优先级 |
|------|------|-------|
| `~/References/nekobox/` | NekoBox for Android 主仓(完整版),含 libcore/ 子目录 + app/ 主 App | ⭐⭐⭐ Phase 3B 全程 |
| `~/References/nekobox/libcore/` | sing-box gomobile 绑定层,Phase 3B-1 抄作业首选 | ⭐⭐⭐ Phase 3B-1 |
| `~/References/nekobox/app/` | Android 主 App,VpnService 集成权威参考 | ⭐⭐⭐ Phase 3B-3 |
| `~/References/libneko/` | 跨平台工具集,非 libcore;Phase 3B-3 可能用其 protect_server/ 子包 | ⭐⭐ Phase 3B-3 |
| `~/References/hiddify-app/` | [DEFERRED] Hiddify 全栈,iOS NEPacketTunnelProvider 唯一参考 | iOS 重启后再用 |
| `~/References/claude-code-sourcemap/` | Claude Code 还原源码,Agent 循环 / Tool 调度参考 | ⭐⭐ Agent 优化时 |
| `~/References/claude-code-deep-dive/` | Claude Code 深度分析 | ⭐⭐ Agent 优化时 |
| `~/References/ai-agent-deep-dive/` | 通用 Agent 设计分析 | ⭐ 架构演进时 |
| `~/References/ios-frameworks/` | [DEFERRED] iOS 预编译 xcframework (Libbox_std 88MB) | iOS 重启后再用 |
| `~/References/test-gomobile-ios.sh` | [DEFERRED] 先前做过的 gomobile iOS 构建测试脚本 | iOS 重启后再用 |
| `~/References/test-singbox-api.sh` | 先前做过的 sing-box API 测试脚本 | ⭐⭐ Phase 3B-2 参考 |

**DeepWiki 捷径**: https://deepwiki.com/MatsuriDayo/NekoBoxForAndroid/ —— NekoBox 整仓库的结构化逆向文档,Phase 3B 期间优先查

## 参考资源(外部)

- [sing-box 官方文档](https://sing-box.sagernet.org/)
- [sing-box Clash API](https://sing-box.sagernet.org/configuration/experimental/clash-api/)
- [sing-box libbox 源码](https://github.com/SagerNet/sing-box/tree/main/experimental/libbox)
- [Anthropic Tool Use 文档](https://docs.anthropic.com/en/docs/build-with-claude/tool-use)
- [NekoBox GitHub](https://github.com/MatsuriDayo/NekoBoxForAndroid)
