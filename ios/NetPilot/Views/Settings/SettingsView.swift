import SwiftUI

struct SettingsView: View {
    @StateObject private var vm = SettingsViewModel()

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    // 订阅管理
                    SectionHeader(title: "订阅管理", count: vm.subscriptions.count)
                    VStack(spacing: 1) {
                        ForEach(vm.subscriptions) { sub in
                            HStack {
                                VStack(alignment: .leading, spacing: 4) {
                                    Text(sub.name)
                                        .font(.system(size: 15))
                                        .foregroundStyle(Theme.textPrimary)
                                    Text("\(sub.nodeCount) 节点")
                                        .font(.system(size: 13))
                                        .foregroundStyle(Theme.textSecondary)
                                }
                                Spacer()
                                Button(action: {
                                    Task { await vm.updateAllSubscriptions() }
                                }) {
                                    Image(systemName: "arrow.clockwise")
                                        .foregroundStyle(Theme.accentBlue)
                                }
                            }
                            .padding(12)
                            .background(Theme.backgroundSecondary)
                        }

                        if vm.subscriptions.isEmpty {
                            Text("暂无订阅")
                                .font(.system(size: 13))
                                .foregroundStyle(Theme.textTertiary)
                                .frame(maxWidth: .infinity)
                                .padding(12)
                                .background(Theme.backgroundSecondary)
                        }
                    }
                    .clipShape(RoundedRectangle(cornerRadius: 12))

                    // 关于
                    SectionHeader(title: "关于", count: 0)
                    VStack(spacing: 1) {
                        SettingsRow(title: "版本", value: "v0.1.0")
                        SettingsRow(title: "sing-box", value: "v1.13.5")
                    }
                    .clipShape(RoundedRectangle(cornerRadius: 12))
                }
                .padding(16)
            }
            .background(Theme.backgroundPrimary)
            .navigationTitle("设置")
            .navigationBarTitleDisplayMode(.inline)
            .toolbarBackground(Theme.backgroundSecondary, for: .navigationBar)
            .toolbarBackground(.visible, for: .navigationBar)
            .task { await vm.loadSubscriptions() }
        }
    }
}

struct SettingsRow: View {
    let title: String
    let value: String

    var body: some View {
        HStack {
            Text(title)
                .font(.system(size: 15))
                .foregroundStyle(Theme.textPrimary)
            Spacer()
            Text(value)
                .font(.system(size: 15))
                .foregroundStyle(Theme.textSecondary)
        }
        .padding(12)
        .background(Theme.backgroundSecondary)
    }
}

#Preview {
    SettingsView()
        .preferredColorScheme(.dark)
}
