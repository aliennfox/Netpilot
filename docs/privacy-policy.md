---
title: Pilotty 隐私政策
---

# Pilotty 隐私政策

**最后更新**: 2026-04-24

本政策说明 Pilotty (以下称 "本应用") 如何处理你的数据。 English version: [Privacy Policy](privacy-policy-en.html).

---

## 1. 数据收集

**本应用不收集、不上传、不共享你的任何个人身份数据 (PII)**。 没有用户注册, 没有账户系统, 没有行为分析, 没有崩溃上报, 没有广告 SDK。

以下数据仅保存在你的设备本地, 本应用或其开发者无法访问:

- **节点订阅 URL 与节点配置**: 存于 App 私有目录 (`filesDir`), Android 沙箱保护。
- **LLM API Key (可选)**: 如果你配置了 DeepSeek / 硅基流动 API Key 以启用 Agent 功能, Key 以明文形式存于 Android `SharedPreferences`, 仅本应用可读。
- **Agent 对话历史**: 本地 SharedPreferences, 最近 40 条消息。
- **VPN 日志**: 内存环形缓冲区, 最多 500 条, 应用关闭即丢失。 你可通过"导出"按钮主动分享给自己 (见第 4 节)。
- **路由规则 / 模板 / 快照**: 本地 JSON 文件, 记录你对代理规则的修改, 支持回滚。

---

## 2. 网络流量

本应用是一个 **VPN 代理客户端**, 它的本职就是路由你的网络流量。

- **代理上游**: 你自己配置的节点订阅决定流量去向 (Shadowsocks / VMess / VLESS / Trojan / Hysteria2 / TUIC / WireGuard 等)。 本应用开发者不运营任何代理服务器, 也无法看到流量内容。
- **LLM 请求 (可选)**: 如果你启用 Agent 功能, 你的自然语言输入会发送给你自己配置的 LLM 服务商 (默认 DeepSeek 通过硅基流动网关) 用于生成回复。 回复内容保存在本地对话历史。 未启用时, 本应用仅使用本地规则引擎, 不向任何外部 API 发请求。
- **订阅更新 HTTP 请求**: 更新订阅时会向你提供的 URL 发 HTTPS GET 请求拉取最新节点列表, 仅此一项外部网络调用。

---

## 3. 权限说明

| 权限 | 用途 |
|---|---|
| `INTERNET` / `ACCESS_NETWORK_STATE` / `CHANGE_NETWORK_STATE` | VPN 核心功能, 代理上游连接 |
| `FOREGROUND_SERVICE` / `FOREGROUND_SERVICE_SYSTEM_EXEMPTED` | 保持 VPN 服务在后台运行, 防被系统杀掉 |
| `POST_NOTIFICATIONS` (Android 13+) | 显示 VPN 连接状态通知 |
| `QUERY_ALL_PACKAGES` | Per-App VPN 功能需要列出已安装应用, 让你选择哪些 App 走代理 |
| `CAMERA` | QR 扫码导入节点。 **可选**: 无相机或拒绝授权也能正常使用应用的其他功能 |

**本应用永不请求位置、联系人、短信、麦克风、健康数据、账号等敏感权限。**

---

## 4. 你的数据, 你的控制

- **卸载即清除**: 卸载 App 会删除所有本地数据 (订阅、API Key、对话历史、日志)。
- **日志导出**: Settings → 观测 → 实时日志 → 导出, 可手动分享日志给自己用于排障。 此动作由你主动发起, 开发者不会收到副本。
- **配置备份**: Settings → 配置导出 / 导入, 本地 JSON 文件, 由你自行管理。

---

## 5. 第三方服务 (仅当你主动启用)

如果你在 Settings 中配置以下任一项, 对应第三方服务的隐私政策将同时适用:

- **DeepSeek / 硅基流动 LLM API**: 启用 Agent 后生效。 请参考 [硅基流动服务条款](https://siliconflow.cn/).
- **节点订阅服务商**: 你粘贴的订阅 URL 属于你自己选择的服务商, 本应用与订阅服务商无任何合作关系。

---

## 6. 儿童隐私

本应用不面向 13 岁以下儿童, 也不主动收集任何年龄信息。

---

## 7. 政策更新

本政策可能随应用功能演进更新。 重大变更会在应用内通知。 最新版本始终见: <https://aliennfox.github.io/Netpilot/privacy-policy.html>

---

## 8. 联系

- GitHub Issues: <https://github.com/aliennfox/Netpilot/issues>
- 邮箱: dannyalberttires@gmail.com
