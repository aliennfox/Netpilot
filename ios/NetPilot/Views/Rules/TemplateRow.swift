import SwiftUI

struct TemplateRow: View {
    let template: TemplateModel
    let onApply: () -> Void

    private var icon: String {
        switch template.id {
        case "netflix": return "play.tv.fill"
        case "ai_services": return "sparkles"
        case "social_media": return "bubble.left.and.bubble.right.fill"
        case "google": return "magnifyingglass"
        default: return "doc.text.fill"
        }
    }

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: icon)
                .foregroundStyle(Theme.accentPurple)
                .frame(width: 24)

            VStack(alignment: .leading, spacing: 2) {
                Text(template.name)
                    .font(.system(size: 15))
                    .foregroundStyle(Theme.textPrimary)
                Text(template.description)
                    .font(.system(size: 12))
                    .foregroundStyle(Theme.textSecondary)
                    .lineLimit(1)
            }

            Spacer()

            Button("应用", action: onApply)
                .font(.system(size: 13, weight: .medium))
                .foregroundStyle(Theme.accentBlue)
                .padding(.horizontal, 12)
                .padding(.vertical, 6)
                .background(Theme.accentBlue.opacity(0.15))
                .clipShape(Capsule())
        }
        .padding(12)
        .background(Theme.backgroundSecondary)
    }
}
