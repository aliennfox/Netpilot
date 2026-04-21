# Phase 3B-4 执行计划 · iOS NEPacketTunnelProvider 落地

> 状态: **调研完成,代码实现待用户介入** (原版 2026-04-17, 3B-3 debrief 补 2026-04-22)
> 主要抄作业对象: **Hiddify / hiddify-app** (`~/References/hiddify-app/ios/`)
> 次要参考: sing-box-for-apple 官方客户端 (GitHub, 未本地克隆)
>
> **3B-3 真机收官后新发现**: 详见第 14 节 debrief, 直接影响第 5 节 PlatformInterface 的范围。

---

## 1. 执行摘要

3B-4 目标: 给 NetPilot iOS 端做出**真能走流量的 VPN 客户端**,而不是 Phase 3A 的"壳"(SwiftUI + APIClient 打 127.0.0.1 Go server,无数据面)。

两条关键路径已打通的关键发现:

1. **xcframework 已存在**: `~/References/ios-frameworks/Libbox_std.xcframework/` 是纯 sing-box libbox 的 gobind 产物,88MB,含 `ios-arm64` 真机 + `ios-arm64_x86_64-simulator` 两套。Headers: `Libbox.objc.h` (1331 行, Objective-C 桥接模式)
2. **Hiddify 的 iOS 是跟 Android 完全同构的双轨设计**: 主 app (`Runner`) + `.appex` (HiddifyPacketTunnel) 两个 target, extension 单独嵌入 xcframework, extension 进程持有 libbox CommandServer

3B-4 交付物(**等用户介入后完成**):
- `ios/NetPilot.xcodeproj` 新增 `NetPilotPacketTunnel` target (Network Extension type)
- `ios/NetPilotPacketTunnel/PacketTunnelProvider.swift` + `PlatformInterface.swift` (抄 Hiddify, 约 500-800 行)
- `ios/NetPilot/VPN/VPNManager.swift` (主 App 侧, 抄 Hiddify VPNManager.swift, ~200 行)
- `scripts/build-xcframework.sh` (构建 `NetPilotCore.xcframework` 含 libbox + libcore + mobile 三包;**或**直接复用 `~/References/ios-frameworks/Libbox_std.xcframework` 做 MVP)
- `ios/project.yml` 更新 targets + entitlements + App Group
- 真机验证: Xcode → 插 iPhone → Build & Run → VPN 权限对话框 → 实跑流量

---

## 2. 总体架构

iOS 双 target 模式,与 Android 单 APK 两组件 (主 Activity + VpnService) 语义等价:

```
NetPilot.app  (Main App target)
├── SwiftUI 5 页面 (Phase 3A 已有: Dashboard/Chat/Nodes/Rules/Settings)
├── APIClient (HTTP 127.0.0.1 → Go server,保留,但服务改为走本地 file)
├── VPNManager.swift ← 新增
│    └── NETunnelProviderManager → startVPNTunnel(options:)
└── Entitlement: networkextension (allow-vpn)

NetPilotPacketTunnel.appex  (NE Extension target, ~15MB 独立进程)
├── PacketTunnelProvider: NEPacketTunnelProvider
│    └── 持有 LibboxCommandServer → libbox 数据面
├── NetPilotPlatformInterface: LibboxPlatformInterfaceProtocol
│    └── openTun() → NEPacketTunnelNetworkSettings → setTunnelNetworkSettings
└── Entitlement: packet-tunnel-provider + application-groups

共享 App Group (双 target 都必须声明同一 group.*):
└── group.com.foxnetpilot.NetPilot
    ├── configs/merged.json (主 App 写, Extension 读)
    ├── configs/ios_tun_base.json (Bundle 打包, Extension 读)
    └── logs/ (Extension 写, 主 App 读显示)
```

**关键约束**: iOS NE Extension 进程受系统严格沙箱化,**不能直接通过 IPC 管道与主 App 通信**。只能用两种机制:
- **App Group 共享文件系统** (`FileManager.default.containerURL(forSecurityApplicationGroupIdentifier:)`)
- **sendProviderMessage/handleAppMessage** (NETunnelProviderSession 内置 RPC, 单次请求-响应)

