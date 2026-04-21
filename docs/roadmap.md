## 开发路线图（第十轮修订 — 反映实际进度）

> 原则：CLI 验证核心 → 模拟器验证 UI + Agent → 真机验证 VPN。每一步都可自动化测试。

---

### Phase 0: 概念验证 ✅ 已完成
- [x] 架构设计（9 轮讨论）
- [x] sing-box Clash API 端到端验证
- [x] libbox iOS framework 编译验证 (gomobile bind)
- [x] 技术栈锁定：Go + sing-box + DeepSeek + iOS

### Phase 1: CLI 原型 — 进行中

**已完成（~5 小时）：**
- [x] 任务 1: EngineAdapter + SingBoxAdapter (Clash API HTTP)
- [x] 任务 2: Intent Router + Local Engine (7 个中文 Action)
- [x] 任务 3: Tool Pipeline + Hook + Snapshot/Rollback + Telemetry
- [x] 任务 4: LLM Agent (DeepSeek tool-use 闭环)
- [x] 任务 5: Config Overlay + 路由规则修改 + 模板系统

**剩余（~3-5 小时）：**
- [ ] 任务 6: 修复 Agent 二轮调用 400 bug ← 正在做
- [ ] 任务 7: Agent 角色分工 (Diagnose/Configure/Verify Orchestrator)
      **参考 Claude Code 源码：** `~/References/claude-code-sourcemap/` 中的：
      - Agent 调度器（agent 如何 fork 子 agent）
      - built-in agents 的 prompt 定义（Explore/Plan/Verification）
      - 主 agent 如何 synthesize 子 agent 结果
- [ ] 任务 8: 对话历史 + 多轮上下文
      **参考 Claude Code 源码：** 消息管理、上下文压缩（compaction）机制
- [ ] 任务 9: 加入真实节点测试（链式代理 detour 验证）
- [ ] 任务 10: 端到端自动化测试脚本

**Phase 1 验证标准：**
- "测速" → Local Engine 直接处理 ✅
- "让 google.com 走 block-out" → Agent 调用 patch_route_rule ✅
- "帮我串联 HK+JP 节点访问银行" → Agent 多步编排 → 配置生效（待真实节点）

---

### Phase 2: 流量感知 (~4-6 小时)

- [ ] Traffic Semantics Engine（Go，按 process/domain/protocol 聚合）
- [ ] 异常检测（滑动窗口 + 冷却期 + 降级/故障分级）
- [ ] Agent 自动修复闭环（检测故障 → Diagnose → Configure → Verify）
- [ ] 主动汇报触发器（硬触发/软触发/静默）

**参考 Claude Code 源码：**
- Hook 系统的事件驱动模式（PostToolUse hook 触发后续动作）
- 后台任务管理（background task lifecycle）

**验证标准：** 手动断节点 → 5 秒内自动检测并切换 + 通知

---

### Phase 3A: iOS App — 无 VPN 版 (~10-15 小时，模拟器可跑)

> **拆分关键：先做不需要 NE 的 iOS App，在模拟器上验证全部 Agent 逻辑和 UI。**

- [ ] Xcode 项目初始化（SwiftUI）
- [ ] Go Agent Core 编译为 iOS framework (gomobile bind)
- [ ] Go ↔ Swift 桥接（Go 启动 HTTP server，Swift 通过 URLSession 调用）
- [ ] SwiftUI 主界面：对话输入框 + Agent 回复气泡
- [ ] SwiftUI 状态卡片：当前节点、延迟、连接数
- [ ] SwiftUI 节点列表页
- [ ] SwiftUI 规则管理页（列表 + 添加模板 + 删除）
- [ ] SwiftUI Agent 操作历史（Telemetry 可视化）
- [ ] LLM Router 四级降级（代理直通 → 硅基流动 → 本地规则 → 本地分类）
- [ ] 冷启动引导流程（导入订阅/手动输入节点）

**这一阶段不需要真机。** 所有 Agent 逻辑、UI 交互、LLM 调用都在模拟器上验证。
sing-box 不在 App 内运行，Go Agent Core 连接 Mac 上运行的 sing-box（开发期间的 mock 模式）。

**参考源码：**
- 不参考 Claude Code（TypeScript/React，参考价值不大）
- 参考 `~/References/hiddify-app/` 的 iOS 目录结构和 gomobile 集成方式

**可自动化：** Xcode UI Testing 框架自动测试所有 UI 交互。

---

### Phase 3A-Android: Android App（优先）~8-15 小时

- [ ] Android Studio 项目初始化（Jetpack Compose + Kotlin）
- [ ] Go Agent Core 编译为 .aar (gomobile bind -target android)
- [ ] Go ↔ Kotlin 桥接（同进程直调，不需要 HTTP server）
- [ ] Jetpack Compose UI（暗色主题，参照 docs/ui-design-spec.md）
- [ ] VpnService 集成（同进程，不需要 NE 拆分）
- [ ] 真机测试（USB 调试，APK 直装）
- [ ] 内测分发（APK 直接分享）

优势：
- VpnService 和 App 在同一个进程，不需要跨进程通信
- 无内存限制（不像 iOS NE 的 50MB）
- 无需开发者账号、无需审批
- APK 直接安装测试

---

### Phase 3B-iOS: iOS VPN 集成（需要付费开发者账号）~20-35 小时

> **这一段必须真机调试，无法完全自动化。**

- [ ] ⚠️ 前置：申请 Apple NE Entitlement（提前做，审批周期不确定）
- [ ] ⚠️ 前置：确定分发策略（非中国区 App Store / TestFlight / 企业签名）
- [ ] NEPacketTunnelProvider 实现
      **参考 Hiddify：** `~/References/hiddify-app/ios/HiddifyPacketTunnel/`
      - `PacketTunnelProvider.swift`
      - `ExtensionProvider.swift`（startTunnel/stopTunnel）
      - `ExtensionPlatformInterface.swift`（TUN 设置、路由、DNS）
