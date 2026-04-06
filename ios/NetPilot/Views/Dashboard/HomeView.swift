import SwiftUI

struct HomeView: View {
    @State private var mode: HomeMode = .dashboard
    @StateObject private var chatVM = ChatViewModel()

    enum HomeMode: String, CaseIterable {
        case dashboard = "Dashboard"
        case chat = "Chat"
    }

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                Picker("", selection: $mode) {
                    ForEach(HomeMode.allCases, id: \.self) { m in
                        Text(m.rawValue).tag(m)
                    }
                }
                .pickerStyle(.segmented)
                .padding(.horizontal, 16)
                .padding(.vertical, 8)

                switch mode {
                case .dashboard:
                    DashboardView()
                case .chat:
                    ChatView(viewModel: chatVM)
                }
            }
            .background(Theme.backgroundPrimary)
            .navigationTitle("NetPilot")
            .navigationBarTitleDisplayMode(.inline)
            .toolbarBackground(Theme.backgroundSecondary, for: .navigationBar)
            .toolbarBackground(.visible, for: .navigationBar)
        }
    }
}

#Preview {
    HomeView()
        .preferredColorScheme(.dark)
}
