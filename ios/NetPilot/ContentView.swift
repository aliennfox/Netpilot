import SwiftUI

struct ContentView: View {
    @State private var selectedTab = 0

    var body: some View {
        TabView(selection: $selectedTab) {
            HomeView()
                .tabItem { Label("首页", systemImage: "house.fill") }
                .tag(0)
            NodesView()
                .tabItem { Label("节点", systemImage: "server.rack") }
                .tag(1)
            RulesView()
                .tabItem { Label("规则", systemImage: "arrow.triangle.branch") }
                .tag(2)
            SettingsView()
                .tabItem { Label("设置", systemImage: "gearshape.fill") }
                .tag(3)
        }
        .tint(Theme.accentBlue)
    }
}

#Preview {
    ContentView()
        .preferredColorScheme(.dark)
}
