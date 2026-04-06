import Foundation

struct APIResponse<T: Decodable>: Decodable {
    let success: Bool
    let data: T?
    let error: String?
}

class APIClient {
    static let shared = APIClient()
    private let baseURL = "http://localhost:8080"
    private let session: URLSession
    private let decoder: JSONDecoder

    private init() {
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 15
        session = URLSession(configuration: config)

        decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
    }

    // MARK: - Status

    func getStatus() async throws -> StatusModel {
        return try await get("/api/status")
    }

    // MARK: - Nodes

    func getNodes() async throws -> [NodeModel] {
        return try await get("/api/nodes")
    }

    func switchNode(group: String = "proxy-group", node: String) async throws {
        let _: [String: String] = try await post("/api/switch", body: [
            "group": group,
            "node": node,
        ])
    }

    // MARK: - Latency

    func testLatencyAll() async throws {
        let _: [String: String] = try await post("/api/latency/all", body: [String: String]())
    }

    // MARK: - Mode

    func setMode(_ mode: String) async throws {
        let _: [String: String] = try await post("/api/mode", body: ["mode": mode])
    }

    // MARK: - Chat

    func sendChat(message: String) async throws -> ChatResponseData {
        return try await post("/api/chat", body: ["message": message])
    }

    // MARK: - Rules

    func getRules() async throws -> [RuleModel] {
        return try await get("/api/rules")
    }

    func deleteRule(tag: String) async throws {
        let _: [String: String] = try await delete("/api/rules/\(tag)")
    }

    // MARK: - Templates

    func getTemplates() async throws -> [TemplateModel] {
        return try await get("/api/templates")
    }

    func applyTemplate(id: String) async throws {
        let _: [String: String] = try await post("/api/templates/apply", body: ["id": id])
    }

    // MARK: - Subscriptions

    func getSubscriptions() async throws -> [SubscriptionModel] {
        return try await get("/api/subscriptions")
    }

    func updateSubscriptions() async throws {
        let _: [String: String] = try await post("/api/subscriptions/update", body: [String: String]())
    }

    // MARK: - Generic Request Methods

    private func get<T: Decodable>(_ path: String) async throws -> T {
        let url = URL(string: baseURL + path)!
        let (data, _) = try await session.data(from: url)
        let response = try decoder.decode(APIResponse<T>.self, from: data)
        if !response.success {
            throw APIError.serverError(response.error ?? "unknown error")
        }
        guard let result = response.data else {
            throw APIError.noData
        }
        return result
    }

    private func post<T: Decodable, B: Encodable>(_ path: String, body: B) async throws -> T {
        var request = URLRequest(url: URL(string: baseURL + path)!)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(body)

        let (data, _) = try await session.data(for: request)
        let response = try decoder.decode(APIResponse<T>.self, from: data)
        if !response.success {
            throw APIError.serverError(response.error ?? "unknown error")
        }
        guard let result = response.data else {
            throw APIError.noData
        }
        return result
    }

    private func delete<T: Decodable>(_ path: String) async throws -> T {
        var request = URLRequest(url: URL(string: baseURL + path)!)
        request.httpMethod = "DELETE"

        let (data, _) = try await session.data(for: request)
        let response = try decoder.decode(APIResponse<T>.self, from: data)
        if !response.success {
            throw APIError.serverError(response.error ?? "unknown error")
        }
        guard let result = response.data else {
            throw APIError.noData
        }
        return result
    }
}

enum APIError: LocalizedError {
    case serverError(String)
    case noData

    var errorDescription: String? {
        switch self {
        case .serverError(let msg): return msg
        case .noData: return "No data in response"
        }
    }
}
