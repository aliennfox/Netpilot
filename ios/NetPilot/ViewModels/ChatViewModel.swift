import Foundation

@MainActor
class ChatViewModel: ObservableObject {
    @Published var messages: [ChatMessage] = []
    @Published var inputText = ""
    @Published var isLoading = false

    private let api = APIClient.shared

    func sendMessage() async {
        let text = inputText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }

        let userMsg = ChatMessage(role: .user, content: text)
        messages.append(userMsg)
        inputText = ""
        isLoading = true
        defer { isLoading = false }

        do {
            let response = try await api.sendChat(message: text)
            let botMsg = ChatMessage(
                role: .assistant,
                content: response.reply,
                actions: response.actions ?? []
            )
            messages.append(botMsg)
        } catch {
            let errorMsg = ChatMessage(
                role: .assistant,
                content: "Error: \(error.localizedDescription)"
            )
            messages.append(errorMsg)
        }
    }

    func sendQuickAction(_ command: String) async {
        inputText = command
        await sendMessage()
    }
}