数据流:
1. 主 App Agent 生成 merged sing-box config → 写入 App Group 共享目录
2. 用户点"开启 VPN" → `VPNManager.connect(config: merged)` → `startVPNTunnel(options: ["Config": merged])`
3. iOS 拉起 Extension 进程, `PacketTunnelProvider.startTunnel(options:)` 收到 config
4. Extension 内: `Libbox.setup` → `Libbox.newCommandServer` → `server.startOrReloadService(config, OverrideOptions())` → sing-box 调用 `openTun` → Extension 返回 TUN fd
5. 主 App 周期性 `sendProviderMessage("stats")` 查询流量, Extension 在 `handleAppMessage` 里返回 `"upload,download"`

---

## 3. xcframework 选型

### 3.1 候选

| 方案 | 来源 | 体积 | 工作量 | 对齐 Android |
|---|---|---|---|---|
| A | 复用 `~/References/ios-frameworks/Libbox_std.xcframework` | 88MB | 0h | ❌ (Android 的 aar 含 libbox+libcore+mobile 三包, iOS 只有 libbox) |
| B | 自己跑 `gomobile bind -target=ios ./libcore ./mobile github.com/sagernet/sing-box/experimental/libbox` → 产出 `NetPilotCore.xcframework` | ~140MB | 4-8h (含 Xcode 链路排坑) | ✅ 完全对齐 Android aar |
| C | 方案 A + 主 App 通过 HTTP 打本地 Go server (Phase 3A 模式) 拿业务 API,不 bind mobile 到 iOS | 88MB | 2-4h | ⚠️ 主 App 和 Extension 架构不一致 |

### 3.2 决策: **先走 C 做 MVP,B 作为 3B-5 升级目标**

理由:
1. C 能让 3B-4 在 1-2 天内真跑代理;B 要在 macOS 上踩 gomobile iOS 工具链的坑(`gomobile bind -target=ios` 在 macOS 外无法跑,linker checklinkname 坑可能复现)
2. Phase 3A 的 iOS app 已经实现 HTTP 打 Go server 拿 status/nodes/chat,这套架构能继续用 —— 主 App 只需新增"生成 merged config 并写入 App Group"的能力(调 Go `ExportConfig` API)
3. Extension 进程只管数据面,用现成 `Libbox_std.xcframework` 即可,不需要 mobile.Client
4. B 在 3B-5 再做,届时可以同时验证 xcframework 自建流程 + 双端架构对齐

**后果**: 主 App Swift 代码暂不能直接调 NetPilot Go 业务层,保持 HTTP 调用;iOS 用户需要 Go server 跑在本机才能用 —— 但 iOS 上 Go server 也在 Extension 进程或主 App 进程里,具体放哪待定。这个坑留到 B 阶段重写。

### 3.3 MVP 下的 xcframework 挂载

- `ios/Frameworks/Libbox_std.xcframework` ← 从 `~/References/ios-frameworks/` 复制进仓库(或 .gitignore 并用 symlink,需用户决定)
- `ios/project.yml` → `NetPilotPacketTunnel` target 添加 `packages:` 或 `dependencies:` 引用 xcframework
- `ios/NetPilotPacketTunnel/Module.modulemap` 或 Bridging Header 引 `Libbox.objc.h`

---

## 4. 目录骨架规划

