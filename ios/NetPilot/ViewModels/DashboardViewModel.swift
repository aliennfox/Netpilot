import Foundation

@MainActor
class DashboardViewModel: ObservableObject {
    @Published var status = StatusModel.empty
    @Published var isLoading = false
    @Published var errorMessage: String?

    private let api = APIClient.shared
    private var refreshTask: Task<Void, Never>?

    func loadStatus() async {
        isLoading = true
        defer { isLoading = false }
        do {
            status = try await api.getStatus()
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func setMode(_ mode: String) async {
        do {
            try await api.setMode(mode)
            await loadStatus()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func startAutoRefresh() {
        refreshTask = Task {
            while !Task.isCancelled {
                await loadStatus()
                try? await Task.sleep(for: .seconds(5))
            }
        }
    }

    func stopAutoRefresh() {
        refreshTask?.cancel()
        refreshTask = nil
    }
}
