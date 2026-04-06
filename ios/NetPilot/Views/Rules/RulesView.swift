import SwiftUI

struct RulesView: View {
    @StateObject private var vm = RulesViewModel()
    @State private var ruleToDelete: RuleModel?
    @State private var templateToApply: TemplateModel?

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
                                        ruleToDelete = rule
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
                                    templateToApply = tpl
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
            // 删除确认弹窗
            .alert("删除规则", isPresented: Binding(
                get: { ruleToDelete != nil },
                set: { if !$0 { ruleToDelete = nil } }
            )) {
                Button("取消", role: .cancel) { ruleToDelete = nil }
                Button("删除", role: .destructive) {
                    if let rule = ruleToDelete {
                        Task { await vm.deleteRule(tag: rule.tag) }
                        ruleToDelete = nil
                    }
                }
            } message: {
                if let rule = ruleToDelete {
                    Text("确定要删除规则「\(rule.tag)」吗？此操作不可撤销。")
                }
            }
            // 应用模板确认弹窗
            .alert("应用模板", isPresented: Binding(
                get: { templateToApply != nil },
                set: { if !$0 { templateToApply = nil } }
            )) {
                Button("取消", role: .cancel) { templateToApply = nil }
                Button("应用") {
                    if let tpl = templateToApply {
                        Task { await vm.applyTemplate(id: tpl.id) }
                        templateToApply = nil
                    }
                }
            } message: {
                if let tpl = templateToApply {
                    Text("确定要应用模板「\(tpl.name)」吗？将添加对应的路由规则。")
                }
            }
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
            HStack(spacing: 8) {
                if rule.domainCount > 0 {
                    Label("\(rule.domainCount) 域名", systemImage: "globe")
                        .font(.system(size: 12))
                        .foregroundStyle(Theme.textTertiary)
                }
                Image(systemName: "arrow.right")
                    .font(.system(size: 10))
                    .foregroundStyle(Theme.textTertiary)
                Text(rule.outbound)
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(Theme.accentBlue)
            }
        }
        .padding(12)
        .background(Theme.backgroundSecondary)
    }
}

#Preview {
    RulesView()
        .preferredColorScheme(.dark)
}