```
ios/
├── project.yml                          # 修改: 新增 NetPilotPacketTunnel target + App Group
├── Frameworks/                          # 新增
│   └── Libbox_std.xcframework/          # 从 ~/References/ 复制或 symlink
├── NetPilot/                            # Phase 3A 已有
│   ├── NetPilotApp.swift
│   ├── ContentView.swift
│   ├── Services/
│   │   └── APIClient.swift              # 保留 (HTTP 打 Go server)
│   ├── VPN/                             # 新增
│   │   ├── VPNManager.swift             # NETunnelProviderManager 管理 (抄 hiddify VPNManager.swift:26-236)
│   │   ├── VPNConfig.swift              # 从 merged.json 生成 NETunnelProviderProtocol
│   │   └── AppGroupFile.swift           # 封装 App Group 容器访问
│   ├── Info.plist
│   └── NetPilot.entitlements            # 新增: app-groups + allow-vpn + packet-tunnel-provider
├── NetPilotPacketTunnel/                # 新增 target 根目录
│   ├── Info.plist                       # NSExtension.NSExtensionPointIdentifier = com.apple.networkextension.packet-tunnel
│   ├── NetPilotPacketTunnel.entitlements
│   ├── PacketTunnelProvider.swift       # NEPacketTunnelProvider 子类 (抄 hiddify PacketTunnelProvider + ExtensionProvider)
│   ├── NetPilotPlatformInterface.swift  # LibboxPlatformInterfaceProtocol 实现 (抄 hiddify ExtensionPlatformInterface.swift:12-511)
│   ├── ConfigMerger.swift               # 与 Android 的 ConfigMerger.kt 镜像: 注入 tun inbound 到 merged config
│   └── NetPilotPacketTunnel-Bridging-Header.h  # import <Libbox_std/Libbox.objc.h>
└── Shared/                              # 新增, 主 App 和 Extension 共享代码
    ├── FilePath.swift                   # App Group 容器路径辅助
    └── LogWriter.swift                  # 共享日志写入
```

---

## 5. PacketTunnelProvider 实现要点

核心流程完全抄 Hiddify 的 ExtensionProvider + sing-box-for-apple (SFA) 官方实现. 关键步骤:

### 5.1 startTunnel 流程

参照 `~/References/hiddify-app/ios/HiddifyPacketTunnel/SingBox/ExtensionProvider.swift:16-88`

```swift
override open func startTunnel(options: [String: NSObject]?) async throws {
    let config = options?["Config"] as? NSString as? String ?? ""
    
    // 1. 创建工作目录 (App Group 共享)
    try createRequiredDirectories()
    
    // 2. 初始化 PlatformInterface
    platformInterface = NetPilotPlatformInterface(self)
    
    // 3. Libbox.setup 全局状态
    let opts = LibboxSetupOptions()
    opts.basePath = FilePath.sharedDirectory.relativePath
    opts.workingPath = FilePath.workingDirectory.relativePath
    opts.tempPath = FilePath.cacheDirectory.relativePath
    opts.logMaxLines = 3000
    try LibboxSetup(opts)
    
    // 4. (可选) checkConfig 预校验
    try LibboxCheckConfig(config)
    
    // 5. 启动 CommandServer
    commandServer = LibboxNewCommandServer(self, platformInterface)
    try commandServer?.start()
    
    // 6. 加载 config, 触发 openTun 回调
    let override = LibboxOverrideOptions()
    try commandServer?.startOrReloadService(config, override)
}
```

### 5.2 PlatformInterface.openTun 实现

参照 `~/References/hiddify-app/ios/HiddifyPacketTunnel/SingBox/ExtensionPlatformInterface.swift:22-200`

关键点:
- 从 `LibboxTunOptions` 抽 `getMTU/getInet4Address/getInet6Address/getDNSServerAddress/getInet4RouteAddress/...`
- 组装 `NEPacketTunnelNetworkSettings` (tunnelRemoteAddress 固定 "::1"):
  - `NEIPv4Settings(addresses:, subnetMasks:)` + `NEIPv4Route(destinationAddress:, subnetMask:)`
  - `NEIPv6Settings(addresses:, networkPrefixLengths:)` + `NEIPv6Route(destinationAddress:, networkPrefixLength:)`
  - `NEDNSSettings(servers:)` + matchDomains=[""]
- `setTunnelNetworkSettings(settings)` 是 **async** 回调, 必须用 `runBlocking` 包装回到 libbox 期望的同步语义 —— Hiddify 有现成 `Extension+RunBlocking.swift` 可以抄
- `packetFlow.fileDescriptor` 是系统给的 TUN fd (**这是 NE 和 Android VpnService.Builder.establish() 的语义差异 —— Android 你建 TUN 返回 fd, NE 你描述网络设置后 `packetFlow` 自动给你 fd**)
- 返回值: `ret0_.pointee = Int32(tunFd)`

### 5.3 autoDetectInterfaceControl / protect 语义

