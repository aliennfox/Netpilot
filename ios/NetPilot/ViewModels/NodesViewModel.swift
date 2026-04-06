import Foundation

@MainActor
class NodesViewModel: ObservableObject {
    @Published var nodes: [NodeModel] = []
    @Published var isLoading = false
    @Published var isTesting = false
    @Published var switchingTag: String?  // 正在切换的节点 tag
    @Published var searchText = ""
    @Published var toastMessage: String?

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
            // 用 status 同步当前活跃节点
            let status = try await api.getStatus()
            if !status.currentNode.isEmpty {
                nodes = nodes.map { node in
                    var n = node
                    n.active = (node.tag == status.currentNode)
                    return n
                }
            }
        } catch {
            showToast(error.localizedDescription)
        }
    }

    func switchNode(_ tag: String) async {
        switchingTag = tag
        defer { switchingTag = nil }
        do {
            try await api.switchNode(node: tag)
            // 本地更新 active 状态，避免重新拉取
            nodes = nodes.map { node in
                var n = node
                n.active = (node.tag == tag)
                return n
            }
        } catch {
            showToast("切换失败: \(error.localizedDescription)")
        }
    }

    func testAllLatency() async {
        isTesting = true
        defer { isTesting = false }
        do {
            try await api.testLatencyAll()
            try? await Task.sleep(for: .seconds(1))
            // 只刷新节点数据，保留 active 状态
            let freshNodes = try await api.getNodes()
            let activeTag = nodes.first(where: { $0.active })?.tag
            nodes = freshNodes.map { node in
                var n = node
                n.active = (node.tag == activeTag)
                return n
            }
        } catch {
            showToast("测速失败: \(error.localizedDescription)")
        }
    }

    private func showToast(_ message: String) {
        toastMessage = message
        Task {
            try? await Task.sleep(for: .seconds(3))
            if toastMessage == message {
                toastMessage = nil
            }
        }
    }
}
