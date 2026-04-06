import SwiftUI

struct ProxyModeCard: View {
    let currentMode: String
    let onModeChange: (String) -> Void

    private let modes = [
        ("direct", "直连"),
        ("global", "全局代理"),
        ("rule", "规则模式"),
    ]

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 8) {
                Image(systemName: "bolt.fill")
                    .foregroundStyle(Theme.accentOrange)
                Text("代理模式")
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundStyle(Theme.textPrimary)
            }

            HStack(spacing: 8) {
                ForEach(modes, id: \.0) { mode in
                    let isActive = currentMode.lowercased().contains(mode.0)
                    Button(action: { onModeChange(mode.0) }) {
                        Text(mode.1)
                            .font(.system(size: 13, weight: .medium))
                            .foregroundStyle(isActive ? .white : Theme.textSecondary)
                            .padding(.horizontal, 14)
                            .padding(.vertical, 8)
                            .background(isActive ? Theme.accentBlue : Theme.backgroundTertiary)
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

#Preview {
    ProxyModeCard(currentMode: "rule", onModeChange: { _ in })
        .padding()
        .background(Theme.backgroundPrimary)
        .preferredColorScheme(.dark)
}
