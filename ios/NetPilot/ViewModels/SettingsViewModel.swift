import Foundation

@MainActor
class SettingsViewModel: ObservableObject {
    @Published var subscriptions: [SubscriptionModel] = []
    @Published var isLoading = false
    @Published var isUpdating = false
    @Published var isAdding = false
    @Published var showAddSheet = false
    @Published var newSubURL = ""
    @Published var newSubName = ""
    @Published var toastMessage: String?
    @Published var toastIsError = true

    private let api = APIClient.shared

    func loadSubscriptions() async {
        isLoading = true
        defer { isLoading = false }
        do {
            subscriptions = try await api.getSubscriptions()
        } catch {
            showToast(error.localizedDescription, isError: true)
        }
    }

    func updateAllSubscriptions() async {
        isUpdating = true
        defer { isUpdating = false }
        do {
            try await api.updateSubscriptions()
            await loadSubscriptions()
            showToast("订阅已更新", isError: false)
        } catch {
            showToast("更新失败: \(error.localizedDescription)", isError: true)
        }
    }

    func addSubscription() async {
        let url = newSubURL.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !url.isEmpty else { return }

        isAdding = true
        defer { isAdding = false }
        do {
            try await api.addSubscription(url: url, name: newSubName.trimmingCharacters(in: .whitespacesAndNewlines))
            newSubURL = ""
            newSubName = ""
            showAddSheet = false
            await loadSubscriptions()
            showToast("订阅导入成功", isError: false)
        } catch {
            showToast("导入失败: \(error.localizedDescription)", isError: true)
        }
    }

    func deleteSubscription(_ id: String) async {
        do {
            try await api.deleteSubscription(id: id)
            subscriptions.removeAll { $0.id == id }
        } catch {
            showToast("删除失败: \(error.localizedDescription)", isError: true)
        }
    }

    private func showToast(_ message: String, isError: Bool) {
        toastMessage = message
        toastIsError = isError
        Task { [weak self] in
            try? await Task.sleep(for: .seconds(3))
            if self?.toastMessage == message {
                self?.toastMessage = nil
            }
        }
    }
}
