import SwiftUI

struct RulesView: View {
    @StateObject private var vm = RulesViewModel()

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    // Agent 规则
                    if !vm.rules.isEmpty {
                        SectionHeader(title: "Agent 规则", count: vm.rules.count)
                        VStack(spacing: 1) {
                            ForEach(vm.rules) { rule in
                                RuleRow(rule: rule) {
                                    Task { await vm.deleteRule(tag: rule.tag) }
                                }
                            }
                        }
                        .clipShape(RoundedRectangle(cornerRadius: 12))
                    }

                    // 快捷模板
                    SectionHeader(title: "快捷模板", count: vm.templates.count)
                    VStack(spacing: 1) {
                        ForEach(vm.templates) { tpl in
                            TemplateRow(template: tpl) {
                                Task { await vm.applyTemplate(id: tpl.id) }
                            }
                        }
                    }
                    .clipShape(RoundedRectangle(cornerRadius: 12))
                }
                .padding(16)
            }
            .background(Theme.backgroundPrimary)
            .navigationTitle("规则")
            .navigationBarTitleDisplayMode(.inline)
            .toolbarBackground(Theme.backgroundSecondary, for: .navigationBar)
            .toolbarBackground(.visible, for: .navigationBar)
            .refreshable { await vm.loadAll() }
            .task { await vm.loadAll() }
        }
    }
}

struct SectionHeader: View {
    let title: String
    let count: Int

    var body: some View {
        HStack {
            Text(title)
                .font(.system(size: 15, weight: .semibold))
                .foregroundStyle(Theme.textSecondary)
            Text("(\(count))")
                .font(.system(size: 13))
                .foregroundStyle(Theme.textTertiary)
        }
    }
}

struct RuleRow: View {
    let rule: RuleModel
    let onDelete: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text(rule.tag)
                    .font(.system(size: 15, weight: .medium))
                    .foregroundStyle(Theme.textPrimary)
                Spacer()
                Button("删除", action: onDelete)
                    .font(.system(size: 13))
                    .foregroundStyle(Theme.accentRed)
            }
            Text(rule.description)
                .font(.system(size: 13))
                .foregroundStyle(Theme.textSecondary)
            Text("\(rule.summary) → \(rule.outbound)")
                .font(.system(size: 12))
                .foregroundStyle(Theme.textTertiary)
        }
        .padding(12)
        .background(Theme.backgroundSecondary)
    }
}

#Preview {
    RulesView()
        .preferredColorScheme(.dark)
}