iOS 的 `NEPacketTunnelProvider` **没有 Android protect(fd) 对应概念** —— NE 框架自己处理 outbound socket 的路由避免回环, 你不用管。所以 `autoDetectInterfaceControl` 在 iOS 上是 no-op. 这里和 Android 不一样,不要照搬 Kotlin 的 `VpnService.protect(fd)` 实现。

### 5.4 stopTunnel / handleAppMessage

- `stopTunnel(with reason)`: `commandServer?.close()`; `platformInterface?.reset()`; 释放 TUN
- `handleAppMessage(_ data)`: 解析主 App 发来的命令,支持 `"stats"` / `"reload"` / `"switch_node:xxx"` 等,响应以 Data 返回

---

## 6. VPNManager 在主 App 的做法

参照 `~/References/hiddify-app/ios/Runner/VPN/VPNManager.swift`

核心 API:
```swift
class VPNManager: ObservableObject {
    @Published var state: NEVPNStatus = .invalid
    
    func setup() async throws {
        let managers = try await NETunnelProviderManager.loadAllFromPreferences()
        manager = managers.first ?? {
            let m = NETunnelProviderManager()
            let proto = NETunnelProviderProtocol()
            proto.providerBundleIdentifier = "com.foxnetpilot.NetPilot.NetPilotPacketTunnel"
            proto.serverAddress = "localhost"
            m.protocolConfiguration = proto
            m.localizedDescription = "NetPilot"
            return m
        }()
    }
    
    func connect(with config: String) async throws {
        manager.isEnabled = true
        try await manager.saveToPreferences()
        try await manager.loadFromPreferences()
        try manager.connection.startVPNTunnel(options: [
            "Config": config as NSString,
        ])
    }
    
    func disconnect() { manager.connection.stopVPNTunnel() }
}
```

第一次 `startVPNTunnel` 会弹系统 VPN 权限对话框 "允许 NetPilot 添加 VPN 配置"。用户同意后永久授权,下次不再弹。

与 Android `VpnController` 的差异:
- Android: `VpnService.prepare()` 返回 Intent,Activity 用 ActivityResultLauncher 接结果
- iOS: NETunnelProviderManager.saveToPreferences() 自动触发对话框,不需要显式处理回调

---

## 7. Entitlements / App Group / Bundle ID 规划

### 7.1 Bundle ID 约定 (抄 Hiddify)

- 主 App: `com.foxnetpilot.NetPilot` (已有, Phase 3A)
- Extension: `com.foxnetpilot.NetPilot.NetPilotPacketTunnel`
- App Group: `group.com.foxnetpilot.NetPilot`

### 7.2 主 App entitlements (`ios/NetPilot/NetPilot.entitlements`)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
    <key>com.apple.developer.networking.networkextension</key>
    <array>
        <string>packet-tunnel-provider</string>
    </array>
    <key>com.apple.developer.networking.vpn.api</key>
    <array>
        <string>allow-vpn</string>
    </array>
    <key>com.apple.security.application-groups</key>
    <array>
        <string>group.com.foxnetpilot.NetPilot</string>
    </array>
</dict></plist>
```

### 7.3 Extension entitlements

```xml
<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
    <key>com.apple.developer.networking.networkextension</key>
    <array>
        <string>packet-tunnel-provider</string>
    </array>
    <key>com.apple.security.application-groups</key>
    <array>
        <string>group.com.foxnetpilot.NetPilot</string>
    </array>
    <key>com.apple.security.network.client</key>
    <true/>
    <key>com.apple.security.network.server</key>
    <true/>
</dict></plist>
```

---

## 8. 构建脚本方案 (scripts/build-xcframework.sh)

**MVP 阶段不需要自己编**,直接复用 `~/References/ios-frameworks/Libbox_std.xcframework`。

**3B-5 升级阶段**, 脚本大致如下 (对应方案 B):

```bash
#!/bin/bash
set -euo pipefail
export GOPROXY="https://goproxy.cn,https://goproxy.io,direct"
export GOSUMDB=off
export GOTOOLCHAIN=local
export GOROOT=/opt/homebrew/opt/go@1.25/libexec
export PATH="$GOROOT/bin:$PATH:$(go env GOPATH)/bin"

