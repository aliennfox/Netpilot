import SwiftUI

struct ChatInputBar: View {
    @Binding var text: String
    let isLoading: Bool
    let onSend: () -> Void

    var body: some View {
        HStack(spacing: 12) {
            TextField("输入指令或问题...", text: $text)
                .font(.system(size: 15))
                .padding(12)
                .background(Theme.backgroundTertiary)
                .clipShape(RoundedRectangle(cornerRadius: 12))
                .foregroundStyle(Theme.textPrimary)
                .onSubmit { onSend() }

            Button(action: onSend) {
                Image(systemName: "arrow.up.circle.fill")
                    .font(.system(size: 32))
                    .foregroundStyle(
                        text.trimmingCharacters(in: .whitespaces).isEmpty || isLoading
                            ? Theme.textTertiary
                            : Theme.accentBlue
                    )
            }
            .disabled(text.trimmingCharacters(in: .whitespaces).isEmpty || isLoading)
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
        .background(Theme.backgroundSecondary)
    }
}
