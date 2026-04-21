# CLAUDE.md — NetPilot

> LLM-Agent 驱动的智能网络代理客户端。sing-box 内核 + Go 后端 + Android/iOS 双端。
>
> **最后更新日期**: 2026-04-22 · 详见本文末尾 Known Issues 节

## 项目概述

NetPilot 是一个能听懂用户意图、实时感知流量状态、自动编排复杂链路的智能网络代理客户端。核心差异:Agent 深度驱动,不只是代理 GUI。

## 技术栈

- **网络内核**: sing-box
  - Phase 1-3A: 通过 Clash API HTTP 控制**外部 sing-box 二进制**
  - Phase 3B: 目标切换为嵌入式 libbox(gomobile bind)+ gRPC 控制
- **后端语言**: Go(module `github.com/foxnetpilot/netpilot`)
- **LLM**: DeepSeek V3 via 硅基流动(OpenAI 兼容 API,裸 HTTP 调用,无 SDK 依赖)
- **目标平台**: Android 优先,iOS 跟进
- **Android**: Jetpack Compose + Kotlin + Material3,Go 后端通过 gomobile 编译为 `.aar`
- **iOS**: 当前为 SwiftUI + APIClient(HTTP 连本地 server);Phase 3B 目标为 NEPacketTunnelProvider + 嵌入式 libbox
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
- Android `NetPilotVpnService.kt` 骨架(fd 已通到 Go,待接 libbox)⚠️
- iOS SwiftUI 5 页面 + APIClient ✅(无 VPN 能力)
- iOS NEPacketTunnelProvider ❌ Phase 3B 任务

### Phase 3B — libbox 嵌入式集成 🟡 **3B-3 真机验证今天进行**

**这是 iOS 可用性的必经关卡,也是 Android 从"壳"变"可用 App"的关键**。

里程碑拆解:
- [x] **3B-1** 引入 sing-box + 编出含 libbox 的 aar ✅ (2026-04-17, 0.5 天含两轮阻塞预研)
- [x] **3B-2** ~~改造 SingBoxAdapter,新增"嵌入式模式"~~ → **显式接受双轨**,见 Known Issue #M12 (2026-04-17)
- [x] **3B-3** Android VpnService 接入 libbox ✅ (2026-04-22 真机验证通过)
  - [x] Kotlin CommandServer + PlatformInterface 代码硬化 (2026-04-17)
  - [x] #H4 修复:Kotlin `ConfigMerger.ensureTunInbound` 方案 c(1c6e942)
  - [x] 真机前功能补齐(Phase 2.5,见上节)+ 文档对齐 (2026-04-21)
  - [x] 真机 `09211JEC204960` 走流量验证:Chrome/GCM 等 UID 经 TUN → libbox → 香港-1 VLESS → 上游;/connections 观测到 `api.ipify.org:443 via ['香港-1','proxy-group']` 等真连接;延迟测 香港-1 2919ms、日本-1 4786ms (2026-04-22)
- [ ] **3B-4** iOS NEPacketTunnelProviderExtension 从零实现(3-5 天)
- [ ] **3B-5** 双端真机联调 + 稳定性修复(2-3 天)

**参考计划文档**: `docs/phase-3b-plan.md`(含完整 4 合 1 预研踩坑史 + 工具链"sweet spot"决策)

**3B-1 交付物**:
- `libcore/` package(box.go + platform.go + gomobile_deps.go), 封装 sing-box libbox
- `mobile/netpilot.go` StartTun/StopTun/TunRunning + 新增 SetPlatformInterface
- `scripts/build-aar.sh` 一键产出 `android/app/libs/netpilot.aar` (40MB, 4 架构)
- `go.mod` 降到 go 1.25 + 引入 `github.com/sagernet/sing-box v1.13.8`
- Android `./gradlew :app:assembleDebug` 成功 (APK 60MB debug, release 瘦后预计 ~20-25MB)
- Kotlin `NetPilotCore.kt` / `NetPilotVpnService.kt` 同步 API 契约
- sing-box libbox 的真启停逻辑挂了 `TODO(3B-3)`, 3B-3 用 `libbox.NewCommandServer` 接入

**战略建议**: 3B-3 完成后(Android 能真跑)先停,消化经验,再做 3B-4 iOS。

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
| iOS 真机走代理 | ❌ Phase 3B-4 |

## Known Issues(已知缺陷)

### 🔴 高优先级

**#H4 — overlay merged.json 在 Android 上缺 tun inbound** ✅ **已修(方案 c,1c6e942)**
- 问题: `mobile/netpilot.go:79` NewClient 用 `configs/minimal.json` 作 base(mixed inbound),`internal/overlay/overlay.go:270` Apply() 合出的 `merged.json` 没有 tun inbound,libbox 永不回调 openTun
- 修复: `android/.../vpn/ConfigMerger.kt:ensureTunInbound` 采用**方案 (c) Kotlin 二次合并**
  - `loadConfigJson()` 读 merged.json 后再与 `android_tun_base.json` 合并
  - 注入缺失的 `inbounds[type=tun]` / `outbounds[type=dns]` / `route.auto_detect_interface`
  - **关键细节**:base 的系统级 route rules(`protocol=dns`、`ip_is_private`)prepend 到 merged.rules 前,否则用户规则("抖音 direct")会拦截 DNS 流量,导致 dns-out 失效
  - 选 (c) 而非 (a)(Go NewClient 加 platform hint)的理由:不触发 aar 重建,避开 #M8 同步纪律
