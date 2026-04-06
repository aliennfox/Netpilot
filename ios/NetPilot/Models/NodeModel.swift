import Foundation

struct NodeModel: Codable, Identifiable {
    var id: String { tag }
    let tag: String
    let type: String
    let server: String
    let port: Int
    let alive: Bool
    let latency: Int
    let groupTag: String
    let active: Bool

    enum CodingKeys: String, CodingKey {
        case tag, type, server, port, alive, latency
        case groupTag = "group_tag"
        case active
    }

    var protocolShort: String {
        switch type.lowercased() {
        case "shadowsocks": return "SS"
        case "hysteria2": return "Hy2"
        case "vless": return "VLESS"
        case "vmess": return "VMess"
        case "trojan": return "Trojan"
        default: return type
        }
    }

    var latencyText: String {
        if latency <= 0 { return "超时" }
        return "\(latency)ms"
    }
}
