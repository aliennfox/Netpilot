import SwiftUI

struct DashboardView: View {
    @StateObject private var vm = DashboardViewModel()

    var body: some View {
        ScrollView {
            VStack(spacing: 12) {
                StatusCard(status: vm.status)
                ProxyModeCard(
                    currentMode: vm.status.mode,
                    onModeChange: { mode in Task { await vm.setMode(mode) } }
                )
                TrafficCard(status: vm.status)
                SubscriptionCard()
            }
            .padding(16)
        }
        .background(Theme.backgroundPrimary)
        .refreshable { await vm.loadStatus() }
        .task {
            await vm.loadStatus()
            vm.startAutoRefresh()
        }
        .onDisappear { vm.stopAutoRefresh() }
    }
}

#Preview {
    DashboardView()
        .preferredColorScheme(.dark)
}
