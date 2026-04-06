import Foundation

struct ChatMessage: Identifiable {
    let id = UUID()
    let role: Role
    let content: String
    let timestamp: Date
    let actions: [ChatAction]

    enum Role {
        case user
        case assistant
    }

    init(role: Role, content: String, actions: [ChatAction] = []) {
        self.role = role
        self.content = content
        self.timestamp = Date()
        self.actions = actions
    }
}

struct ChatAction: Codable, Identifiable {
    var id: String { command }
    let label: String
    let command: String
}

struct ChatResponseData: Codable {
    let reply: String
    let source: String
    let stages: [String]?
    let actions: [ChatAction]?
}
