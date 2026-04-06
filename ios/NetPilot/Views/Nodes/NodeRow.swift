import SwiftUI

struct NodeRow: View {
    let node: NodeModel
    let onTap: () -> Void

    var body: some View {
        Button(action: onTap) {
            HStack(spacing: 12) {
                // 活跃指示器
                Circle()
                    .fill(node.active ? Theme.accentBlue : .clear)
                    .frame(width: 6, height: 6)

                // 节点名
                Text(node.tag)
                    .font(.system(size: 15))
                    .foregroundStyle(Theme.textPrimary)
                    .lineLimit(1)

                // 协议标签
                Text(node.protocolShort)
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(Theme.protocolColor(node.type))
                    .padding(.horizontal, 6)
                    .padding(.vertical, 2)
                    .background(Theme.protocolColor(node.type).opacity(0.2))
                    .clipShape(RoundedRectangle(cornerRadius: 4))

                Spacer()

                // 延迟
                Text(node.latencyText)
                    .font(.system(size: 15, weight: .medium, design: .monospaced))
                    .foregroundStyle(
                        node.latency > 0
                            ? Theme.latencyColor(node.latency)
                            : Theme.textTertiary
                    )
            }
            .padding(.vertical, 12)
            .padding(.horizontal, 12)
            .background(Theme.backgroundSecondary)
        }
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }
}

#Preview {
    VStack(spacing: 1) {
        NodeRow(node: NodeModel(
            tag: "美国A", type: "hysteria2", server: "1.2.3.4", port: 443,
            alive: true, latency: 430, groupTag: "proxy-group", active: true
        ), onTap: {})
        NodeRow(node: NodeModel(
            tag: "日本-1", type: "shadowsocks", server: "5.6.7.8", port: 443,
            alive: true, latency: 2388, groupTag: "proxy-group", active: false
        ), onTap: {})
    }
    .padding()
    .background(Theme.backgroundPrimary)
    .preferredColorScheme(.dark)
}
