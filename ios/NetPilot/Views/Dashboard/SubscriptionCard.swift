import SwiftUI

struct SubscriptionCard: View {
    @StateObject private var vm = SettingsViewModel()

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 8) {
                Image(systemName: "doc.text.fill")
                    .foregroundStyle(Theme.accentPurple)
                Text("订阅信息")
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundStyle(Theme.textPrimary)
                Spacer()
                Button("更新") {
                    Task { await vm.updateAllSubscriptions() }
                }
                .font(.system(size: 13, weight: .medium))
                .foregroundStyle(Theme.accentBlue)
            }

            if vm.subscriptions.isEmpty {
                Text("暂无订阅")
                    .font(.system(size: 13))
                    .foregroundStyle(Theme.textTertiary)
            } else {
                ForEach(vm.subscriptions) { sub in
                    HStack {
                        Text(sub.name)
                            .font(.system(size: 15))
                            .foregroundStyle(Theme.textPrimary)
                        Spacer()
                        Text("\(sub.nodeCount) 节点")
                            .font(.system(size: 13, design: .monospaced))
                            .foregroundStyle(Theme.textSecondary)
                    }
                }
            }
        }
        .padding(16)
        .background(Theme.backgroundSecondary)
        .clipShape(RoundedRectangle(cornerRadius: 16))
        .task { await vm.loadSubscriptions() }
    }
}

#Preview {
    SubscriptionCard()
        .padding()
        .background(Theme.backgroundPrimary)
        .preferredColorScheme(.dark)
}
