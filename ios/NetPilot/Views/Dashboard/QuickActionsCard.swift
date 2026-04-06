import SwiftUI

struct QuickActionsCard: View {
    let onAction: (String) -> Void

    private let actions = [
        ("测速", "speedometer"),
        ("换最快节点", "arrow.triangle.2.circlepath"),
        ("配 Netflix", "play.tv.fill"),
        ("查日志", "doc.text.magnifyingglass"),
        ("诊断网络", "stethoscope"),
    ]

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 8) {
                Image(systemName: "sparkles")
                    .foregroundStyle(Theme.accentPurple)
                Text("AI 快捷操作")
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundStyle(Theme.textPrimary)
            }

            FlowLayout(spacing: 8) {
                ForEach(actions, id: \.0) { action in
                    Button(action: {
                        onAction(action.0)
                    }) {
                        HStack(spacing: 4) {
                            Image(systemName: action.1)
                                .font(.system(size: 11))
                            Text(action.0)
                                .font(.system(size: 13))
                        }
                        .foregroundStyle(Theme.textPrimary)
                        .padding(.horizontal, 14)
                        .padding(.vertical, 8)
                        .background(Theme.backgroundTertiary)
                        .clipShape(Capsule())
                    }
                }
            }
        }
        .padding(16)
        .background(Theme.backgroundSecondary)
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }
}

// 简易 FlowLayout
struct FlowLayout: Layout {
    var spacing: CGFloat = 8

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let result = arrangeSubviews(proposal: proposal, subviews: subviews)
        return result.size
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        let result = arrangeSubviews(proposal: proposal, subviews: subviews)
        for (index, pos) in result.positions.enumerated() {
            subviews[index].place(at: CGPoint(x: bounds.minX + pos.x, y: bounds.minY + pos.y), proposal: .unspecified)
        }
    }

    private func arrangeSubviews(proposal: ProposedViewSize, subviews: Subviews) -> (positions: [CGPoint], size: CGSize) {
        let maxWidth = proposal.width ?? .infinity
        var positions: [CGPoint] = []
        var x: CGFloat = 0
        var y: CGFloat = 0
        var rowHeight: CGFloat = 0

        for subview in subviews {
            let size = subview.sizeThatFits(.unspecified)
            if x + size.width > maxWidth && x > 0 {
                x = 0
                y += rowHeight + spacing
                rowHeight = 0
            }
            positions.append(CGPoint(x: x, y: y))
            rowHeight = max(rowHeight, size.height)
            x += size.width + spacing
        }

        return (positions, CGSize(width: maxWidth, height: y + rowHeight))
    }
}

#Preview {
    QuickActionsCard(onAction: { _ in })
        .padding()
        .background(Theme.backgroundPrimary)
        .preferredColorScheme(.dark)
}
