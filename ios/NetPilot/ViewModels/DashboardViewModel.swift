import Foundation
import SwiftUI

@MainActor
class DashboardViewModel: ObservableObject {
    @Published var status = StatusModel.empty
    @Published var isLoading = false
    @Published var errorMessage: String?

    // 当前节点详情（从 nodes API 获取）
    @Published var currentNodeProtocol: String = ""
    @Published var currentNodeLatency: Int = 0

    // 实时速率（通过前后两次 status 差值计算）
    @Published var uploadSpeed: Int64 = 0
    @Published var downloadSpeed: Int64 = 0

    // 连接时长追踪
    @Published var connectionDuration: TimeInterval = 0
    private var connectionStartDate: Date?
    private var durationTimer: Timer?

    private let api = APIClient.shared
    private var refreshTask: Task<Void, Never>?
    private var previousUpload: Int64 = 0
    private var previousDownload: Int64 = 0
    private var lastFetchTime: Date?

    func loadStatus() async {
        isLoading = true
        defer { isLoading = false }
        do {
            let newStatus = try await api.getStatus()

            // 计算速率
            let now = Date()
            if let lastTime = lastFetchTime {
                let interval = now.timeIntervalSince(lastTime)
                if interval > 0 {
                    uploadSpeed = max(0, Int64(Double(newStatus.upload - previousUpload) / interval))
                    downloadSpeed = max(0, Int64(Double(newStatus.download - previousDownload) / interval))
                }
            }
            previousUpload = newStatus.upload
            previousDownload = newStatus.download
            lastFetchTime = now

            // 连接时长
            let isConnected = newStatus.currentNode != "-" && !newStatus.currentNode.isEmpty
            if isConnected && connectionStartDate == nil {
                connectionStartDate = now
                startDurationTimer()
            } else if !isConnected {
                connectionStartDate = nil
                connectionDuration = 0
                stopDurationTimer()
            }

            status = newStatus
            errorMessage = nil

            // 获取当前节点协议和延迟
            if isConnected {
                let nodes = try await api.getNodes()
                if let current = nodes.first(where: { $0.tag == newStatus.currentNode }) {
                    currentNodeProtocol = current.protocolShort
                    currentNodeLatency = current.latency
                }
            } else {
                currentNodeProtocol = ""
                currentNodeLatency = 0
            }
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func setMode(_ mode: String) async {
        do {
            try await api.setMode(mode)
            await loadStatus()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func startAutoRefresh() {
        refreshTask = Task {
            while !Task.isCancelled {
                await loadStatus()
                try? await Task.sleep(for: .seconds(5))
            }
        }
    }

    func stopAutoRefresh() {
        refreshTask?.cancel()
        refreshTask = nil
    }

    // MARK: - Duration Timer

    private func startDurationTimer() {
        stopDurationTimer()
        durationTimer = Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { [weak self] _ in
            Task { @MainActor [weak self] in
                guard let self, let start = self.connectionStartDate else { return }
                self.connectionDuration = Date().timeIntervalSince(start)
            }
        }
    }

    private func stopDurationTimer() {
        durationTimer?.invalidate()
        durationTimer = nil
    }

    var durationFormatted: String {
        let total = Int(connectionDuration)
        let h = total / 3600
        let m = (total % 3600) / 60
        let s = total % 60
        return String(format: "%02d:%02d:%02d", h, m, s)
    }

    var uploadSpeedFormatted: String { formatSpeed(uploadSpeed) }
    var downloadSpeedFormatted: String { formatSpeed(downloadSpeed) }

    private func formatSpeed(_ bytesPerSec: Int64) -> String {
        let kb = Double(bytesPerSec) / 1024
        let mb = kb / 1024
        if mb >= 1 { return String(format: "%.1f MB/s", mb) }
        if kb >= 1 { return String(format: "%.1f KB/s", kb) }
        return "\(bytesPerSec) B/s"
    }
}
