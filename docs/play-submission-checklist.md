# Google Play Submission Checklist — Pilotty Android v1

> **状态**: 预研完成 (2026-04-22),用于 3B-4 抛光阶段最终提交 Play 审核前对照使用
> **来源**: Google Play Console Help 官方文档 + 行业踩坑经验 (NekoBox / VPN 同类 App 分发社区)
> **政策版本**: 截至 2026-04-22 官网版本

---

## 1. VpnService Declaration Form — 必填项

VpnService 类 App 必须在 Play Console → App content → VPN declaration 里完成声明表。以下 7 问必答:

- [ ] **Q1**: VPN 是否为 App 的 core functionality? Pilotty 回答 **"是"** (代理客户端即核心)。选"是"可跳过 Q2。
- [ ] **Q2**: (若 Q1=否) 在 7 类允许之外的非核心 VPN 用途里选择?Pilotty 不适用
- [ ] **Q2a**: (若 Q2="None of the above") 详细解释。 不适用
- [ ] **Q3**: **Demo video** 链接 (YouTube / cloud storage URL, 公开可访问)
  - 长度 **≤ 90 秒**
  - 必须展示:打开 App → 显示 prominent disclosure → 用户同意/拒绝双路径 → 成功走流量
  - 建议加旁白或字幕,解释为什么需要 VpnService 以及用途
- [ ] **Q4a**: 是否通过 VpnService **收集或分享** 个人/敏感数据? Pilotty 当前回答 **"否"** —— 本地处理流量,不上报任何 user traffic 到我们的服务器。 (LLM API Key 和 chat message 走另外的答题路径, 见 §2)
- [ ] **Q4b**: (若 Q4a=是) 数据类型清单。 不适用
- [ ] **Q5**: **Prominent disclosure video** —— 展示 App 内部 disclosure 的视频 (可与 Q3 合并)
- [ ] **Q5a**: 是否 redirect/manipulate 其他 App 的流量用于 **monetization**? Pilotty 回答 **"否"**
- [ ] **Q5b**: (若 Q5a=是) 说明场景。 不适用

### Q3/Q5 Demo Video 脚本 (90s 提纲)

```
0-10s   首页打开, 显示"Pilotty 代理客户端, 仅你控制的节点"
10-25s  进入"添加订阅"页面: 显示 prominent disclosure 文案 -- "本 App 将通过
        VpnService 接管设备流量, 并转发至你指定的第三方代理节点"
25-35s  用户点击"同意并继续"; 不同意则返回
35-55s  订阅导入 → 节点列表出现 → 用户点一个节点 → 系统 VPN 权限对话框
55-75s  VPN 连接成功, 浏览器访问 https://ipinfo.io 显示出口 IP 已变
75-90s  截屏 "所有流量均加密上行至你选择的节点, 不经过 Pilotty 服务器"
```

**录屏工具**: `adb shell screenrecord /sdcard/netpilot-demo.mp4 --time-limit 90`;macOS QuickTime 录 iPhone 的方式这里不适用

## 2. Data Safety Form — 必填项

Play Console → App content → Data safety。 Pilotty 需声明的数据类别:

### 2.1 会收集 / 使用的数据

| 数据类型 | 是否收集 | 是否分享 | 目的 | 是否可选 |
|---|---|---|---|---|
| **LLM API Key** (用户自填) | ✅ 本地加密存储 | ❌ | App 功能 (Agent Chat) | ✅ 用户可跳过 |
| **Chat 消息内容** | ❌ 不存设备之外 | ✅ 发给 deepseek.com / api.siliconflow.cn | App 功能 (LLM 响应) | ✅ 用户可不用 Chat tab |
| **订阅 URL** | ✅ 本地明文 (用户自填) | ❌ | App 功能 | 必填用才能用 |
| **节点列表 / 流量统计** | ✅ 本地明文 | ❌ | App 功能 | 随订阅生效 |
| **设备标识符 (IMEI/Android ID)** | ❌ | ❌ | 不收集 | — |
| **Crash 日志** | ✅ 本地 (M7 "导出" 按钮) | ❌ 不自动上报 | Bug 诊断 | 默认关, 用户手动触发 |

**关键点**: LLM Chat 这项必须申报 "Chat messages shared with third party (DeepSeek via SiliconFlow)"。 在 Play Data Safety 的分类里归到:
- **Data type**: Messages → In-app messages
- **Purpose**: App functionality
- **Processing**: Shared with third party
- **Optional**: Yes (用户可选择不用 Chat tab)

### 2.2 In-App Prominent Disclosure

Chat tab 第一次打开时,必须弹 dialog 告知:
> "你输入的消息将发送到 SiliconFlow (api.siliconflow.cn) 托管的 DeepSeek 大模型处理。Pilotty 不保留聊天记录服务器副本。API Key 存储在本机加密区。"
> [同意] [返回]

