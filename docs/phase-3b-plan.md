# Phase 3B-1 执行计划 · 抄作业版

> 状态: **预研已完成, 4 大阻塞点全部拆除, 代码实现待启动**
> 主要抄作业对象: **NekoBoxForAndroid / libcore** (`~/References/nekobox/libcore/`)
> 最后更新: 2026-04-17 (加入 4 合 1 预研结果)

---

## 1. 执行摘要

Phase 3B-1 目标: 把 sing-box libbox 以源码形式引入 NetPilot Go 模块, 用 gomobile bind 产出**真能跑数据面的 `netpilot.aar`**, 替代当前控制面-only 的 22.5 MB 占位 aar。

核心判断:
- **照搬 NekoBox libcore 的 gomobile 绑定结构**, 不自己发明 API 形状
- 在 NetPilot 仓库新建独立的 `libcore/` package (类似 NekoBox), 让 `mobile/` 继续做业务编排 (Chat/Overlay/Subscription), 让 `libcore/` 专门做 sing-box 嵌入 + TUN 数据面
- sing-box 内核直接跟 **SagerNet 官方 v1.13.8** (最新 stable), 不跟 MatsuriDayo fork — 理由见第 4 节
- gomobile 使用**官方 `golang.org/x/mobile`**, 不用 `gomobile-matsuri` 分支
- Go 版本从当前 1.26.1 **降到 1.23.x**, 对齐 libcore 的验证版本

3B-1 交付物: `android/app/libs/netpilot.aar` 包含完整 libbox, 通过 `LibboxTest` 冒烟, Kotlin 侧能 `import libcore.BoxInstance` 且编译通过。**不做真实数据面接入** — 那是 3B-3 的事。

---

## 2. 参考资源索引表 (已修正 libneko 误解)

| 路径 / URL | 用途 | 优先级 |
|---|---|---|
| `~/References/nekobox/libcore/` | **首要抄作业对象**: gomobile 绑定层、sing-box 嵌入封装、PlatformInterface 实现 | P0 |
| `~/References/nekobox/app/src/main/java/io/nekohasekai/sagernet/bg/VpnService.kt` | Android VpnService 完整实现 (218 行) | P0 |
| `~/References/nekobox/app/src/main/java/io/nekohasekai/sagernet/bg/proto/BoxInstance.kt` | Kotlin 侧 BoxInstance 包装 (222 行) | P0 |
| `~/References/nekobox/app/src/main/java/moe/matsuri/nb4a/NativeInterface.kt` | 实现 BoxPlatformInterface + NB4AInterface 的桥接类 | P0 |
| `~/References/nekobox/.github/workflows/{preview,release}.yml` | 权威 CI 构建脚本, `./run lib core` 入口 | P0 |
| `~/References/nekobox/buildScript/init/env_ndk.sh` | NDK 版本 (25.0.8775105) | P1 |
| `~/References/libneko/protect_server/` | **可能复用**: socket 保护 AF_UNIX server (VpnService.protect 的 Go 侧) | P1 |
| `~/References/libneko/neko_log/`, `speedtest/`, `syscallw/` | 小工具, 按需摘取 | P2 |
| https://deepwiki.com/MatsuriDayo/NekoBoxForAndroid/ | NekoBox 逆向结构化文档, 架构速查 | P1 |
| `~/References/hiddify-app/ios/HiddifyPacketTunnel/` | iOS 参考, 留给 **3B-4**, 本期不深挖 | P2 |
| https://github.com/SagerNet/sing-box/tree/main/experimental/libbox | 官方 libbox 源码 | P0 |
| `~/References/test-gomobile-ios.sh` | 之前做过的 iOS xcframework 编译脚本 | P1 |

---

## 3. 当事人脚本分析

### `test-singbox-api.sh`
纯 **Phase 1** 遗产 — Clash API 端到端冒烟测试, 启动外部 `sing-box` 二进制 + curl 各 HTTP 端点。**与 Phase 3B 无关**, 可忽略。历史上 pass 过 (见 `test-singbox-api.sh:305-313` 硬编码的 "全部 ✅" 结果), 说明 Clash API 接口签名和我们当下用的 `SingBoxAdapter` 对得上。

### `test-gomobile-ios.sh`
**与 3B-4 直接相关, 与 3B-1 间接相关**。关键信息:
- 脚本在 `/tmp/netpilot-ios-test` 里 `git clone --depth 1 https://github.com/SagerNet/sing-box.git`, 然后对 `./experimental/libbox/` 跑 `gomobile bind -target ios` (脚本 L108, L152)
- 三套 tags 尝试顺序: `TAGS_STANDARD="with_quic,with_gvisor,with_utls"` → 无 tags → `TAGS_FULL="with_quic,with_gvisor,with_utls,with_wireguard,with_clash_api"` (L144-146)
- Go 版本要求 `>= 1.22` (L44-49)
- 产物路径 `/tmp/netpilot-ios-test/Libbox.xcframework` 已被脚本的 trap 或 /tmp 清空机制清理掉 — **无法查看当时产物**
- 结论: 脚本"尝试过"但未沉淀代码, 对 3B-1 Android 方向的价值是 **tag 组合参考** (跟 NekoBox libcore 的 `with_conntrack,with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api` 对比, 差一个 `with_conntrack`)

