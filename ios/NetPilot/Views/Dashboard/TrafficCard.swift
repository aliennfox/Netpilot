import SwiftUI

struct TrafficCard: View {
    let status: StatusModel

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 8) {
                Image(systemName: "chart.bar.fill")
                    .foregroundStyle(Theme.accentTeal)
                Text("实时流量")
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundStyle(Theme.textPrimary)
            }

            HStack(spacing: 24) {
                VStack(alignment: .leading, spacing: 4) {
                    Text("活跃连接")
                        .font(.system(size: 13))
                        .foregroundStyle(Theme.textSecondary)
                    Text("\(status.connections)")
                        .font(.system(size: 20, weight: .semibold, design: .monospaced))
                        .foregroundStyle(Theme.textPrimary)
                }
                VStack(alignment: .leading, spacing: 4) {
                    Text("节点数")
                        .font(.system(size: 13))
                        .foregroundStyle(Theme.textSecondary)
                    Text("\(status.nodeCount)")
                        .font(.system(size: 20, weight: .semibold, design: .monospaced))
                        .foregroundStyle(Theme.textPrimary)
                }
                Spacer()
            }
        }
        .padding(16)
        .background(Theme.backgroundSecondary)
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }
}

#Preview {
    TrafficCard(status: .empty)
        .padding()
        .background(Theme.backgroundPrimary)
        .preferredColorScheme(.dark)
}