- 遗留:iOS 3B-4 需要复刻同等合并逻辑(或改走方案 a 统一收敛)

**#H1 — libbox 数据面真机验证** ✅ **已完成 (2026-04-22)**
- Kotlin `NetPilotVpnService` + `NetPilotPlatformInterface` + `DefaultNetworkMonitor` 全量接通 libbox
- 关键踩坑: Android P+ `registerDefaultNetworkCallback` 会把 VPN 自己当默认网络返回, 导致 sing-box 上游回环 (表现: UI 已连接但 ping 超时)。修复: API 31+ 用 `registerBestMatchingNetworkCallback(NetworkRequest)`, API 28-30 用 `requestNetwork`, request 不含 VPN capability (需 `CHANGE_NETWORK_STATE` 权限)。抄作业来源: NekoBoxForAndroid `DefaultNetworkListener.kt`
- `getInterfaces()` / `startDefaultInterfaceMonitor` / `closeDefaultInterfaceMonitor` 改为真实现 (原 stub 导致同样的回环)
- 验证数据: `/proxies/香港-1/delay` 2919ms、日本-1 4786ms;`/connections` 观测到 Chrome / GCM / Safe Browsing 全部经 proxy-group
- 遗留: iOS 3B-4 需复刻同等 PlatformInterface 真实现 (NE 侧用 `nw_path_monitor`)

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
- 位置: `internal/engine/singbox_adapter.go` (保持原样); `mobile/netpilot.go:197-252` StartTun/StopTun/TunRunning 空壳; `libcore/box.go` BoxInstance stub; `android/.../NetPilotVpnService.kt` Kotlin 直接驱动 libbox

**#M13 — Android UI 连接状态不随 tunRunning 刷新**(Phase 3B-3 真机发现, 2026-04-22)
- 现象: `NetPilotVpnService.startService()` 成功 (openTun fd 建立, libbox 跑起来, tun0 拿到地址, /connections 显示真流量走 proxy-group) 后 UI 仍显示"未连接"
- 根因待查: `NetPilotCore.markTunRunning(true)` 是被调用了, 但 Composable 读的那个 state flow 没刷新 —— 推测是 MutableStateFlow / Jetpack Compose 的观测链路断了
- 影响: 仅 UI 视觉, 不影响真流量;用户体验坏 (以为点了没生效, 会反复点触发 startService 重入 —— 幂等保护已覆盖这一副作用)
- 位置: `android/app/src/main/java/com/foxnetpilot/netpilot/NetPilotCore.kt` + 对应 Composable
- 修复方向: 查 markTunRunning 改的是哪个 state, 观测侧是不是同一个实例;若不是, 改用 StateFlow 全局单例

### 🟢 轻微

- **#L1** gofmt 13 文件违规,`gofmt -w .` 一次性可修复
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
| `docs/ios-platform.md` | iOS NE 实现、进程架构、IPC、Entitlement(含 Hiddify 验证数据) | Phase 3B-4 时 |
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
| `~/References/hiddify-app/` | Hiddify 全栈,iOS NEPacketTunnelProvider 唯一参考 | ⭐⭐⭐ Phase 3B-4 |
| `~/References/claude-code-sourcemap/` | Claude Code 还原源码,Agent 循环 / Tool 调度参考 | ⭐⭐ Agent 优化时 |
| `~/References/claude-code-deep-dive/` | Claude Code 深度分析 | ⭐⭐ Agent 优化时 |
| `~/References/ai-agent-deep-dive/` | 通用 Agent 设计分析 | ⭐ 架构演进时 |
| `~/References/ios-frameworks/` | iOS 预编译 framework 产物(用途待确认) | ❓ 检查后定 |
| `~/References/test-gomobile-ios.sh` | 先前做过的 gomobile iOS 构建测试脚本 | ⭐⭐⭐ Phase 3B-1 必读 |
| `~/References/test-singbox-api.sh` | 先前做过的 sing-box API 测试脚本 | ⭐⭐ Phase 3B-2 参考 |

**DeepWiki 捷径**: https://deepwiki.com/MatsuriDayo/NekoBoxForAndroid/ —— NekoBox 整仓库的结构化逆向文档,Phase 3B 期间优先查

## 参考资源(外部)

- [sing-box 官方文档](https://sing-box.sagernet.org/)
- [sing-box Clash API](https://sing-box.sagernet.org/configuration/experimental/clash-api/)
- [sing-box libbox 源码](https://github.com/SagerNet/sing-box/tree/main/experimental/libbox)
- [Anthropic Tool Use 文档](https://docs.anthropic.com/en/docs/build-with-claude/tool-use)
- [NekoBox GitHub](https://github.com/MatsuriDayo/NekoBoxForAndroid)