# gomobile bind iOS (需要 macOS 主机 + Xcode command line tools)
gomobile bind \
    -target=ios,iossimulator \
    -trimpath \
    -ldflags='-s -w -checklinkname=0' \
    -tags='with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api' \
    -o ios/Frameworks/NetPilotCore.xcframework \
    ./libcore ./mobile \
    github.com/sagernet/sing-box/experimental/libbox
```

预期坑:
- iOS 的 gomobile bind 需要 Xcode command line tools
- `-target=ios,iossimulator` 会产生 fat xcframework, 真机 arm64 + 模拟器 arm64+x86_64
- `-checklinkname=0` 这个逃生舱 iOS 侧也需要 (同 #M11)
- **预计首次会撞 Cgo/Objc 互操作的坑**, 需要 2-4h 排查时间 (Hiddify 有现成脚本可抄)

---

## 9. 步骤清单 (用户介入后按天拆)

### Day 1 (user 参与, Xcode + 真机在手)
- [ ] 在 Apple Developer 后台创建 App Group `group.com.foxnetpilot.NetPilot` (需付费账号,若无需找替代方案)
- [ ] 在 Apple Developer 创建两个 App ID: `com.foxnetpilot.NetPilot` 和 `com.foxnetpilot.NetPilot.NetPilotPacketTunnel`, 给两者勾选 `Network Extensions` capability + 关联 App Group
- [ ] 创建 provisioning profile (development) 覆盖两个 bundle id
- [ ] 在 Xcode 中 `xcodebuild -exportArchive` 流程配置好 (或用 automatic signing)

### Day 2 (claude 无人值守可做, 用户审阅)
- [ ] 把 `~/References/ios-frameworks/Libbox_std.xcframework` 复制到 `ios/Frameworks/` 或加 .gitignore symlink
- [ ] 写 `ios/NetPilotPacketTunnel/` 目录下全部 Swift 源码 (PacketTunnelProvider / NetPilotPlatformInterface / ConfigMerger) — **约 500-800 行**
- [ ] 写 `ios/NetPilot/VPN/VPNManager.swift` + VPNConfig.swift + AppGroupFile.swift (**约 300 行**)
- [ ] 更新 `ios/project.yml` 新增 target + entitlements + App Group
- [ ] 跑 `xcodegen generate` 产出更新后的 xcodeproj

### Day 3 (user 真机测试)
- [ ] Xcode 打开 NetPilot.xcodeproj, 选 iPhone 物理设备
- [ ] Build & Run 主 App
- [ ] 点"开启 VPN" → 系统弹权限对话框 → 允许
- [ ] 验证真走流量 (浏览器访问境外站点)
- [ ] 发现问题修 PlatformInterface 细节 (routes/dns 映射等)

### Day 4-5 (user 参与, 稳定化 = 3B-5 iOS 部分)
- [ ] 网络切换 (Wi-Fi ↔ 蜂窝) 稳定性测试
- [ ] 后台杀进程恢复测试 (on-demand rules)
- [ ] 多订阅切换测试
- [ ] DNS 泄漏验证 (dnsleaktest.com)

---

## 10. 需要用户介入的清单 (无法无人值守的环节)

| 环节 | 为什么必须用户 | 替代方案 |
|---|---|---|
| Apple Developer 账号付费 ($99/年) | App Group 和 Network Extension 都是**付费账号才有**的 capability。免费 provisioning profile 无法启用 `com.apple.developer.networking.networkextension` | 无 (iOS 限制) |
| App ID 创建 | developer.apple.com 网页操作,需要手工点击 | 无 |
| Provisioning Profile | 需手机/Mac 绑定 Developer 账号, 下载 mobileprovision | Xcode Automatic Signing (仍需账号登录) |
| 真机 USB 连接 + Xcode Trust | 首次连接需在 iPhone 上点"信任此电脑" | 无 |
| 真机 VPN 权限对话框 | 首次 `saveToPreferences()` 系统弹框, 需用户点同意 | 无 |
| TestFlight 或 Ad Hoc 分发 (后期) | 非 user 账号无法分发 | 无 |

---

## 11. 已知风险

**R1 — xcframework 体积使 App Store 拒审**  
`Libbox_std.xcframework` 88MB, 加 NetPilot 主 App 估 120-150MB APK 。App Store 的 `Over-the-air` cellular 下载限制 200MB (已放宽到 App Store 上限但 WWAN 下载仍受限)。真机 development 不受影响,上架前需测包大小。

**R2 — `Libbox_std.xcframework` 版本不可控**  
`~/References/ios-frameworks/` 这份是什么时候编的?什么 sing-box 版本?未知。跟 Android aar 里的 sing-box v1.13.8 可能差版本,导致行为不一致。3B-5 自编阶段必须解决。

**R3 — packet-flow vs fd 语义**  
Android 直接给 fd, iOS 给 `packetFlow: NEPacketTunnelFlow` 对象。sing-box libbox 的 `OpenTun` 期望返回 Int32 fd —— Hiddify ExtensionPlatformInterface 里的做法是 **不返回 fd, 让 libbox 内部从 packetFlow 抓包**(?)。这个细节需要 3B-4 实装时再次验证,可能需要读 sing-box iOS 侧 tun 实现。

**R4 — 主 App 与 Extension 的 config 同步**  
Extension 每次 startTunnel 只收一次 config,之后用户在主 App 改节点 (overlay.Apply),merged.json 更新,但 Extension 不会自动重载。需要:
- (a) 主 App 通过 `sendProviderMessage("reload")` 让 Extension 重读 merged.json
- (b) 或 Extension 定时 poll merged.json 的 mtime

**R5 — iOS 16+ 的 "Always-on VPN" 与 `NEOnDemandRule`**  
Hiddify 用了 on-demand rules 让 VPN 在所有网络下自动连接。这个对 UX 重要但也是常见踩坑点 —— 用户关了 Wi-Fi 又开,Extension 会被拉起多次。3B-5 阶段考虑。

---

## 12. 与 Android 的差异清单 (方便开发时心智对照)

| 主题 | Android (3B-3 已完成) | iOS (3B-4) |
|---|---|---|
| VPN 实体 | `VpnService` 前台服务 | `NEPacketTunnelProvider` Extension appex |
| 权限请求 | `VpnService.prepare()` + Intent | `NETunnelProviderManager.saveToPreferences()` 系统弹框 |
| TUN fd | `VpnService.Builder.establish().fd` | `packetFlow.fileDescriptor` 或 不返回 fd (待验证) |
| outbound protect | `VpnService.protect(fd)` | 无,NE 框架代劳 |
| 配置目录 | `filesDir/configs/` | App Group container `Library/configs/` |
| 前台通知 | 是 (`startForeground`) | 否 (Extension 进程无 UI) |
| 共享内存 | 单进程, 直接 Kotlin 引用 | 双进程, 走 App Group 文件 |
| Libbox.setup | `NetPilotApp.onCreate()` | `PacketTunnelProvider.startTunnel()` |
| CommandServer 生命周期 | 挂在 VpnService | 挂在 Extension |
| ConfigMerger | `ConfigMerger.kt` (JSONObject) | `ConfigMerger.swift` (JSONSerialization) |
| 无 tun inbound 兜底 | `android_tun_base.json` → 合并到 merged | `ios_tun_base.json` → 同上, 复用逻辑 |

---

## 13. 开发顺序建议 (给接手的 claude 或人)

1. **先读** 本文 + `~/References/hiddify-app/ios/HiddifyPacketTunnel/` 全部 Swift
2. **然后读** Hiddify 主 App 的 `Runner/VPN/VPNManager.swift`
3. **接着** 复制 xcframework → 改 project.yml → xcodegen generate,确认 Xcode 能打开工程
4. **再** 写 Extension 代码 (PacketTunnelProvider + PlatformInterface),先让 Extension 能编译
5. **之后** 写主 App VPNManager,先让主 App 能调用 startVPNTunnel
6. **最后** 真机跑通 → 调路由映射细节

务必先让"能编译 + 能启动"跑通,再追求 DNS/route 完全对齐。MVP 就是能连上代理访问到墙外,其他都是稳定性。

---

## 14. 3B-3 Debrief — 直接应用到 iOS 的经验教训 (2026-04-22 补)

3B-3 在 Pixel 4a 上从"UI 已连接但 ping 超时"到真走流量,踩了 5 个坑。写给未来实现 3B-4 的人:

### 14.1 sing-box 1.13 schema 必须先迁移 (和 Android 完全同款)

Android 侧是今天发现的硬伤。任何 1.11- 的配置语法直接 `Libbox.checkConfig` 失败:

- 删: `outbounds[type="dns"]`、`outbounds[type="block"]`
- 改: 用 `route.rules` 里 `{action:"hijack-dns"}` / `{action:"reject"}` / `{action:"sniff"}`
- 注: DNS server `detour` 不能指向没特殊配置的 direct outbound (报错 "detour to an empty direct outbound makes no sense")
- `route.default_domain_resolver` 在 1.12+ 必需

**iOS 影响**: `ios/NetPilotPacketTunnel/ios_tun_base.json` (或放 App Group container 的 fallback 配置) 必须按 1.13 schema 写。直接抄 `android/app/src/main/assets/android_tun_base.json` 即可,两边 TUN inbound 形态一样。

### 14.2 ConfigMerger 逻辑几乎可以 1:1 抄 Kotlin 版

Android 走通的合并策略:
- inbounds: merged 里没 `type=tun` → 从 base 拷一份
- route: 若无 `auto_detect_interface` / `default_domain_resolver` → 补;系统级 rules (`sniff` / `hijack-dns` / `ip_is_private direct`) 必须 prepend 到 merged.rules 前 (用户规则如 "抖音 direct" 会截胡 DNS 流量)
- dns: 若无 dns 字段 → 整段拷 base

iOS Swift 版用 `JSONSerialization` + `Dictionary<String,Any>` 基本 1:1 对应 `JSONObject`。**不要自造逻辑**,抄现成的。

### 14.3 iOS 侧 `getInterfaces` / `startDefaultInterfaceMonitor` 不用真实现 (!)

这是今天最大的意外收获。Android 这两个 stub 掉是灾难 (#H1 loopback, 花了半天查), 但 **iOS 不需要** —— `underNetworkExtension() = true` 告诉 sing-box "我在 NE 沙箱里,数据面你用 NE 的代码路径", 它就不会 fallback 去枚举物理网卡。

证据: `~/References/hiddify-app/ios/HiddifyPacketTunnel/SingBox/ExtensionPlatformInterface.swift:257` 和 `:292` 两个方法都在顶部直接 `return` / `throw`, 而真实现都 **注释掉或 unreachable**。Hiddify 跑得好好的。

**这意味着 Swift 版 NetPilotPlatformInterface 大约只有 Kotlin 版的 60% 代码量**, 原来估的 500-800 行应修正到 300-500 行。

但留个口子: 若真机测出 "sing-box 上游连接回环" 症状, 回来看 `NWPathMonitor` 的真实现 (Hiddify 有注释掉的代码可启用)。

### 14.4 VPN-as-default-network loopback bug 在 iOS 上不存在

Android P+ `registerDefaultNetworkCallback` 把 VPN 自己当默认网络返回 → sing-box 出口指向 tun0 → 死循环。这是 Android 平台特定 bug, NE 框架自己保证不回环 (sing-box 通过 NE 的 packet flow 直接读包, 不走 OS 路由表选出口)。

**iOS 影响**: 这个坑不用防, 但要记住为什么不用防 —— 避免将来有人"好心"地在 iOS 侧加一个 `NWPathMonitor` 真实现却观察到 VPN 接口被误选。

### 14.5 Extension 进程的 startTunnel 幂等保护必须有 (和 Android 同款坑)

Android 侧发现: 用户双击"开启"、或 Android 重复投递 intent, 第二次 `startService()` 会试图启第二个 libbox, 撞 `bind 127.0.0.1:9090 address already in use`, catch 路径 stopService() 把正常 instance 一起带走, 表现为"点了没反应"。

**iOS 对应场景**: iOS 重连后台 / 网络切换 / on-demand rules 触发, 都可能让系统把已运行的 Extension 再次 `startTunnel(options:)` 一次。`PacketTunnelProvider` 内部需要:

```swift
private var isRunning = false

