import Foundation

struct StatusModel: Codable {
    let currentNode: String
    let mode: String
    let nodeCount: Int
    let connections: Int
    let upload: Int64
    let download: Int64
    let agentReady: Bool

    enum CodingKeys: String, CodingKey {
        case currentNode = "current_node"
        case mode
        case nodeCount = "node_count"
        case connections
        case upload, download
        case agentReady = "agent_ready"
    }

    static let empty = StatusModel(
        currentNode: "-",
        mode: "unknown",
        nodeCount: 0,
        connections: 0,
        upload: 0,
        download: 0,
        agentReady: false
    )

    var uploadFormatted: String { formatBytes(upload) }
    var downloadFormatted: String { formatBytes(download) }

    private func formatBytes(_ bytes: Int64) -> String {
        let kb = Double(bytes) / 1024
        let mb = kb / 1024
        let gb = mb / 1024
        if gb >= 1 { return String(format: "%.1f GB", gb) }
        if mb >= 1 { return String(format: "%.1f MB", mb) }
        if kb >= 1 { return String(format: "%.1f KB", kb) }
        return "\(bytes) B"
    }
}
