import SwiftUI

struct StatusCard: View {
    let status: StatusModel

    private var isConnected: Bool { status.currentNode != "-" && status.currentNode != "" }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            // 标题行
            HStack(spacing: 8) {
                Circle()
                    .fill(isConnected ? Theme.accentGreen : Theme.accentRed)
                    .frame(width: 8, height: 8)
                Text(isConnected ? "已连接" : "未连接")
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundStyle(Theme.textPrimary)
                Spacer()
            }

            if isConnected {
                // 当前节点
                Text(status.currentNode)
                    .font(.system(size: 15))
                    .foregroundStyle(Theme.textPrimary)

                // 流量统计
                HStack(spacing: 16) {
                    Label(status.uploadFormatted, systemImage: "arrow.up")
                        .font(.system(size: 13, design: .monospaced))
                        .foregroundStyle(Theme.textSecondary)
                    Label(status.downloadFormatted, systemImage: "arrow.down")
                        .font(.system(size: 13, design: .monospaced))
                        .foregroundStyle(Theme.textSecondary)
                }
            }
        }
        .padding(16)
        .background(Theme.backgroundSecondary)
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }
}

#Preview {
    StatusCard(status: .empty)
        .padding()
        .background(Theme.backgroundPrimary)
        .preferredColorScheme(.dark)
}