override func startTunnel(options: ...) async throws {
    if isRunning {
        NSLog("startTunnel re-entered, ignoring")
        return
    }
    isRunning = true
    defer { if !success { isRunning = false } }
    // ... real startup
}
```

对应 `reset()` 路径要把 `isRunning = false` 写回去。

### 14.6 validation 路径: 一定用 `/connections` 而不是 `/delay`

3B-3 一开始只看 `/proxies/香港-1/delay` 成功 (2919ms), 差点以为 DoD 达成。但 delay 测试是在 sing-box 进程内部直接走 outbound 栈, **不经过 TUN 入口**, 证明不了其他 app 的流量真的被路由了。

真正证据是 `/connections` 里看到 `api.ipify.org:443 via ['香港-1', 'proxy-group']` 且 metadata 里的 process 不是 NetPilot 自己。

**iOS 影响**: 3B-4 验证时第一时间 `curl http://127.0.0.1:19090/connections` (记得 adb forward 在 iOS 上换成其他机制, 或进 App 看内置面板), 确认有外部 UID 的连接经 proxy-group。仅看 delay 通过会漏掉"其他 app 流量走 Bypass VPN" 这种静默错误。

### 14.7 文件路径契约

Android 两边达成的约定:
- Go overlay 写: `filesDir/merged.json` (不带 configs/ 前缀, 来自 `overlay.MergedConfigPath()`)
- Kotlin VpnService 读: `filesDir/merged.json`, fallback `filesDir/configs/android_tun_base.json`
- App bootstrap 把 asset 拷到 `filesDir/configs/{android_tun_base.json, minimal.json}` (后者是 Go overlay base 的硬要求)

