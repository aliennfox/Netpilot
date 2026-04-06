import SwiftUI

struct RulesView: View {
    @StateObject private var vm = RulesViewModel()

    var body: some View {
        NavigationStack {
            ZStack(alignment: .bottom) {
                ScrollView {
                    VStack(alignment: .leading, spacing: 16) {
                        // Agent 规则
                        if !vm.rules.isEmpty {
                            SectionHeader(title: "Agent 规则", count: vm.rules.count)
                            VStack(spacing: 1) {
                                ForEach(vm.rules) { rule in
                                    RuleRow(
                                        rule: rule,
                                        isDeleting: vm.deletingRuleTag == rule.tag
                                    ) {
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
                                TemplateRow(
                                    template: tpl,
                                    isApplying: vm.applyingTemplateId == tpl.id
                                ) {
                                    Task { await vm.applyTemplate(id: tpl.id) }
                                }
                            }
                        }
                        .clipShape(RoundedRectangle(cornerRadius: 12))
                    }
                    .padding(16)
                }

                // Toast
                if let toast = vm.toastMessage {
                    Text(toast)
                        .font(.system(size: 14))
                        .foregroundStyle(.white)
                        .padding(.horizontal, 16)
                        .padding(.vertical, 10)
                        .background(
                            (vm.toastIsError ? Theme.accentRed : Theme.accentGreen).opacity(0.9)
                        )
                        .clipShape(Capsule())
                        .padding(.bottom, 20)
                        .transition(.move(edge: .bottom).combined(with: .opacity))
                        .animation(.easeInOut, value: vm.toastMessage)
                }
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
            if count > 0 {
                Text("(\(count))")
                    .font(.system(size: 13))
                    .foregroundStyle(Theme.textTertiary)
            }
        }
    }
}

struct RuleRow: View {
    let rule: RuleModel
    var isDeleting: Bool = false
    let onDelete: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text(rule.tag)
                    .font(.system(size: 15, weight: .medium))
                    .foregroundStyle(Theme.textPrimary)
                Spacer()
                if isDeleting {
                    ProgressView()
                        .scaleEffect(0.7)
                        .tint(Theme.textTertiary)
                } else {
                    Button("删除", action: onDelete)
                        .font(.system(size: 13))
                        .foregroundStyle(Theme.accentRed)
                }
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
