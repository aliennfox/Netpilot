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
            // 消息列表 — 反转 ScrollView 技巧，新消息自动出现在底部
            ScrollView {
                VStack(spacing: 12) {
                    // 反转后 VStack 第一个元素在视觉最底部
                    // 所以 loading 放最前，消息倒序排列

                    if viewModel.isLoading {
                        HStack {
                            ProgressView()
                                .tint(Theme.accentPurple)
                            Text("AI 处理中...")
                                .font(.system(size: 13))
                                .foregroundStyle(Theme.textSecondary)
                            Spacer()
                        }
                        .padding(.horizontal, 16)
                        .rotationEffect(.degrees(180))
                        .scaleEffect(x: -1, y: 1, anchor: .center)
                    }

                    ForEach(viewModel.messages.reversed()) { message in
                        ChatBubble(
                            message: message,
                            isProcessing: viewModel.isProcessing
                        ) { command in
                            Task { await viewModel.sendQuickAction(command) }
                        }
                        .rotationEffect(.degrees(180))
                        .scaleEffect(x: -1, y: 1, anchor: .center)
                    }
                }
                .padding(16)
            }
            // 反转整个 ScrollView：顶部变底部，新消息自然出现在可视区域底部
            .rotationEffect(.degrees(180))
            .scaleEffect(x: -1, y: 1, anchor: .center)

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

#Preview {
    ChatView(viewModel: ChatViewModel())
        .preferredColorScheme(.dark)
}
