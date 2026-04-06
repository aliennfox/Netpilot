import SwiftUI

struct ChatView: View {
    @ObservedObject var viewModel: ChatViewModel

    var body: some View {
        VStack(spacing: 0) {
            // 消息列表
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(spacing: 12) {
                        ForEach(viewModel.messages) { message in
                            ChatBubble(message: message) { command in
                                Task { await viewModel.sendQuickAction(command) }
                            }
                        }

                        if viewModel.isLoading {
                            HStack {
                                ProgressView()
                                    .tint(Theme.accentPurple)
                                Text("思考中...")
                                    .font(.system(size: 13))
                                    .foregroundStyle(Theme.textSecondary)
                                Spacer()
                            }
                            .padding(.horizontal, 16)
                            .id("loading")
                        }
                    }
                    .padding(16)
                }
                .onChange(of: viewModel.messages.count) {
                    withAnimation {
                        proxy.scrollTo(viewModel.messages.last?.id, anchor: .bottom)
                    }
                }
            }

            // 输入栏
            ChatInputBar(text: $viewModel.inputText, isLoading: viewModel.isLoading) {
                Task { await viewModel.sendMessage() }
            }
        }
        .background(Theme.backgroundPrimary)
    }
}

#Preview {
    ChatView(viewModel: ChatViewModel())
        .preferredColorScheme(.dark)
}
