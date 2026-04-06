import SwiftUI

struct ChatInputBar: View {
    @Binding var text: String
    let isLoading: Bool
    var isProcessing: Bool = false
    let onSend: () -> Void

    private var isDisabled: Bool { isLoading || isProcessing }

    var body: some View {
        HStack(spacing: 12) {
            TextField(
                isProcessing ? "AI 处理中..." : "输入指令或问题...",
                text: $text
            )
            .font(.system(size: 15))
            .padding(12)
            .background(Theme.backgroundTertiary)
            .clipShape(RoundedRectangle(cornerRadius: 12))
            .foregroundStyle(isProcessing ? Theme.textTertiary : Theme.textPrimary)
            .disabled(isProcessing)
            .onSubmit { if !isDisabled { onSend() } }

            Button(action: onSend) {
                Image(systemName: "arrow.up.circle.fill")
                    .font(.system(size: 32))
                    .foregroundStyle(
                        text.trimmingCharacters(in: .whitespaces).isEmpty || isDisabled
                            ? Theme.textTertiary
                            : Theme.accentBlue
                    )
            }
            .disabled(text.trimmingCharacters(in: .whitespaces).isEmpty || isDisabled)
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
        .background(Theme.backgroundSecondary)
    }
}
