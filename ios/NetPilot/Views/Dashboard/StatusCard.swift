import SwiftUI

struct StatusCard: View {
    let status: StatusModel
    var protocolName: String = ""
    var latency: Int = 0
    var duration: String = "00:00:00"

    private var isConnected: Bool { status.currentNode != "-" && !status.currentNode.isEmpty }

    // 绿色=已连接且延迟<500ms，橙色=已连接但延迟>500ms，红色=未连接
    private var statusColor: Color {
        guard isConnected else { return Theme.accentRed }
        if latency <= 0 || latency >= 500 { return Theme.accentOrange }
        return Theme.accentGreen
    }

    private var statusText: String {
        guard isConnected else { return "未连接" }
        if latency > 0 && latency < 500 { return "已连接" }
        return "已连接"
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            // 标题行：状态圆点 + 文字
            HStack(spacing: 8) {
                Circle()
                    .fill(statusColor)
                    .frame(width: 8, height: 8)
                Text(statusText)
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundStyle(Theme.textPrimary)
                Spacer()
            }

            if isConnected {
                // 当前节点名 + 协议标签 + 延迟
                HStack(spacing: 8) {
                    Text(status.currentNode)
                        .font(.system(size: 15))
                        .foregroundStyle(Theme.textPrimary)

                    if !protocolName.isEmpty {
                        Text(protocolName)
                            .font(.system(size: 11, weight: .medium))
                            .foregroundStyle(Theme.protocolColor(protocolName))
                            .padding(.horizontal, 6)
                            .padding(.vertical, 2)
                            .background(Theme.protocolColor(protocolName).opacity(0.2))
                            .clipShape(RoundedRectangle(cornerRadius: 4))
                    }

                    if latency > 0 {
                        Text("\(latency)ms")
                            .font(.system(size: 15, weight: .medium, design: .monospaced))
                            .foregroundStyle(Theme.latencyColor(latency))
                    }
                }

                // 流量统计 + 连接时长
                HStack(spacing: 16) {
                    Label(status.uploadFormatted, systemImage: "arrow.up")
                        .font(.system(size: 13, design: .monospaced))
                        .foregroundStyle(Theme.textSecondary)
                    Label(status.downloadFormatted, systemImage: "arrow.down")
                        .font(.system(size: 13, design: .monospaced))
                        .foregroundStyle(Theme.textSecondary)
                    Spacer()
                    Label(duration, systemImage: "clock")
                        .font(.system(size: 13, design: .monospaced))
                        .foregroundStyle(Theme.textSecondary)
                }
            }
        }
        .padding(16)
        .background(Theme.backgroundSecondary)
        .clipShape(RoundedRectangle(cornerRadius: 16))
        .animation(.easeInOut(duration: 0.3), value: status.currentNode)
        .animation(.easeInOut(duration: 0.3), value: latency)
    }
}

#Preview {
    StatusCard(status: .empty)
        .padding()
        .background(Theme.backgroundPrimary)
        .preferredColorScheme(.dark)
}
