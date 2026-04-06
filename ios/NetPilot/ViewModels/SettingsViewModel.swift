import Foundation

@MainActor
class SettingsViewModel: ObservableObject {
    @Published var subscriptions: [SubscriptionModel] = []
    @Published var isLoading = false
    @Published var errorMessage: String?

    private let api = APIClient.shared

    func loadSubscriptions() async {
        isLoading = true
        defer { isLoading = false }
        do {
            subscriptions = try await api.getSubscriptions()
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func updateAllSubscriptions() async {
        do {
            try await api.updateSubscriptions()
            await loadSubscriptions()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
