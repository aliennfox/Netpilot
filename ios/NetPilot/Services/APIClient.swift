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
    private let chatSession: URLSession  // Agent/LLM 请求用长超时
    private let decoder: JSONDecoder

    // Bearer token 认证（开发模式下可为空）
    var authToken: String? {
        UserDefaults.standard.string(forKey: "netpilot_api_token")
    }

    private init() {
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 15
        session = URLSession(configuration: config)

        let chatConfig = URLSessionConfiguration.default
        chatConfig.timeoutIntervalForRequest = 120
        chatSession = URLSession(configuration: chatConfig)

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
        return try await post("/api/chat", body: ["message": message], using: chatSession)
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

    func addSubscription(url: String, name: String) async throws {
        let _: [String: String] = try await post("/api/subscriptions", body: [
            "url": url,
            "name": name,
        ])
    }

    func deleteSubscription(id: String) async throws {
        let _: [String: String] = try await delete("/api/subscriptions/\(id)")
    }

    func updateSubscriptions() async throws {
        let _: [String: String] = try await post("/api/subscriptions/update", body: [String: String]())
    }

    // MARK: - Generic Request Methods

    private func get<T: Decodable>(_ path: String) async throws -> T {
        guard let url = URL(string: baseURL + path) else {
            throw APIError.invalidURL(path)
        }
        var request = URLRequest(url: url)
        applyAuth(&request)

        let (data, response) = try await session.data(for: request)
        try validateHTTPResponse(response)
        return try decodeResponse(data)
    }

    private func post<T: Decodable, B: Encodable>(_ path: String, body: B, using overrideSession: URLSession? = nil) async throws -> T {
        guard let url = URL(string: baseURL + path) else {
            throw APIError.invalidURL(path)
        }
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(body)
        applyAuth(&request)

        let (data, response) = try await (overrideSession ?? session).data(for: request)
        try validateHTTPResponse(response)
        return try decodeResponse(data)
    }

    private func delete<T: Decodable>(_ path: String) async throws -> T {
        guard let url = URL(string: baseURL + path) else {
            throw APIError.invalidURL(path)
        }
        var request = URLRequest(url: url)
        request.httpMethod = "DELETE"
        applyAuth(&request)

        let (data, response) = try await session.data(for: request)
        try validateHTTPResponse(response)
        return try decodeResponse(data)
    }

    // MARK: - Helpers

    private func applyAuth(_ request: inout URLRequest) {
        if let token = authToken, !token.isEmpty {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
    }

    private func validateHTTPResponse(_ response: URLResponse) throws {
        guard let httpResponse = response as? HTTPURLResponse else {
            throw APIError.invalidResponse
        }
        guard (200..<300).contains(httpResponse.statusCode) else {
            throw APIError.httpError(httpResponse.statusCode)
        }
    }

    private func decodeResponse<T: Decodable>(_ data: Data) throws -> T {
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
    case invalidURL(String)
    case invalidResponse
    case httpError(Int)

    var errorDescription: String? {
        switch self {
        case .serverError(let msg): return msg
        case .noData: return "No data in response"
        case .invalidURL(let path): return "Invalid URL: \(path)"
        case .invalidResponse: return "Invalid server response"
        case .httpError(let code): return "HTTP error: \(code)"
        }
    }
}
