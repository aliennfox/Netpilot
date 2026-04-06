import Foundation

@MainActor
class RulesViewModel: ObservableObject {
    @Published var rules: [RuleModel] = []
    @Published var templates: [TemplateModel] = []
    @Published var isLoading = false
    @Published var applyingTemplateId: String?
    @Published var deletingRuleTag: String?
    @Published var toastMessage: String?
    @Published var toastIsError = true

    private let api = APIClient.shared

    func loadAll() async {
        isLoading = true
        defer { isLoading = false }
        do {
            async let r = api.getRules()
            async let t = api.getTemplates()
            rules = try await r
            templates = try await t
        } catch {
            showToast(error.localizedDescription, isError: true)
        }
    }

    func deleteRule(tag: String) async {
        deletingRuleTag = tag
        defer { deletingRuleTag = nil }
        do {
            try await api.deleteRule(tag: tag)
            rules.removeAll { $0.tag == tag }
            showToast("规则已删除", isError: false)
        } catch {
            showToast("删除失败: \(error.localizedDescription)", isError: true)
        }
    }

    func applyTemplate(id: String) async {
        applyingTemplateId = id
        defer { applyingTemplateId = nil }
        do {
            try await api.applyTemplate(id: id)
            await loadAll()
            showToast("模板已应用", isError: false)
        } catch {
            showToast("应用失败: \(error.localizedDescription)", isError: true)
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
