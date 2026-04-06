import Foundation

@MainActor
class RulesViewModel: ObservableObject {
    @Published var rules: [RuleModel] = []
    @Published var templates: [TemplateModel] = []
    @Published var isLoading = false
    @Published var errorMessage: String?

    private let api = APIClient.shared

    func loadAll() async {
        isLoading = true
        defer { isLoading = false }
        do {
            async let r = api.getRules()
            async let t = api.getTemplates()
            rules = try await r
            templates = try await t
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func deleteRule(tag: String) async {
        do {
            try await api.deleteRule(tag: tag)
            await loadAll()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func applyTemplate(id: String) async {
        do {
            try await api.applyTemplate(id: id)
            await loadAll()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