同意后再展示 input box。"同意" 状态存 EncryptedSharedPreferences,下次不再弹。

## 3. 隐私政策

### 3.1 必备章节 (托管到 GitHub Pages 或静态站)

- [ ] App 名称 + 开发者主体
- [ ] 生效日期 (版本号对应)
- [ ] **收集的信息类型**: 参照 §2.1 表格
- [ ] **数据用途**: 明确 "仅为实现 App 功能, 不做广告、分析、画像"
- [ ] **第三方分享**: 
  - SiliconFlow API (LLM 推理): 仅 chat 消息内容
  - 用户自配订阅服务器: 所有走代理的流量 (本来就是用户授权的)
- [ ] **数据存储位置**: 本机 `filesDir`, 不上云
- [ ] **数据保留期**: 用户自删;App 卸载时全部清除
- [ ] **儿童隐私** (COPPA): App 未面向 < 13 岁用户
- [ ] **联系方式**: 开发者邮箱
- [ ] **政策变更通知方式**

### 3.2 隐私政策的 URL 要求

- 必须 HTTPS
- 在 Play Console App content → Privacy policy 填写
- App 内 "关于" 页必须有可点击入口 (M6 Must-have 一部分)

## 4. Keystore 与签名策略

### 4.1 方案选择

**采用 Play App Signing** (Google 推荐, 也是行业标准):
- **Upload key**: 我们自己保管, 用于签名上传到 Play
- **App signing key**: Google 保管, Play 用它在分发前最终签名
- **优势**: upload key 丢失/泄露 → Console 可以申请 reset,不影响已安装用户。若不用 Play App Signing, keystore 丢了就完蛋

### 4.2 Keystore 生成

```bash
# 在 ~/.netpilot-keys/ (不要放 repo!) 生成 upload keystore
mkdir -p ~/.netpilot-keys
cd ~/.netpilot-keys
keytool -genkey -v \
    -keystore netpilot-upload.keystore \
    -keyalg RSA -keysize 4096 -validity 10000 \
    -alias netpilot-upload \
    -storepass '<STORE_PASS>' -keypass '<KEY_PASS>' \
    -dname "CN=Pilotty, O=foxnetpilot, L=Internal, C=CN"
```

### 4.3 备份策略 (三地三介质)

- [ ] **本地工作机**: `~/.netpilot-keys/` (600 权限)
- [ ] **iCloud Keychain / 1Password**: keystore 文件 + 密码 单独条目 (二选一, 推荐 1Password 因 iCloud Keychain 不存二进制)
- [ ] **物理离线备份**: 加密 U 盘或打印二维码 (存到金属标签牌尤佳, 防火防水)
- [ ] **绝不** git commit: 本 repo 的 `.gitignore` 明确排除 `*.keystore` `*.jks` `keystore.properties`

### 4.4 CI / 构建时注入

使用 `keystore.properties` 本地文件 (gitignored):
```properties
storeFile=/Users/fox/.netpilot-keys/netpilot-upload.keystore
storePassword=<STORE_PASS>
keyAlias=netpilot-upload
keyPassword=<KEY_PASS>
```

然后 `build.gradle.kts` 读取。 见 §6。

## 5. ProGuard / R8 规则

### 5.1 关键发现

- sing-box libbox 通过 gomobile 生成 JNI 桥接类, **反射调用密集**。R8 默认策略会 strip 掉没显式引用的符号
- NekoBox 生产环境用的是 `-dontobfuscate` (不改名,只 strip + inline)。这是权衡产物:混淆能省更多体积但 libbox crash 风险更高
- kotlinx.serialization `@Serializable` 生成的 companion 对象必须 keep,否则运行时 `SerializationException: Serializer for class X is not found`

### 5.2 建议的 proguard-rules.pro (初版)

```proguard
# === gomobile 桥接 (libbox + mobile 包) ===
-keep class go.** { *; }
-keep class mobile.** { *; }
-keep class libbox.** { *; }
-keepclassmembers class libbox.** { *; }

# === kotlinx.serialization ===
-keepattributes *Annotation*, InnerClasses
-dontnote kotlinx.serialization.AnnotationsKt
-keep,includedescriptorclasses class com.foxnetpilot.netpilot.data.**$$serializer { *; }
-keepclassmembers class com.foxnetpilot.netpilot.data.** {
    *** Companion;
}
-keepclasseswithmembers class com.foxnetpilot.netpilot.data.** {
    kotlinx.serialization.KSerializer serializer(...);
}

# === Compose runtime (Material3 反射使用) ===
-dontwarn androidx.compose.**

# === 保留行号便于 crash 排查 ===
-keepattributes SourceFile,LineNumberTable
-renamesourcefileattribute SourceFile

# === 不混淆 (同 NekoBox, 避免 libbox crash) ===
-dontobfuscate

# === sing-box / Go runtime 内部常见误报 ===
-dontwarn sun.misc.**
-dontwarn java.beans.**
```

