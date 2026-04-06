import SwiftUI

struct NodesView: View {
    @StateObject private var vm = NodesViewModel()

    var body: some View {
        NavigationStack {
            ZStack(alignment: .bottom) {
                VStack(spacing: 0) {
                    // 搜索栏
                    HStack {
                        Image(systemName: "magnifyingglass")
                            .foregroundStyle(Theme.textTertiary)
                        TextField("搜索节点...", text: $vm.searchText)
                            .font(.system(size: 15))
                            .foregroundStyle(Theme.textPrimary)
                        if !vm.searchText.isEmpty {
                            Button {
                                vm.searchText = ""
                            } label: {
                                Image(systemName: "xmark.circle.fill")
                                    .foregroundStyle(Theme.textTertiary)
                            }
                        }
                    }
                    .padding(10)
                    .background(Theme.backgroundTertiary)
                    .clipShape(RoundedRectangle(cornerRadius: 12))
                    .padding(.horizontal, 16)
                    .padding(.vertical, 8)

                    // 节点列表
                    ScrollView {
                        LazyVStack(spacing: 1) {
                            ForEach(vm.filteredNodes) { node in
                                NodeRow(
                                    node: node,
                                    isSwitching: vm.switchingTag == node.tag,
                                    isTesting: vm.isTesting
                                ) {
                                    Task { await vm.switchNode(node.tag) }
                                }
                            }
                        }
                        .padding(.horizontal, 16)
                    }
                }

                // Toast
                if let toast = vm.toastMessage {
                    Text(toast)
                        .font(.system(size: 14))
                        .foregroundStyle(.white)
                        .padding(.horizontal, 16)
                        .padding(.vertical, 10)
                        .background(Theme.accentRed.opacity(0.9))
                        .clipShape(Capsule())
                        .padding(.bottom, 20)
                        .transition(.move(edge: .bottom).combined(with: .opacity))
                        .animation(.easeInOut, value: vm.toastMessage)
                }
            }
            .background(Theme.backgroundPrimary)
            .navigationTitle("节点")
            .navigationBarTitleDisplayMode(.inline)
            .toolbarBackground(Theme.backgroundSecondary, for: .navigationBar)
            .toolbarBackground(.visible, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button(action: {
                        Task { await vm.testAllLatency() }
                    }) {
                        if vm.isTesting {
                            ProgressView()
                                .tint(Theme.textSecondary)
                        } else {
                            Text("测速")
                                .font(.system(size: 15, weight: .medium))
                                .foregroundStyle(Theme.accentBlue)
                        }
                    }
                    .disabled(vm.isTesting)
                }
            }
            .refreshable { await vm.loadNodes() }
            .task {
                await vm.loadNodes()
                await vm.testAllLatency()
            }
        }
    }
}

#Preview {
    NodesView()
        .preferredColorScheme(.dark)
}