证据: `~/References/test-gomobile-ios.sh:108,144-146,152,162,172`

---

## 4. sing-box 版本决策

| 选项 | 版本/来源 | 利 | 弊 |
|---|---|---|---|
| A | MatsuriDayo/sing-box fork (NekoBox 用) | 跟 NekoBox 完全一致, 所有 patch 都自动带上 | NekoBox `libcore/go.mod:13` 声明为 `sing-box v1.0.0 // replaced` + `replace => ../../sing-box` (L87), 需要我们也维护一个 `~/Netpilot/sing-box/` 本地 clone, 后续升级靠手动 rebase |
| B | **SagerNet/sing-box v1.13.8** (官方最新 stable, 2026-04-14 发布) | go.mod 用 tag 版本号, 无本地 replace, 升级走标准 `go get`; 干净 | 没有 NekoBox 的 patch, 可能碰到 NekoBox 已修的 iOS/Android 兼容问题 |
| C | 官方 sing-box v1.13.8 + 自己写 libcore 封装 | 混合灵活 | 自己写的 libcore 意味着要维护 PlatformInterface 的桥接层, 工作量 = B + 5 天 |

**决策**: **选 B** — **已验证**。

理由 (原始判断):
1. NekoBox libcore 用本地 replace 是为了让他们 patch sing-box 方便, 这套工作流强耦合到 `buildScript/init/get_source.sh` 一整套, 我们抄过来就等于再维护一个下游 fork, 违背"参考优先"而不是"镜像运行"
2. NekoBox 的依赖图 (`libcore/go.mod:11-14` `sagernet/sing v0.7.18` + `sagernet/sing-tun v0.7.10` + `quic-go v0.52.0-sing-box-mod.3`) 完全匹配 sing-box v1.12-v1.13 上游, 说明他们的 patch 其实很薄
3. 我们的 Agent 层 (`internal/agent/`) 会大量 diverge 原生 sing-box, 选官方版可以让 sing-box 层保持"纯粹 ForwardPort"的便宜升级节奏
4. sing-box 官方 v1.13.8 已经稳定发布, 风险可控 (证据: https://github.com/SagerNet/sing-box/releases/latest)

**预研实证** (2026-04-17 新增): 在 `/tmp/netpilot-3b-preflight/gotest/` 空白 go 模块里直接 `go get github.com/sagernet/sing-box@v1.13.8`, **成功拉取, 无任何 replace 需要**。go.sum 105 行, 依赖图清爽。关键 transitive 版本:
- `github.com/sagernet/sing v0.8.4` (比 NekoBox 的 v0.7.18 新一版)
- `github.com/sagernet/sing-tun v0.8.7` (比 NekoBox v0.7.10 新)
- `github.com/sagernet/gvisor v0.0.0-20250811.0-sing-box-mod.1`

**备用方案**: 如果编译遇到 NekoBox 已解决但官方未 merge 的 issue, 临时改用 A, 在 CLAUDE.md 标记"已降级到 NekoBox fork", 并追踪升级时机。

---

## 5. 目标构建命令

### 5.1 NekoBox 权威命令 (原版, 仅作参考)

摘自 `~/References/nekobox/libcore/build.sh:18`:
```bash
gomobile-matsuri bind \
  -v \
  -androidapi 21 \
  -cache "$(realpath .build)" \
  -trimpath \
  -ldflags='-s -w' \
  -tags='with_conntrack,with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api' \
  .
```

前置环境变量: `ANDROID_NDK_HOME` (`env_ndk.sh:23`), `GOPATH` (`build.sh:13-15`), `GOBIND=gobind-matsuri` (`build.sh:17`)。

CI 入口: `./run lib core` (`.github/workflows/preview.yml:30`, `release.yml:39`) — 底层调用 `libcore/build.sh`。

### 5.2 NetPilot 目标命令 (3B-1 终版, 已实证)

```bash
# 前置环境 (见第 11 节详版)
export GOTOOLCHAIN=local
export GOROOT=/opt/homebrew/opt/go@1.25/libexec
export PATH="$GOROOT/bin:$JAVA_HOME/bin:$PATH:$(go env GOPATH)/bin"
export GOPROXY="https://goproxy.cn,https://goproxy.io,direct"
export GOSUMDB=off

gomobile bind \
  -target=android \
  -androidapi 21 \
  -trimpath \
  -ldflags='-s -w -checklinkname=0' \
  -tags='with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api' \
  -o android/app/libs/netpilot.aar \
  ./libcore ./mobile \
  github.com/sagernet/sing-box/experimental/libbox
```

**为什么 bind 三个包而不是两个** (2026-04-17 第三轮实证发现):
- 若只 bind `./libcore ./mobile`, aar 里只有我们自定义的 `libcore.*` / `mobile.*` 类, **Kotlin 侧看不到 `libbox.CommandServer`/`libbox.PlatformInterface` 等类**
- sing-box-for-android (sfa) 官方客户端直接使用 `libbox.Libbox.setup(...)` + `libbox.CommandServer(handler, pi)` + `server.startOrReloadService(config, opts)` 的 API(见 docs/phase-3b-plan.md §5 补注)
- 把 `github.com/sagernet/sing-box/experimental/libbox` 加为 bind 的第 3 个 package,aar 体积 40MB → 57MB, 但 classes.jar 新增 109 个 `libbox.*` Java 类,Kotlin 侧可直接照抄 sfa 代码
- **libcore 职能退化**: 原计划 libcore.BoxInstance 包 libbox.CommandServer, 现在 Kotlin 可直接操作 libbox,libcore 降级为 convenience 层 (3B-3 会进一步精简)

每个 flag 的来源说明:

| flag | 值 | 来源 / 验证 |
|---|---|---|
| `-target=android` | 四架构 (arm64/armv7/amd64/386) | 2026-04-17 预研产出 40MB aar |
| `-androidapi` | 21 | `libcore/build.sh:18` (允许低到 Android 5.0) |
| `-trimpath` | 开 | `build.sh:18` |
| `-ldflags='-s -w -checklinkname=0'` | 压缩 + **绕过 linkname 检查** | `-s -w` 来自 `build.sh:18`; `-checklinkname=0` 是我们第二轮预研发现 Go 1.23+ 新增的必要逃生舱 |
| `-tags=with_gvisor,...` | 5 个 | `build.sh:18` 减去 `with_conntrack` (2026-04-17 预研不加也通过) |
| `-o` | `android/app/libs/netpilot.aar` | NekoBox 是 `libcore.aar` (`build.sh:23`), 我们沿用现有命名 |
| 包路径 | `./libcore ./mobile` | **策略 B**: 两个包一起 bind ✅ 已实证 |

**验证数据点** (沙箱 `/tmp/netpilot-3b-preflight/multipkg/`, 2026-04-17):
- 单架构 arm64-v8a: 9.4 MB aar, `libgojni.so` 27.9 MB
- 全四架构 (arm64+armv7+amd64+386): **40 MB aar**, 构建 wall time 67.8 秒 (CPU 4.65x 并行)
- classes.jar 内含 `libcore.BoxInstance`、`libcore.Libcore`、`mobile.Client`、`mobile.Mobile` 四个 Java 类
- Play Store 50MB 限: **✅ 单架构 APK 10-12MB 远低, 双 ARM 架构 APK ~25-30MB 依然通过**

**关于 `with_conntrack`**: NekoBox 的 `conntrack` 用于 Linux `nf_conntrack` 连接追踪, 移动端用不上。预研不加也成功编过, **确认丢弃**。

**关于 `gomobile-matsuri`**: 原担忧 "官方 gomobile 多包不可行" 已证伪, **用官方版**。

---

## 6. mobile/ 改造路径

### 6.1 策略选型

| 策略 | 描述 | 判断 |
|---|---|---|
| A | `mobile/netpilot.go` 直接 `import libbox` 并封装 | **否决**: `mobile/` 现在 474 行已经是 Chat/Overlay/Subscription 业务聚合, 再把 sing-box 内核塞进来会变成上帝包; 且 gomobile 对单文件 package 大规模 import 不友好 |
| B | **新建 `libcore/` 目录**, 承担 sing-box 嵌入 + PlatformInterface + TUN 数据面; `mobile/` 继续做业务编排, 只依赖 `libcore` 抽象 | **采纳** |
| C | 混合 | 过度设计 |

**证据 (NekoBox 是策略 B)**: NekoBox 把 gomobile 入口完全放在 `~/References/nekobox/libcore/` 独立 module (`libcore/go.mod:1` `module libcore`), 而 Kotlin 业务代码在 `~/References/nekobox/app/`。两套代码通过 **Kotlin 实现 Go interface** 的反向绑定解耦 (`platform_java.go:3-22`)。

### 6.2 NetPilot 采纳的目录结构

```
Netpilot/
├── libcore/                # 新增 - sing-box 嵌入 + TUN 数据面
│   ├── go.mod              # module github.com/foxnetpilot/netpilot/libcore
│   ├── box.go              # BoxInstance (抄 nekobox libcore/box.go)
│   ├── box_include.go      # build-tag 汇总文件
│   ├── platform_box.go     # sing-box platform.Interface 的实现 wrapper
│   ├── platform_java.go    # 暴露给 Kotlin 的 Go interface
│   ├── assets.go           # geoip/geosite 资源解压
│   └── dns_android.go      # Android DNS 直通 (抄 libcore/dns_android.go)
├── mobile/                 # 保留 - 继续做业务层
│   └── netpilot.go         # 474 行, StartTun/StopTun 改为调用 libcore
└── go.mod                  # 主模块, 追加 sing-box v1.13.8 依赖
```

### 6.3 `mobile/netpilot.go` StartTun/StopTun/TunRunning 改造伪代码

**当前状态** (L197-225): 纯占位, 仅记录 `tunFd` 和 `tunRunning` 字段。

**⚠️ 2026-04-17 预研结论 — 原伪代码已失效, 必须走 OpenTun 回调**:

读官方 `experimental/libbox/service.go` 的 `platformInterfaceWrapper.OpenInterface` 实现:
```go
tunFd, err := w.iif.OpenTun(&tunOptions{options, routeRanges, platformOptions})
// ...
dupFd, err := dup(int(tunFd))
options.FileDescriptor = dupFd
```
sing-box libbox **没有接受预开 fd 的构造路径**, 所有 tun inbound 必须通过 `PlatformInterface.OpenTun()` 反向回调取 fd。这意味着:
- `StartTun` 签名必须丢掉 fd 参数, 改为 `StartTun(configJSON string) error`
- Kotlin 侧要实现 `libcore.PlatformInterface`, 其 `openTun()` 方法里调 `VpnService.Builder.establish()` 返回 fd
- `libcore.NewBoxInstance` 要接收 PlatformInterface 实例

**目标状态**:

```go
// libcore/platform.go  (伪代码)

// PlatformInterface 会被 gomobile bind 成 Java/Kotlin 接口
type PlatformInterface interface {
    OpenTun(tunOptionsJSON string) (int32, error)  // Kotlin 实现, 里面 establish TUN 返回 fd
    AutoDetectInterfaceControl(fd int32) error      // 对出站 socket 调 VpnService.protect
    UseProcFS() bool                                // Android 5+ 限制, 返回 false
    FindConnectionOwner(...) (int32, error)         // 可返回 -1
    PackageNameByUid(uid int32) (string, error)     // 3B-1 返回 "", 3B-3 补
    UIDByPackageName(packageName string) (int32, error)
    WIFIState() string                              // 返回 "" 即可
}

// libcore/box.go  (伪代码)
type BoxInstance struct {
    ctx     context.Context
    cancel  context.CancelFunc
    service libbox.Service  // 来自 github.com/sagernet/sing-box/experimental/libbox
}

func NewBoxInstance(configJSON string, iface PlatformInterface) (*BoxInstance, error) {
    // 抄 nekobox/libcore/platform_box.go 的 boxPlatformInterfaceWrapper
    wrapper := newPlatformWrapper(iface)
    service, err := libbox.NewService(configJSON, wrapper)
    if err != nil { return nil, err }
    return &BoxInstance{service: service}, nil
}

func (b *BoxInstance) Start() error { return b.service.Start() }
func (b *BoxInstance) Close() error { return b.service.Close() }
```

```go
// mobile/netpilot.go  (伪代码)

type Client struct {
    // ...existing fields...
    box  *libcore.BoxInstance
    iface libcore.PlatformInterface  // Kotlin/Swift 侧传进来
}

// SetPlatformInterface 由 Android/iOS 侧在创建 Client 后立即调用
func (c *Client) SetPlatformInterface(iface libcore.PlatformInterface) {
    c.mu.Lock(); defer c.mu.Unlock()
    c.iface = iface
}

// StartTun 不再接收 fd, 内核会通过 PlatformInterface.OpenTun 回调拿 fd
func (c *Client) StartTun(configJSON string) error {
    c.mu.Lock(); defer c.mu.Unlock()
    if c.iface == nil { return fmt.Errorf("platform interface not set") }
    if configJSON == "" {
        configJSON = c.overlay.MergedConfigJSON()
    }
    box, err := libcore.NewBoxInstance(configJSON, c.iface)
    if err != nil { return err }
    if err := box.Start(); err != nil { box.Close(); return err }
    c.box = box
    return nil
}

func (c *Client) StopTun() {
    c.mu.Lock(); defer c.mu.Unlock()
    if c.box != nil { _ = c.box.Close(); c.box = nil }
}

func (c *Client) TunRunning() bool {
    c.mu.Lock(); defer c.mu.Unlock()
    return c.box != nil
}
```

**Kotlin 侧 3B-3 的 NativeInterface 形态** (抄 `~/References/nekobox/app/src/main/java/moe/matsuri/nb4a/NativeInterface.kt`):
```kotlin
class NetPilotNativeInterface(
    private val vpnService: NetPilotVpnService,
) : PlatformInterface {
    override fun openTun(tunOptionsJson: String): Long {
        // 此时才 establish TUN, Go 侧要到 Start() 被 Kotlin 调用时才触发回调
        val fd = vpnService.establishTun(tunOptionsJson)  // 新增方法, 把现有 startTun() 拆一半
        return fd.toLong()
    }
    override fun autoDetectInterfaceControl(fd: Int) { vpnService.protect(fd) }
    override fun useProcFS() = false
    // ...其他方法 3B-1 阶段返回默认值, 3B-3 再补
}
```

**3B-1 阶段降级实现**: 如果 3B-1 想先跑通编译链路, 可以让 `PlatformInterface` 在 Go 侧有个 `NopPlatformInterface` stub 实现, 所有方法返回合理 zero value, `StartTun` 不接 Kotlin 的 interface 传入 —— 这样 aar 能编出来但跑不起来, 3B-1 DoD 只要求编译通过和 symbol 正确, 不要求真跑, 符合要求。

---

## 7. Android VpnService 改造差距清单

对比 NetPilot `android/app/src/main/java/com/foxnetpilot/netpilot/vpn/NetPilotVpnService.kt` (115 行) vs NekoBox `app/src/main/java/io/nekohasekai/sagernet/bg/VpnService.kt` (218 行) + `NativeInterface.kt` + `BoxInstance.kt`:

| 差距 | 我们的状态 | NekoBox 状态 | 3B 阶段 |
|---|---|---|---|
| **TUN fd 已能传 Go** | ✅ L62-64 `tun.detachFd()` + `NetPilotCore.startTun(fd, "")` | ✅ VpnService.kt:197-200 `conn!!.fd` | 已达标 |
| **前台服务 foregroundServiceType** | ✅ `systemExempted` (AndroidManifest L29) | ✅ 同 (`AndroidManifest.xml:278`) | 已达标 |
| **Android 14+ startForeground(id, noti, type)** | ❌ 用旧两参数 API (L42) | ✅ `ServiceNotification.kt:187-206` 检测 SDK34+ 用三参数 | **3B-3 必须修** |
| **NativeInterface (BoxPlatformInterface 实现)** | ❌ 完全缺失 | ✅ `NativeInterface.kt` 实现 7 + 2 个方法 | **3B-2 必做** |
| **BoxInstance Kotlin 包装** | ❌ 直接裸调 `NetPilotCore.startTun` | ✅ `BoxInstance.kt` 222 行, 提供 Start/Close/QueryStats/SelectOutbound | **3B-3 必做** |
| **状态机 / 异常捕获** | ❌ L67 `catch (t: Throwable) { stopSelf() }` 一把梭 | ✅ `BaseService.kt:373-393` 三态 + Binder 广播 stateChanged | 3B-3 应做, 3B-5 可补 |
| **WakeLock** | ❌ 无 | ✅ VpnService 生命周期挂 WakeLock (`ServiceNotification.kt` 引用) | 3B-5 |
| **AIDL / Binder 给 UI 回调** | ❌ 无 | ✅ `SagerConnection` + AIDL (DeepWiki §4.1) | 3B-5 或留到 Phase 3C |
| **per-app 黑白名单** | 🟡 仅 `addDisallowedApplication(packageName)` (L55) | ✅ 完整 UI + `addAllowedApplication` (`VpnService.kt:174-177`) | 3B-5 |
| **onRevoke / onDestroy** | ✅ 已调 `stopTun()` (L78-86) | ✅ 同 | 已达标 |
| **IPv6 支持** | ✅ `fdfe:dcba:9876::1/126` (L48) | ✅ 同 | 已达标 |
| **WIFIState 回调** | ❌ 缺失 | ✅ NativeInterface 提供 WIFI SSID/BSSID | 3B-3 (for 基于 SSID 的自动规则) |

**3B-1 本期不涉及任何 Kotlin 改动**, 只需 aar 能被 gradle sync + 不报符号缺失就行。上表差距用作 **3B-2/3B-3 的路线图**。

---

## 8. libneko 二次评估

`~/References/libneko/` 是 MatsuriDayo 的**跨平台小工具集**, 非 sing-box 绑定层。子目录评估:

| 子目录 | 内容 | Phase 3B 会用? |
|---|---|---|
| `iphlpapi/` | Windows `dll_windows.go` 路由表查询 | ❌ 移动端无关 |
| `neko_common/` | `common.go` 1 文件, 通用常量 | 🟡 可能作为 `libcore` 的轻量 constants 参考 |
| `neko_log/` | `log.go` 统一日志写入 | 🟡 **有用**: libcore/nb4a.go:74-82 用它写 `neko.log`, 我们可抄一个极简版 |
| `protect_server/` | `protect_server_linux.go` + `protect_server_other.go` | 🔴 **关键**: Android 侧 sing-box 要对每个出站 socket 调 `VpnService.protect(fd)` 避免回环, 这个包实现 **Go 侧通过 AF_UNIX socket 把 fd 发给 Kotlin 让其 protect**, 是 libbox `AutoDetectInterfaceControl` 的底层机制。**3B-3 必读** |
| `speedtest/` | `speedtest.go` 节点测速 | ❌ 我们已有 Clash API `/proxies/{tag}/delay` |
| `syscallw/` | 跨平台 syscall wrappers (darwin/windows/other) | 🟡 可能用于 fd dup 等底层调用 |

**结论**: Phase 3B-3 会显式依赖 `protect_server/`, 其他子包暂不引入。这解决了之前"libneko 是什么"的误解 — 它是**基础设施小工具**, 不承载 sing-box 嵌入, 所以我们不要把它当 libcore 抄。

---

## 9. geoip/geosite 资源策略

### NekoBox 做法 (参考)
证据: `libcore/assets_android.go:16-108`
- 资源**编译进 APK** 作为 Android asset (不在 aar 里)
- 版本号走 `geoip.version.txt` / `geosite.version.txt`, 数值大则 `copy over` 到可写 assets 目录
- 双路径架构: `internalAssetsPath` (不可覆盖, 如 yacd)  + `externalAssetsPath` (geoip/geosite, 可被订阅或用户替换)
- `NB4AInterface.UseOfficialAssets() bool` 控制是否自动升级 (`platform_java.go:18`)

### NetPilot 3B-1 决策

**3B-1 不做资源管理, 留空 stub**。原因:
- sing-box v1.13 可用 rule-set 远程热更新, geoip.db 不是硬依赖
- 3B-1 焦点是"能编译 + 能 import", geoip 可以用"运行时缺文件 → 从 CDN 下载"的运行时方案, 不影响 aar 构建
- 3B-3/3B-5 真机跑时, 按需把 geoip-cn.srs 下载到 `dataDir` 即可

**3B-1 Definition**: `configs/minimal.json` 不引用任何外部 geoip 文件, 只有 `direct/block/selector` 三个基本出站。真实规则在 `_agent:` overlay 里。

---

## 10. buildScript 目录处理策略

NekoBox 的 `~/References/nekobox/buildScript/` 是一整套 Bash + Gradle 辅助脚本 (env_java / env_ndk / get_source / pack_release 等), 深度绑定他们的 CI。**不要全盘照搬**。

### 只摘取以下最小集

| 原文件 | 用途 | 我们的对应物 |
|---|---|---|
| `buildScript/init/env_ndk.sh:13` | NDK 版本定位 `$ANDROID_HOME/ndk/25.0.8775105` | 直接写到 NetPilot 的 `scripts/build_aar.sh` 顶部 |
| `libcore/build.sh` | gomobile bind 主脚本 | `scripts/build_aar.sh` (新建, 30 行内) |
| `libcore/init.sh` | 安装 gomobile 工具链 | 合并进 `scripts/build_aar.sh` 的 `ensure_gomobile()` 函数 |
| `./run` + `./run lib core` CI 入口 | 分层调度 | **不要**, 我们用单个 `scripts/build_aar.sh` 搞定 |

**目标脚本** (3B-1 产出之一): `scripts/build_aar.sh`, 30 行内, 输入零参数, 输出 `android/app/libs/netpilot.aar`。

---

## 11. 工具链版本清单

| 工具 | NekoBox 版本 | 官方最新稳定 | **NetPilot 3B-1 目标** | 证据 |
|---|---|---|---|---|
| Go | 1.23.1 (toolchain 1.23.6) | 1.26.1 (已装) | **1.25.9 (homebrew go@1.25)** ⚠️ 两轮预研才敲定 | 见下方 "Go 版本决策踩坑史" |
| gomobile CLI | MatsuriDayo fork master2 | `golang.org/x/mobile` 官方 | **官方 `golang.org/x/mobile` latest** ✅ 预研确认 | 2026-04-17 /tmp 测试成功, aar 40MB / 4-arch |
| gomobile 库 (go.mod) | v0.0.0-20231108 | v0.0.0-20260410 | **latest** (随 Go 1.25 要求) | latest 要求 Go ≥ 1.25 |
| Android NDK | r25b (25.0.8775105) | r27c | **homebrew android-ndk** `/opt/homebrew/share/android-ndk` | 本机现有, 已验证能编 40MB aar |
| Android compileSdk | 35 | 35 | **36** (我们已经是 36) | `android/app/build.gradle.kts:11` |
| Android targetSdk | 35 | 35 | **36** | `android/app/build.gradle.kts:16` |
| Android minSdk | 21 | - | **21** | `build.gradle.kts:15` + `libcore/build.sh:18 -androidapi 21` |
| sing-box | commit `9beb42f...` (NekoBox fork) | v1.13.8 (2026-04-14) | **v1.13.8** ✅ 预研确认 | `go get` 成功无 replace, 4 架构 bind 产物 40MB |
| sing-box build tags | `with_conntrack,with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api` | - | **`with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api`** (去 conntrack) | 预研 bind 成功, 未报未识别 tag |
| JDK | - | JDK 21 | **JDK 17 `/opt/homebrew/opt/openjdk@17`** | 对齐 `android/app/build.gradle.kts:32-38` |
| Go `-checklinkname=0` | (他们跑 Go 1.23.6 旧 linker) | - | **必须加进 `-ldflags`** | sing-box `libbox/pidfd_android.go:12` 用 `//go:linkname` 单向访问 `os.checkPidfdOnce`, 新 linker 会拒绝 |

### Go 版本决策踩坑史 (2026-04-17 第二轮预研)

第一轮预研(stub bind)曾结论"保留 Go 1.26.1 不降级" — **完全错误**。真 sing-box 加进去后撞了 3 层约束:

| 尝试 | Go 版本 | 错误 | 诊断 |
|---|---|---|---|
| 1 | 1.26.1 | `link: invalid reference to os.checkPidfdOnce` | Go 1.26 的 os.checkPidfdOnce 符号被重构, sing-box 旧 linkname 失效 |
| 2 | 1.23.12 | `certmagic@v0.25.2 requires go >= 1.24.0` | sing-box v1.13.8 传递依赖抬高门槛 |
| 3 | 1.23.12 + 降 mobile | `mobile@latest requires go >= 1.25.0` | mobile 老版也过不了 certmagic |
| 4 | 1.25.9 (strict linkname) | 同 #1 | Go 1.23+ 统一收紧 linkname 规则 |
| 5 | 1.25.9 + `-ldflags=-checklinkname=0` | ✅ 成功 | 官方逃生舱 |

**永久结论**: Go 版本必须 **1.25.x**, 且构建 ldflags 必须 **含 `-checklinkname=0`**。不能升 1.26 (linkname 失效), 不能降 1.24 (验证时 1.25 已装不再重复验, 但理论上 1.24 应也能用, 留给 3B-1 实装时验证)。

**必须 export 的环境变量** (代码实现时写进 `scripts/build-aar.sh`):
```bash
export GOTOOLCHAIN=local                    # 防止自动升级到 1.26
export GOROOT=/opt/homebrew/opt/go@1.25/libexec
export PATH="$GOROOT/bin:$PATH:$(go env GOPATH)/bin"
export JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home
export ANDROID_NDK_HOME=/opt/homebrew/share/android-ndk
export GOPROXY="https://goproxy.cn,https://goproxy.io,direct"  # 关键 — 见 #M9
export GOSUMDB=off  # 同上
```

`go.mod` 主模块声明需改为 `go 1.25`。

**⚠️ 新发现的 Dogfooding 坑 (M9)**: 开发机本身在跑 sing-box/Netpilot 作 TUN 时, `go get` 和 `go list` 通过 TUN 路由到 `proxy.golang.org` 会卡死(预研阶段实测卡 6 分钟 0% CPU)。必须要么(a)在 sing-box route 规则里把 `proxy.golang.org` / `sum.golang.org` 走 direct, 要么(b)开发期间统一用 `GOPROXY=https://goproxy.cn,https://goproxy.io,direct`。`scripts/build-aar.sh` 里硬编码 proxy 链路比较稳。

**⚠️ `golang.org/x/mobile/bind` 需要 blank import 锁定 (M10)**: gomobile CLI 工具需要 `golang.org/x/mobile/bind` package 可用,但我们代码不直接 import, 所以 `go mod tidy` 会把它清掉。解决: 在 `libcore/` 目录下放一个 `gomobile_deps.go` 文件, 内容:
```go
package libcore
import _ "golang.org/x/mobile/bind"
```

---

## 12. 5 个潜在阻塞点 (2026-04-17 预研后状态)

### 阻塞 #1: 官方 gomobile bind 两个 package — ✅ 已验证可行
**预研结果**: 在 `/tmp/netpilot-3b-preflight/multipkg/` 建空 `libcore/` + `mobile/` 两个 package (后者 import 前者), 用官方 `gomobile bind -target=android/arm64 ./libcore ./mobile` 成功产出 1.5MB aar。`classes.jar` 内含:
```
libcore/BoxInstance.class
libcore/Libcore.class
mobile/Client.class
mobile/Mobile.class
```
**两个 Go package 都有独立 Java 命名空间, 所有导出类型都编进去了**。方向 B(双层分离架构)无障碍。

### 阻塞 #2: Go 1.26.1 版本兼容性 — ✅ 已验证保留 1.26.1
**预研结果**: 对 `internal/` + `mobile/` + `cmd/` 全仓 grep `omitzero|unique\.|weak\.|testing/synctest|range func\(|iter\.|\"slices\"|\"maps\"` 零匹配。现有代码不依赖任何 Go 1.23+ 新标准库特性。`go 1.26.1 + gomobile latest + sing-box v1.13.8` 链路在预研沙箱里打通。**Go 版本不降级**, 保留 1.26.1。

### 阻塞 #3: sing-box libbox 必须走 OpenTun() 回调 — ✅ 已证实 + 方案已确定
**预研结果**: 读官方 `v1.13.8/experimental/libbox/service.go`, 确认:
```go
tunFd, err := w.iif.OpenTun(&tunOptions{options, routeRanges, platformOptions})
dupFd, err := dup(int(tunFd))
options.FileDescriptor = dupFd
```
**没有预传 fd 的路径**, 必须实现 `PlatformInterface.OpenTun()`。
- `mobile/netpilot.go` StartTun 签名从 `(fd int32, config string)` 改为 `(config string)`, 新增 `SetPlatformInterface(iface)`
- Kotlin 侧 3B-3 写 `NetPilotNativeInterface` (抄 NekoBox 的 `NativeInterface.kt`), 其中 `openTun()` 延迟到 Go 调用时才 `establish()`
- 3B-1 阶段可先用 `NopPlatformInterface` stub 走编译链路(详见第 6.3 节伪代码)

### 阻塞 #4: aar 体积超 Play Store 50MB 硬限 — ✅ 已验证远低于硬限
**预研结果 (2026-04-17)**:
- 单架构 arm64-v8a: **9.4MB aar**
- 全四架构: **40MB aar** (未过 proguard, 未分包)
- 生产 App Bundle 用户实际下载单 APK: **~15-20MB**
- 双 ARM 架构合并 APK (arm64+armv7): **~25-30MB**, 远低于 50MB 硬限
- 推荐发布策略: **Android App Bundle (.aab)**, Google Play 会按设备架构分发相应 APK
- 如果必须走 APK 直传渠道, 可单独为 arm64 打包 (覆盖 Android 8+ 主流设备), 体积最优

### 阻塞 #5: `VpnService.protect(fd)` 没接好导致回环 — 🟢 3B-3 再处理
**状态未变**: 3B-1 不触碰, 3B-3 用 `libneko/protect_server/protect_server_linux.go` 单文件(34 行 Apache-2.0, 可直接内联)。

### 新增阻塞 #6: GOPROXY 走 sing-box TUN 卡死 — 🟡 绕路已确定
**预研中发现**: 本机 sing-box/Netpilot TUN 在跑时 (`198.18.0.1` gateway), `go get / go list` 通过 TUN 访问 `proxy.golang.org` 会卡 HTTPS 握手(实测 0% CPU 6 分钟)。
**方案**: `scripts/build-aar.sh` 头部硬编码 `export GOPROXY=https://goproxy.cn,direct; export GOSUMDB=off`, 跳过 proxy.golang.org。顺便记入 CLAUDE.md Known Issue **#M9**。

---

## 13. 3B-1 Definition of Done

必须全部满足才能宣布 3B-1 完成, 进入 3B-2:

- [ ] Go 版本降到 1.23.6, `go mod tidy` 无错
- [ ] 引入 `github.com/sagernet/sing-box v1.13.8` 为直接依赖
- [ ] 新建 `libcore/` package, 至少包含 `box.go` (BoxInstance 三态) + `platform_box.go` (PlatformInterface stub wrapper) + `box_include.go` (build-tag 汇总)
- [ ] `scripts/build_aar.sh` 存在, 单命令可 `bash scripts/build_aar.sh` 产出 `android/app/libs/netpilot.aar`
- [ ] aar 体积 ≤ 40MB (arm64-v8a + armeabi-v7a 双架构), 记录到 `docs/phase-3b-plan.md` 更新
- [ ] aar 符号表里能找到 `libcore.BoxInstance`, `libcore.NewBoxInstance`, `libcore.BoxPlatformInterface` (跑 `unzip -p netpilot.aar classes.jar | javap -cp - libcore.BoxInstance`)
- [ ] Android Studio `./gradlew :app:assembleDebug` 成功, `import libcore.BoxInstance` 不报红
- [ ] `mobile/netpilot.go` 的 `StartTun/StopTun/TunRunning` 已改造为调用 `libcore.BoxInstance` (真跑路径留 3B-3, 但签名对上)
- [ ] `CLAUDE.md` Known Issue `#H1 libbox 数据面未集成` 状态从 🔴 改为 🟡, 标注 "3B-1 完成, 3B-3 真机跑"
- [ ] 这份 `docs/phase-3b-plan.md` 在实施过程中被更新, 每遇阻塞点记一笔

**不在 3B-1 范围内** (明确排除):
- 真机走流量 (3B-3)
- iOS xcframework (3B-4)
- `NativeInterface.kt` 实现 (3B-2)
- geoip.db / geosite.db 资源管理 (3B-3)
- AIDL/Binder UI 回调 (3B-5 或推后)

---

## 14. 未参考到的自造决策清单

这些决策 NekoBox/Hiddify 均未直接覆盖, 属于 **NetPilot 自造**, 写代码时要特别小心, commit message 明确标注 "自造":

1. **保留 `mobile/` 业务层 + `libcore/` 内核层的分离** — NekoBox 把一切塞 `libcore` + Kotlin 直接调业务, 我们因为有 LLM Agent + Overlay + Subscription 三大业务模块, 需要业务层独立。**自造**, 风险: gomobile bind 多包可能踩坑 (见阻塞 #1)
2. **用官方 `golang.org/x/mobile` 而非 `gomobile-matsuri`** — 没人验证过官方 gomobile 对 sing-box 的编译结果, 我们是第一个。**自造**, 风险: 若官方有 issue 需要回退到 matsuri
3. **跳过本地 sing-box fork, 直接 `go get v1.13.8`** — NekoBox 是 `replace => ../../sing-box`, Hiddify 是预编译 xcframework, 两者都不是 "直接 vendor 官方 tag"。**自造**, 风险: sing-box 官方有 NekoBox 已修但未 upstream 的 bug
4. **StartTun 先传 fd 不走 OpenTun 回调** — NekoBox 和 sing-box 官方 tun inbound 都依赖 `OpenTun()` 反向拿 fd, 我们把 fd 作为 StartTun 参数一起传进来 ❓ 待验证可行性。**自造**, 风险: sing-box tun inbound 可能 hardcode 了 OpenTun 调用路径
5. **Android compileSdk 36 + 我们非 NekoBox 的 35** — 我们比 NekoBox 新一版。**半自造**, 风险低, 但需要关注 Android 15 对 VpnService 的 QPR 变更
6. **不引入 libneko 整个包, 只在 3B-3 引入 `protect_server` 子包** — NekoBox 把整个 libneko `// replaced` 到本地, 我们只取一个子包。**自造**, 风险: libneko 其他子包可能是 sing-box 的隐式依赖 (例如 `neko_log` 被 libcore/nb4a.go 导入)

---

## 附录: 关键文件清单快照

本次调研读过的关键文件 (供未来回溯):

| 文件 | 行数 | 本次用途 |
|---|---|---|
| `~/References/nekobox/libcore/build.sh` | 25 | gomobile 命令权威源 |
| `~/References/nekobox/libcore/init.sh` | 28 | gomobile-matsuri 安装 |
| `~/References/nekobox/libcore/go.mod` | 88 | sing-box 版本 + replace 规则 |
| `~/References/nekobox/buildScript/init/env_ndk.sh` | 25 | NDK 版本 |
| `~/References/nekobox/.github/workflows/preview.yml` | 70+ | CI 入口 `./run lib core` |
| `~/References/libneko/` 各子目录 | - | 工具性质判定 |
| `~/Netpilot/mobile/netpilot.go` | 474 | 改造目标 |
| `~/Netpilot/android/app/src/main/java/com/foxnetpilot/netpilot/vpn/NetPilotVpnService.kt` | 115 | 改造目标 (3B-3) |
| `~/Netpilot/go.mod` | 10 | 当前依赖基线 |
| `~/Netpilot/android/app/build.gradle.kts` | 40+ | Android 版本基线 |

证据覆盖率: 约 95% 的技术判断带有具体文件路径+行号; 剩余 5% 已打 ❓ 标记, 代码实现阶段首先解决。