**iOS 对应**:
- Go overlay 写: `$GROUP_CONTAINER/Library/merged.json`
- Swift PacketTunnelProvider 读: 同上, fallback `$GROUP_CONTAINER/Library/configs/ios_tun_base.json`
- 主 App 启动时把 Bundle 里的 `ios_tun_base.json` 拷到 container 并同时写 `configs/minimal.json` (给 Go overlay 吃)

注意 iOS App Group container 的具体子目录由 `FileManager.default.containerURL(forSecurityApplicationGroupIdentifier:)` 返回, 里面常用 `Library/` 做工作区, `Library/Caches/` 做临时。

### 14.8 调试工具链 (Android 的经验可复用)

Android 侧一路用:
- `adb logcat -v brief > file` 流式
- `adb shell run-as <pkg> sh -c '...'` 读 Extension 私有目录
- `adb forward tcp:19090 tcp:9090` 把设备 Clash API 映射到主机
- `adb shell am start -a android.intent.action.VIEW -d <url>` 触发外部 UID 流量

**iOS 对应**:
- `Console.app` + 选 iPhone 设备 + 过滤 `subsystem:com.foxnetpilot.NetPilot` 看日志流
- `xcrun devicectl device list` + `devicectl` 系列命令 (新式替代 iOS 16+)
- Clash API 映射: 无直接等价, 需要在主 App 内做一个调试面板 HTTP 打 Extension 内的 127.0.0.1:9090, 前提是 Extension 绑定在 container 内 (不是 loopback)。或改 Clash API 绑到 Unix domain socket 放 App Group container, 让主 App 通过 AppGroupFile.swift 读响应
- 触发外部流量: 在 iOS 上打开 Safari 访问网站即可,但"看连接列表"不如 Android 方便

### 14.9 Android UI 残留 bug (#M13) 对 iOS 的启示

Android 侧发现 `NetPilotCore.markTunRunning(true)` 被调用了但 Compose UI 没更新, 推测是 state flow 观测链路断。

**iOS 影响**: Swift 侧用 `@Published var state: NEVPNStatus` + `NotificationCenter.default.addObserver(forName: .NEVPNStatusDidChange)` 来同步,这是系统标准机制,基本不会踩坑。反而比 Android 稳。但仍要注意: Extension 的状态变化通过 `manager.connection.status` 获取, 不要自己维护一份。

### 14.10 3B-4 前应先修 #M13 么?

不修。理由:
- 3B-4 换了 Swift 侧, Android Compose 的 bug 不会带过去
- 3B-4 和 #M13 是正交的 work
- #M13 只影响视觉, 流量是通的

但 3B-5 双端联调阶段一定要修 (UX 不能接受用户以为没连上反复点)。
