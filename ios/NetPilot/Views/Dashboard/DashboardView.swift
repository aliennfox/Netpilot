import SwiftUI

struct DashboardView: View {
    @StateObject private var vm = DashboardViewModel()
    @Environment(\.scenePhase) private var scenePhase

    var body: some View {
        ScrollView {
            VStack(spacing: 12) {
                StatusCard(
                    status: vm.status,
                    protocolName: vm.currentNodeProtocol,
                    latency: vm.currentNodeLatency,
                    duration: vm.durationFormatted
                )
                ProxyModeCard(
                    currentMode: vm.status.mode,
                    onModeChange: { mode in Task { await vm.setMode(mode) } }
                )
                TrafficCard(
                    status: vm.status,
                    uploadSpeed: vm.uploadSpeedFormatted,
                    downloadSpeed: vm.downloadSpeedFormatted
                )
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
        .onChange(of: scenePhase) { _, newPhase in
            switch newPhase {
            case .active:
                vm.startAutoRefresh()
            case .inactive, .background:
                vm.stopAutoRefresh()
            @unknown default:
                break
            }
        }
    }
}

#Preview {
    DashboardView()
        .preferredColorScheme(.dark)
}
