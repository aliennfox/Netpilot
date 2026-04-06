import Foundation

struct SubscriptionModel: Codable, Identifiable {
    let id: String
    let name: String
    let url: String
    let nodeCount: Int
    let tags: [String]?
    let lastUpdate: Date?
    let autoUpdate: Bool
    let interval: Int

    enum CodingKeys: String, CodingKey {
        case id, name, url, tags
        case nodeCount = "node_count"
        case lastUpdate = "last_update"
        case autoUpdate = "auto_update"
        case interval = "interval_minutes"
    }
}

struct TemplateModel: Codable, Identifiable {
    let id: String
    let name: String
    let description: String
    let keywords: [String]
}
