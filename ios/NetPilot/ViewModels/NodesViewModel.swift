import Foundation

@MainActor
class NodesViewModel: ObservableObject {
    @Published var nodes: [NodeModel] = []
    @Published var isLoading = false
    @Published var isTesting = false
    @Published var errorMessage: String?
    @Published var searchText = ""

    private let api = APIClient.shared

    var filteredNodes: [NodeModel] {
        if searchText.isEmpty { return nodes }
        return nodes.filter { $0.tag.localizedCaseInsensitiveContains(searchText) }
    }

    func loadNodes() async {
        isLoading = true
        defer { isLoading = false }
        do {
            nodes = try await api.getNodes()
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func switchNode(_ tag: String) async {
        do {
            try await api.switchNode(node: tag)
            await loadNodes()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func testAllLatency() async {
        isTesting = true
        defer { isTesting = false }
        do {
            try await api.testLatencyAll()
            // 延迟后刷新节点列表以获取新延迟
            try? await Task.sleep(for: .seconds(1))
            await loadNodes()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
