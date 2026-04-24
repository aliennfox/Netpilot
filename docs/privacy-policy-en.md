---
title: Pilotty Privacy Policy
---

# Pilotty Privacy Policy

**Last updated**: 2026-04-24

This policy explains how Pilotty ("the App") handles your data. 中文版本: [隐私政策](privacy-policy.html).

---

## 1. Data Collection

**The App does not collect, upload, or share any personally identifiable information (PII).** No user accounts, no sign-in, no behavioral analytics, no crash reporting, no ad SDKs.

The following data is stored only on your device and is inaccessible to the App's developer:

- **Node subscriptions and node configs**: stored in the App's private directory (`filesDir`), protected by Android's per-app sandbox.
- **LLM API key (optional)**: if you configure a DeepSeek / SiliconFlow API key to enable the Agent feature, the key is stored in plaintext in Android `SharedPreferences`, readable only by this App.
- **Agent conversation history**: local SharedPreferences, last 40 messages.
- **VPN logs**: in-memory ring buffer, up to 500 entries, discarded when the App is closed. You can export them manually via the Export button (see section 4).
- **Routing rules / templates / snapshots**: local JSON files recording your proxy-rule edits, with rollback support.

---

## 2. Network Traffic

The App is a **VPN proxy client**; its core function is routing your network traffic.

- **Proxy upstream**: your own subscription-configured nodes determine where traffic goes (Shadowsocks, VMess, VLESS, Trojan, Hysteria2, TUIC, WireGuard, etc.). The App's developer runs no proxy servers and has no access to the traffic content.
- **LLM requests (optional)**: if you enable the Agent, your natural-language input is sent to your configured LLM provider (default: DeepSeek via SiliconFlow gateway) to generate responses. Responses are saved in local conversation history. When the Agent is disabled, the App uses only its on-device intent router and makes no outbound API calls.
- **Subscription update HTTP**: when you refresh subscriptions, the App sends an HTTPS GET to your provided URL to fetch the latest node list. This is the App's only initiated external network call.

---

## 3. Permissions

| Permission | Purpose |
|---|---|
| `INTERNET` / `ACCESS_NETWORK_STATE` / `CHANGE_NETWORK_STATE` | Core VPN functionality and upstream connections |
| `FOREGROUND_SERVICE` / `FOREGROUND_SERVICE_SYSTEM_EXEMPTED` | Keep the VPN service alive in background so Android does not kill it |
| `POST_NOTIFICATIONS` (Android 13+) | Display VPN connection status notification |
| `QUERY_ALL_PACKAGES` | Per-App VPN requires listing installed apps so you can pick which apps use the proxy |
| `CAMERA` | QR scan for importing nodes. **Optional**: the App functions without a camera or when this permission is denied |

**The App never requests location, contacts, SMS, microphone, health, or account-related sensitive permissions.**

---

## 4. Your Data, Your Control

- **Uninstall clears everything**: uninstalling the App deletes all local data (subscriptions, API keys, conversation history, logs).
- **Log export**: Settings → Observe → Live logs → Export, shares logs with yourself for troubleshooting. This is a user-initiated action; the developer receives no copy.
- **Config backup**: Settings → Export / Import, local JSON files managed by you.

---

## 5. Third-party Services (only if you opt in)

If you configure any of the following in Settings, that third party's privacy policy additionally applies:

- **DeepSeek / SiliconFlow LLM API**: active when the Agent is enabled. See [SiliconFlow Terms](https://siliconflow.cn/).
- **Subscription providers**: the URLs you paste are providers of your own choosing; the App has no business relationship with them.

---

## 6. Children's Privacy

The App is not intended for children under 13. It does not collect age information.

---

## 7. Policy Updates

This policy may be updated as the App evolves. Material changes will be announced in-app. Latest version: <https://aliennfox.github.io/Netpilot/privacy-policy-en.html>

---

## 8. Contact

- GitHub Issues: <https://github.com/aliennfox/Netpilot/issues>
- Email: dannyalberttires@gmail.com
