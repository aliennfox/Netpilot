import SwiftUI

struct Theme {
    // 背景
    static let backgroundPrimary = Color(hex: "0A0A0F")
    static let backgroundSecondary = Color(hex: "12121A")
    static let backgroundTertiary = Color(hex: "1A1A26")
    static let backgroundElevated = Color(hex: "22222E")

    // 文字
    static let textPrimary = Color(hex: "EAEAF0")
    static let textSecondary = Color(hex: "8888A0")
    static let textTertiary = Color(hex: "55556A")
    static let textInverse = Color(hex: "0A0A0F")

    // 功能色
    static let accentBlue = Color(hex: "4A9EFF")
    static let accentGreen = Color(hex: "34D399")
    static let accentRed = Color(hex: "F87171")
    static let accentOrange = Color(hex: "FB923C")
    static let accentPurple = Color(hex: "A78BFA")
    static let accentTeal = Color(hex: "2DD4BF")

    // 延迟色阶
    static func latencyColor(_ ms: Int) -> Color {
        switch ms {
        case ..<0: return textTertiary
        case 0..<100: return accentGreen
        case 100..<300: return accentTeal
        case 300..<500: return accentOrange
        default: return accentRed
        }
    }

    // 协议色
    static func protocolColor(_ proto: String) -> Color {
        switch proto.lowercased() {
        case "hysteria2", "hy2": return accentBlue
        case "vless": return accentPurple
        case "shadowsocks", "ss": return accentGreen
        case "trojan": return accentOrange
        case "vmess": return accentRed
        default: return textSecondary
        }
    }
}

extension Color {
    init(hex: String) {
        let scanner = Scanner(string: hex)
        var rgbValue: UInt64 = 0
        scanner.scanHexInt64(&rgbValue)
        self.init(
            red: Double((rgbValue >> 16) & 0xFF) / 255.0,
            green: Double((rgbValue >> 8) & 0xFF) / 255.0,
            blue: Double(rgbValue & 0xFF) / 255.0
        )
    }
}
