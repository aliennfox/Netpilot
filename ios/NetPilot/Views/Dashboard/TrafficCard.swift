import SwiftUI

struct TrafficCard: View {
    let status: StatusModel
    var uploadSpeed: String = "0 B/s"
    var downloadSpeed: String = "0 B/s"

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 8) {
                Image(systemName: "chart.bar.fill")
                    .foregroundStyle(Theme.accentTeal)
                Text("实时流量")
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundStyle(Theme.textPrimary)
            }

            HStack(spacing: 0) {
                // 活跃连接
                VStack(alignment: .leading, spacing: 4) {
                    Text("活跃连接")
                        .font(.system(size: 13))
                        .foregroundStyle(Theme.textSecondary)
                    Text("\(status.connections)")
                        .font(.system(size: 20, weight: .semibold, design: .monospaced))
                        .foregroundStyle(Theme.textPrimary)
                }
                .frame(maxWidth: .infinity, alignment: .leading)

                // 节点数
                VStack(alignment: .leading, spacing: 4) {
                    Text("节点数")
                        .font(.system(size: 13))
                        .foregroundStyle(Theme.textSecondary)
                    Text("\(status.nodeCount)")
                        .font(.system(size: 20, weight: .semibold, design: .monospaced))
                        .foregroundStyle(Theme.textPrimary)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }

            // 上传/下载速率
            HStack(spacing: 0) {
                HStack(spacing: 4) {
                    Image(systemName: "arrow.up")
                        .font(.system(size: 11))
                        .foregroundStyle(Theme.accentGreen)
                    Text(uploadSpeed)
                        .font(.system(size: 13, weight: .medium, design: .monospaced))
                        .foregroundStyle(Theme.textSecondary)
                }
                .frame(maxWidth: .infinity, alignment: .leading)

                HStack(spacing: 4) {
                    Image(systemName: "arrow.down")
                        .font(.system(size: 11))
                        .foregroundStyle(Theme.accentBlue)
                    Text(downloadSpeed)
                        .font(.system(size: 13, weight: .medium, design: .monospaced))
                        .foregroundStyle(Theme.textSecondary)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .padding(16)
        .background(Theme.backgroundSecondary)
        .clipShape(RoundedRectangle(cornerRadius: 16))
        .animation(.easeInOut(duration: 0.3), value: status.connections)
    }
}

#Preview {
    TrafficCard(status: .empty, uploadSpeed: "1.2 MB/s", downloadSpeed: "5.6 MB/s")
        .padding()
        .background(Theme.backgroundPrimary)
        .preferredColorScheme(.dark)
}
