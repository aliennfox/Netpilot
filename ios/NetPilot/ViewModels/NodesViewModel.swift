import Foundation

struct NodeGroup: Identifiable {
    let id: String  // subscription id or "ungrouped"
    let name: String
    let nodeCount: Int
    var nodes: [NodeModel]
}

@MainActor
class NodesViewModel: ObservableObject {
    @Published var nodes: [NodeModel] = []
    @Published var groups: [NodeGroup] = []
    @Published var isLoading = false
    @Published var isTesting = false
    @Published var switchingTag: String?
    @Published var searchText = ""
    @Published var toastMessage: String?
    @Published var collapsedGroups: Set<String> = []

    private let api = APIClient.shared
    private var subscriptions: [SubscriptionModel] = []

    var filteredGroups: [NodeGroup] {
        if searchText.isEmpty { return groups }
        return groups.compactMap { group in
            let filtered = group.nodes.filter {
                $0.tag.localizedCaseInsensitiveContains(searchText)
            }
            if filtered.isEmpty { return nil }
            return NodeGroup(id: group.id, name: group.name, nodeCount: filtered.count, nodes: filtered)
        }
    }

    func loadNodes() async {
        isLoading = true
        defer { isLoading = false }
        do {
            async let nodesReq = api.getNodes()
            async let subsReq = api.getSubscriptions()
            async let statusReq = api.getStatus()

            let fetchedNodes = try await nodesReq
            subscriptions = try await subsReq
            let status = try await statusReq

            // 设置 active 状态
            nodes = fetchedNodes.map { node in
                var n = node
                n.active = (node.tag == status.currentNode)
                return n
            }

            buildGroups()
        } catch {
            showToast(error.localizedDescription)
        }
    }

    func switchNode(_ tag: String) async {
        switchingTag = tag
        defer { switchingTag = nil }
        do {
            try await api.switchNode(node: tag)
            nodes = nodes.map { node in
                var n = node
                n.active = (node.tag == tag)
                return n
            }
            buildGroups()
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
            let freshNodes = try await api.getNodes()
            let activeTag = nodes.first(where: { $0.active })?.tag
            nodes = freshNodes.map { node in
                var n = node
                n.active = (node.tag == activeTag)
                return n
            }
            // 测速后按延迟排序
            nodes.sort { n1, n2 in
                let l1 = n1.latency > 0 ? n1.latency : Int.max
                let l2 = n2.latency > 0 ? n2.latency : Int.max
                return l1 < l2
            }
            buildGroups()
        } catch {
            showToast("测速失败: \(error.localizedDescription)")
        }
    }

    func toggleGroup(_ groupId: String) {
        if collapsedGroups.contains(groupId) {
            collapsedGroups.remove(groupId)
        } else {
            collapsedGroups.insert(groupId)
        }
    }

    // MARK: - Private

    private func buildGroups() {
        // 用订阅的 tags 字段将节点分组
        var grouped: [NodeGroup] = []
        var assignedTags = Set<String>()

        for sub in subscriptions {
            guard let subTags = sub.tags, !subTags.isEmpty else { continue }
            let subTagSet = Set(subTags)
            let groupNodes = nodes.filter { subTagSet.contains($0.tag) }
            if !groupNodes.isEmpty {
                grouped.append(NodeGroup(
                    id: sub.id,
                    name: sub.name,
                    nodeCount: groupNodes.count,
                    nodes: groupNodes
                ))
                assignedTags.formUnion(subTagSet)
            }
        }

        // 未分组的节点
        let ungrouped = nodes.filter { !assignedTags.contains($0.tag) }
        if !ungrouped.isEmpty {
            grouped.append(NodeGroup(
                id: "ungrouped",
                name: "其他节点",
                nodeCount: ungrouped.count,
                nodes: ungrouped
            ))
        }

        groups = grouped
    }

    private func showToast(_ message: String) {
        toastMessage = message
        Task { [weak self] in
            try? await Task.sleep(for: .seconds(3))
            if self?.toastMessage == message {
                self?.toastMessage = nil
            }
        }
    }
}
