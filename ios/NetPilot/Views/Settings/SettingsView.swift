import SwiftUI

struct SettingsView: View {
    @StateObject private var vm = SettingsViewModel()

    private let dateFormatter: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "MM/dd HH:mm"
        return f
    }()

    var body: some View {
        NavigationStack {
            ZStack(alignment: .bottom) {
                ScrollView {
                    VStack(alignment: .leading, spacing: 16) {
                        // 订阅管理
                        HStack {
                            SectionHeader(title: "订阅管理", count: vm.subscriptions.count)
                            Spacer()
                            if vm.isUpdating {
                                ProgressView()
                                    .scaleEffect(0.8)
                                    .tint(Theme.textSecondary)
                            } else {
                                Button {
                                    Task { await vm.updateAllSubscriptions() }
                                } label: {
                                    Image(systemName: "arrow.clockwise")
                                        .font(.system(size: 14))
                                        .foregroundStyle(Theme.accentBlue)
                                }
                            }
                        }

                        VStack(spacing: 1) {
                            ForEach(vm.subscriptions) { sub in
                                SubscriptionRow(
                                    subscription: sub,
                                    dateFormatter: dateFormatter,
                                    onDelete: {
                                        Task { await vm.deleteSubscription(sub.id) }
                                    }
                                )
                            }

                            if vm.subscriptions.isEmpty {
                                Text("暂无订阅，点击下方按钮添加")
                                    .font(.system(size: 13))
                                    .foregroundStyle(Theme.textTertiary)
                                    .frame(maxWidth: .infinity)
                                    .padding(12)
                                    .background(Theme.backgroundSecondary)
                            }

                            // 添加订阅按钮
                            Button {
                                vm.showAddSheet = true
                            } label: {
                                HStack {
                                    Image(systemName: "plus.circle.fill")
                                        .foregroundStyle(Theme.accentBlue)
                                    Text("添加订阅")
                                        .font(.system(size: 15))
                                        .foregroundStyle(Theme.accentBlue)
                                    Spacer()
                                }
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
            .navigationTitle("设置")
            .navigationBarTitleDisplayMode(.inline)
            .toolbarBackground(Theme.backgroundSecondary, for: .navigationBar)
            .toolbarBackground(.visible, for: .navigationBar)
            .task { await vm.loadSubscriptions() }
            .sheet(isPresented: $vm.showAddSheet) {
                AddSubscriptionSheet(vm: vm)
            }
        }
    }
}

// MARK: - 订阅行

struct SubscriptionRow: View {
    let subscription: SubscriptionModel
    let dateFormatter: DateFormatter
    let onDelete: () -> Void

    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: 4) {
                Text(subscription.name)
                    .font(.system(size: 15))
                    .foregroundStyle(Theme.textPrimary)
                HStack(spacing: 8) {
                    Text("\(subscription.nodeCount) 节点")
                    if let lastUpdate = subscription.lastUpdate {
                        Text("更新: \(dateFormatter.string(from: lastUpdate))")
                    }
                }
                .font(.system(size: 12))
                .foregroundStyle(Theme.textTertiary)
            }
            Spacer()
            Button(action: onDelete) {
                Image(systemName: "trash")
                    .font(.system(size: 14))
                    .foregroundStyle(Theme.accentRed)
            }
        }
        .padding(12)
        .background(Theme.backgroundSecondary)
    }
}

// MARK: - 添加订阅 Sheet

struct AddSubscriptionSheet: View {
    @ObservedObject var vm: SettingsViewModel
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            VStack(spacing: 20) {
                VStack(alignment: .leading, spacing: 8) {
                    Text("订阅链接")
                        .font(.system(size: 13, weight: .medium))
                        .foregroundStyle(Theme.textSecondary)
                    TextField("https://example.com/sub", text: $vm.newSubURL)
                        .font(.system(size: 15))
                        .padding(12)
                        .background(Theme.backgroundTertiary)
                        .clipShape(RoundedRectangle(cornerRadius: 10))
                        .foregroundStyle(Theme.textPrimary)
                        .autocorrectionDisabled()
                        .textInputAutocapitalization(.never)
                }

                VStack(alignment: .leading, spacing: 8) {
                    Text("名称（可选）")
                        .font(.system(size: 13, weight: .medium))
                        .foregroundStyle(Theme.textSecondary)
                    TextField("我的订阅", text: $vm.newSubName)
                        .font(.system(size: 15))
                        .padding(12)
                        .background(Theme.backgroundTertiary)
                        .clipShape(RoundedRectangle(cornerRadius: 10))
                        .foregroundStyle(Theme.textPrimary)
                }

                Button {
                    Task { await vm.addSubscription() }
                } label: {
                    if vm.isAdding {
                        ProgressView()
                            .tint(.white)
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 14)
                    } else {
                        Text("导入")
                            .font(.system(size: 17, weight: .semibold))
                            .foregroundStyle(.white)
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 14)
                    }
                }
                .background(
                    vm.newSubURL.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
                        ? Theme.textTertiary
                        : Theme.accentBlue
                )
                .clipShape(RoundedRectangle(cornerRadius: 12))
                .disabled(
                    vm.newSubURL.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || vm.isAdding
                )

                Spacer()
            }
            .padding(20)
            .background(Theme.backgroundPrimary)
            .navigationTitle("添加订阅")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("取消") { dismiss() }
                        .foregroundStyle(Theme.textSecondary)
                }
            }
        }
        .presentationDetents([.medium])
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
