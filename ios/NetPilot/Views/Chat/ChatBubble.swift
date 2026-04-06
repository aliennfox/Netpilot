import SwiftUI

struct ChatBubble: View {
    let message: ChatMessage
    var isProcessing: Bool = false
    let onAction: (String) -> Void

    private var isUser: Bool { message.role == .user }

    var body: some View {
        HStack {
            if isUser { Spacer(minLength: 60) }

            VStack(alignment: isUser ? .trailing : .leading, spacing: 8) {
                Text(message.content)
                    .font(.system(size: 15))
                    .foregroundStyle(isUser ? .white : Theme.textPrimary)
                    .padding(12)
                    .background(isUser ? Theme.accentBlue : Theme.backgroundTertiary)
                    .clipShape(
                        RoundedRectangle(cornerRadius: 16)
                    )

                // Agent 建议操作按钮
                if !isUser && !message.actions.isEmpty {
                    HStack(spacing: 8) {
                        ForEach(message.actions) { action in
                            Button(action: { onAction(action.command) }) {
                                Text(action.label)
                                    .font(.system(size: 13))
                                    .foregroundStyle(
                                        isProcessing ? Theme.textTertiary : Theme.textPrimary
                                    )
                                    .padding(.horizontal, 12)
                                    .padding(.vertical, 6)
                                    .background(Theme.backgroundTertiary)
                                    .clipShape(Capsule())
                            }
                            .disabled(isProcessing)
                        }
                    }
                }
            }

            if !isUser { Spacer(minLength: 40) }
        }
    }
}