### 5.3 风险

- `-dontobfuscate` 会让 release APK 比完整混淆大 10-15%。但好处是 crash stack trace 可读,且避开 libbox 反射踩坑
- 首次 `assembleRelease` 必须真机验证 **VPN 连接 + Chat + 订阅导入**,不能只验证"能启动"

## 6. build.gradle.kts 变更清单

- [ ] 顶部读取 `keystore.properties` (若存在)
- [ ] `android.signingConfigs.create("release") { ... }` 从 properties 注入
- [ ] `android.buildTypes.release { signingConfig = ...; isMinifyEnabled = true; isShrinkResources = true }`
- [ ] `android.buildTypes.debug` 保持默认, 不改
- [ ] `.gitignore` 新增 `keystore.properties` + `*.keystore` + `*.jks`

## 7. minSdk / targetSdk / Android 兼容性

- **当前**: minSdk=21 (Android 5.0 Lollipop), targetSdk=36 (Android 16), compileSdk=36
- **风险点**:
  - sing-box 官方 SFA (sing-box for android) minSdk=21, 所以 libbox 本身兼容 21+
  - libbox 用到 `VpnService` protect(fd) + `PROXY_CONFIG` 在 Q (API 29) 后才 stable, 低版本旧 code path 已验证
  - gomobile bind 产物支持 armeabi-v7a/arm64-v8a/x86/x86_64 (当前 aar 已含 4 架构)
  - #M13 已修的 StateFlow API Level 兼容:需 API 21+,无问题
- **建议**: minSdk 保持 21 不变 (覆盖 ~99% 国内用户基数),targetSdk 保持 36 (Play 2026 上架硬要求)
- **验证方式**: 真机 Android 5.0 (物理机难找) 或 API 21 emulator 跑一次 Smoke

## 8. 应用名 / 商标可用性

- [ ] 搜 Google Play "Pilotty" — 若存在同名 App,考虑 "Pilotty AI" / "Pilotty Agent" 加 disambiguate
- [ ] 搜 trademark DB (tmsearch.uspto.gov + 国家知识产权局) — 排除冲突
- [ ] 不一定要注册商标, 但要确保 Play 不因"可能侵权" 拒审

### 结论

2026-04-22 预研阶段暂未进 Play Store 搜索验证 (执行阶段再做)。 若 Pilotty 已占用 → 退路:**Pilotty Agent** / **Pilotty Kit**。

## 9. 发布前最终 Gate (M1-M9 全绿后)

- [ ] `./gradlew assembleRelease` 成功,APK < 50MB (minify 后应该能达成)
- [ ] 真机安装 release APK,golden path (订阅导入 → 切节点 → 浏览器走代理) 通过
- [ ] crash 采集 7 天无 P0
- [ ] 隐私政策 URL 活的,Data Safety 表填写,VPN declaration 视频就绪
- [ ] Play Console 创建应用 → 设置分级 → 上传 AAB (非 APK, Play 偏好 AAB)
- [ ] 内部测试轨道先发 10 人以内验证;外部测试轨道 50-100 人;正式上架

## 10. 参考来源

- [Google Play VpnService 政策 (answer/12564964)](https://support.google.com/googleplay/android-developer/answer/12564964)
- [Play Data Safety Form (answer/10144311)](https://support.google.com/googleplay/android-developer/answer/10144311)
- [Play App Signing (answer/9842756)](https://support.google.com/googleplay/android-developer/answer/9842756)
- [Android App Signing 官方指南](https://developer.android.com/studio/publish/app-signing)
- [R8 / ProGuard 配置](https://developer.android.com/build/shrink-code)
- `~/References/nekobox/app/proguard-rules.pro` (NekoBox 的实际 proguard 规则作为基线参考)
- sing-box GitHub issue #1895 (gomobile 反射相关 crash)

---

## 附录 A:Pilotty 特有风险点

- **LLM API Key 存本地**: 即使用 EncryptedSharedPreferences,仍属 "sensitive credentials storage"。 Data Safety 按 "Stored locally only" 申报,不声明为"collection" 因为我们不传到自己服务器
- **Chat 消息过 SiliconFlow**: 技术上是 "shared with third party" 必须披露,不能藏
- **Agent 自动修改配置**: 相当于"写权限"。不需要 Play 申报,但应在 App 内部有 permission / audit log (Phase 1 已实现 `/trust` 模式)
- **机场订阅解析**: 我们下载用户订阅 URL → 解析节点。Play 可能质疑"是否下载的是黑产代理服务"。辩护角度:用户自主填入 URL,同 browser 访问任意 URL 无本质差异
