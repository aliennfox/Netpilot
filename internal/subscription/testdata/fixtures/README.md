# Subscription Fixtures

端到端回归矩阵的输入样本。 每个文件是一个真实订阅格式的脱敏缩影。

| 文件 | 格式 | 包含协议 |
|---|---|---|
| `clash-mixed.yaml` | Clash / Clash.Meta YAML | SS, VMess, VLESS+Reality, Trojan, Hy2, TUIC, AnyTLS, WG |
| `singbox-mixed.json` | sing-box native JSON | VLESS+Reality, TUIC, Trojan |
| `base64-uri-mixed.txt` | Base64 (SIP002 URI list) | SS, Trojan, VLESS, Hy2, TUIC, AnyTLS, Hy1 |

**规则**:
- 所有字段必须**脱敏**(假 UUID / 假 password / 假域名 *.example.com)
- 不能出现真实可连的节点
- 单文件应有"这类订阅常见会出现的" ≥1 种协议

**添加新 fixture 的流程**:
1. 拿到可疑订阅(真用户报告"粘贴进去丢节点")
2. 把所有 UUID / password / 真实域名 replace 成 example.com / fake-secret
3. 落到这里,文件名按 "格式-关键字.扩展" 命名
4. 在对应 fmt 的 `TestSubscriptionFixtures` 里加一条期望的节点数断言
