import SwiftUI

struct ChatView: View {
    @ObservedObject var viewModel: ChatViewModel

    private let quickActions = [
        ("测速", "speedometer"),
        ("换最快节点", "arrow.triangle.2.circlepath"),
        ("配Netflix", "play.tv.fill"),
        ("查日志", "doc.text.magnifyingglass"),
        ("诊断网络", "stethoscope"),
    ]

    var body: some View {
        VStack(spacing: 0) {
            // 消息列表
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(spacing: 12) {
                        ForEach(viewModel.messages) { message in
                            ChatBubble(
                                message: message,
                                isProcessing: viewModel.isProcessing
                            ) { command in
                                Task { await viewModel.sendQuickAction(command) }
                            }
                        }

                        // Typing indicator
                        if viewModel.isLoading {
                            TypingIndicator()
                        }

                        // 底部锚点
                        Color.clear
                            .frame(height: 1)
                            .id("bottom")
                    }
                    .padding(16)
                }
                .onChange(of: viewModel.messages.count) { _, _ in
                    withAnimation(.easeOut(duration: 0.3)) {
                        DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
                            proxy.scrollTo("bottom", anchor: .bottom)
                        }
                    }
                }
                .onChange(of: viewModel.isLoading) { _, _ in
                    withAnimation(.easeOut(duration: 0.3)) {
                        DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
                            proxy.scrollTo("bottom", anchor: .bottom)
                        }
                    }
                }
            }

            // 建议按钮栏
            suggestionsBar

            // 输入栏
            ChatInputBar(
                text: $viewModel.inputText,
                isLoading: viewModel.isLoading,
                isProcessing: viewModel.isProcessing
            ) {
                Task { await viewModel.sendMessage() }
            }
        }
        .background(Theme.backgroundPrimary)
    }

    // MARK: - 建议按钮

    @ViewBuilder
    private var suggestionsBar: some View {
        let agentActions = viewModel.latestActions

        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 8) {
                // 固定快捷操作
                ForEach(quickActions, id: \.0) { label, icon in
                    Button {
                        Task { await viewModel.sendQuickAction(label) }
                    } label: {
                        HStack(spacing: 4) {
                            Image(systemName: icon)
                                .font(.system(size: 11))
                            Text(label)
                                .font(.system(size: 13))
                        }
                        .foregroundStyle(
                            viewModel.isProcessing ? Theme.textTertiary : Theme.textPrimary
                        )
                        .padding(.horizontal, 14)
                        .padding(.vertical, 8)
                        .background(Theme.backgroundTertiary)
                        .clipShape(Capsule())
                    }
                    .disabled(viewModel.isProcessing)
                }

                // Agent 返回的动态建议
                if !agentActions.isEmpty {
                    Divider()
                        .frame(height: 20)

                    ForEach(agentActions) { action in
                        Button {
                            Task { await viewModel.sendQuickAction(action.command) }
                        } label: {
                            Text(action.label)
                                .font(.system(size: 13))
                                .foregroundStyle(
                                    viewModel.isProcessing ? Theme.textTertiary : Theme.accentPurple
                                )
                                .padding(.horizontal, 14)
                                .padding(.vertical, 8)
                                .background(Theme.accentPurple.opacity(0.15))
                                .clipShape(Capsule())
                        }
                        .disabled(viewModel.isProcessing)
                    }
                }
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 8)
        }
        .background(Theme.backgroundSecondary)
    }
}

// MARK: - Typing Indicator (三点跳动动画)

struct TypingIndicator: View {
    @State private var animating = false

    var body: some View {
        HStack {
            HStack(spacing: 4) {
                ForEach(0..<3) { index in
                    Circle()
                        .fill(Theme.accentPurple)
                        .frame(width: 8, height: 8)
                        .offset(y: animating ? -4 : 4)
                        .animation(
                            .easeInOut(duration: 0.5)
                                .repeatForever(autoreverses: true)
                                .delay(Double(index) * 0.15),
                            value: animating
                        )
                }
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 12)
            .background(Theme.backgroundTertiary)
            .clipShape(RoundedRectangle(cornerRadius: 16))

            Spacer(minLength: 40)
        }
        .onAppear { animating = true }
    }
}

#Preview {
    ChatView(viewModel: ChatViewModel())
        .preferredColorScheme(.dark)
}