- [ ] IPC 四通道实现
      - startVPNTunnel(options:) — 启动时传配置
      - sendProviderMessage() — 运行时查流量
      - App Group 共享文件 — 配置/日志
      - gRPC (libbox 原生) — 实时 stats/groups/logs
- [ ] EngineAdapter 切换：ClashAPIAdapter → LibboxGRPCAdapter
- [ ] NE 内存管理
      - LibboxSetMemoryLimit(true) — Go 软限 30MB
      - 远程 rule-set 替代本地 GeoIP/GeoSite
      - mixed TUN 栈（TCP system + UDP gVisor）
- [ ] 网络切换处理（Wi-Fi ↔ 4G）
- [ ] LLM API bypass 通道（代理自身的 API 请求绕过 VPN）
- [ ] 真机端到端测试

**参考源码：** 不参考 Claude Code。全程参考 Hiddify。
**不可自动化：** 签名配置、真机 VPN 测试、NE 内存调优、App Store 审核

---

### Phase 4: UI 打磨 + 观测性 (~10-15 小时)

- [ ] 流量可视化（实时连接列表）
- [ ] 节点地图 / 链路拓扑图
- [ ] 流量曲线图（上传/下载带宽）
- [ ] Agent 操作时间线（带推理过程展示）
- [ ] 设置界面（信任级别、通知偏好、LLM 模型选择）
- [ ] 节点质量评分系统
- [ ] 用户行为学习（常用场景自动识别）

**参考 Claude Code 源码：**
- TUI 组件架构（组件拆分思路）
- 命令系统 UX 设计

---

## Claude Code 源码参考索引

`~/References/claude-code-sourcemap/` 中与 Pilotty 各阶段对应的模块：

| Pilotty 组件 | 参考 Claude Code 中的 | 阶段 |
|--------------|----------------------|------|
| Agent 主循环 (tool-use loop) | query/agent 主循环, tool_use 处理 | Phase 1 ✅已参考 |
| Agent 角色分工 | built-in agents (Explore/Plan/Verification), AgentTool prompt | Phase 1 任务 7 |
| 动态 System Prompt | `src/constants/prompts.ts` getSystemPrompt() | Phase 1 ✅已参考 |
| Tool 执行管道 | Tool execution pipeline (pre-hook → permission → execute → post-hook) | Phase 1 ✅已参考 |
| 对话上下文管理 | 消息历史管理, context compaction | Phase 1 任务 8 |
| Hook 系统 | `src/hooks/` hook decision 逻辑 | Phase 1 ✅已参考 |
| 权限模型 | Permission system (allow/deny/ask_user) | Phase 1 任务 7 |
| Skill/模板系统 | Skill 加载和 prompt 注入 | Phase 1 ✅已参考 |
| 命令系统 UX | `src/commands.ts` 命令注册和执行 | Phase 4 |

**在 Claude Code 任务提示词中这样引用：**
```
参考 ~/References/claude-code-sourcemap/ 中的 [具体模块]，
理解它的 [具体机制]，然后为 Pilotty 用 Go 重新实现。
不要照搬 TypeScript 代码，理解设计模式后用 Go 惯用方式实现。
```

---

## 时间估算汇总

| 阶段 | 预估小时 | 自动化程度 | 核心产出 |
|------|---------|-----------|---------|
| Phase 1 剩余 | 3-5h | 高 | CLI 功能完整 |
| Phase 2 | 4-6h | 高 | 流量感知 + 自动修复 |
| Phase 3A | 10-15h | 高（模拟器） | iOS App（无 VPN） |
| Phase 3B | 20-35h | 低（需真机） | iOS VPN 完整集成 |
| Phase 4 | 10-15h | 中 | UI 打磨 |
| **总计** | **~50-75h** | | **可测试产品** |

按集中投入（每天 5-8 小时）：约 **8-15 个工作日**。

---

## 真机测试前功能补全计划（基于业务功能矩阵审查）

### 第一批：Go 后端补功能（CLI 验证）~16h

| # | 任务 | 说明 | 工时 |
|---|------|------|------|
| 1 | Reality 协议解析 | VLESS+Reality 是当前使用率最高的协议组合 | 4h |
| 2 | WireGuard 协议解析 | 机场常见协议 | 4h |
| 3 | 链式代理 (detour chain) | Agent 核心卖点："串联 HK+JP 节点" | 3h |
| 4 | Permission 系统 | Agent 写操作前用户确认 | 3h |
| 5 | 订阅流量配额解析 (subscription-userinfo) | HTTP header 解析到期时间/剩余流量 | 2h |

### 第二批：Android 集成 ~23h

| # | 任务 | 说明 | 工时 |
|---|------|------|------|
| 6 | gomobile 编译 Go core 为 .aar | -androidapi 21 | 3h |
| 7 | Android Studio 项目初始化 | Compose + Kotlin | 2h |
| 8 | Go ↔ Kotlin 桥接 | 同进程直调 | 4h |
| 9 | VpnService 集成 | TUN fd + 路由表 + DNS | 6h |
| 10 | Compose UI（4 页面对齐 iOS） | Dashboard/Chat/Nodes/Rules/Settings | 8h |

### 第三批：收尾 ~13h

| # | 任务 | 说明 | 工时 |
|---|------|------|------|
| 11 | 故障自动切换 (URLTest) | 节点不通自动跳下一个 | 6h |
| 12 | 配置备份/导出 | JSON 导出导入 | 3h |
| 13 | 冒烟测试脚本 | 导入→切节点→开VPN→ping | 4h |

总计：~52h
