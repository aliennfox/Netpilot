import Foundation

struct RuleModel: Codable, Identifiable {
    var id: String { tag }
    let tag: String
    let domainSuffix: [String]?
    let domain: [String]?
    let ipCidr: [String]?
    let processName: [String]?
    let outbound: String
    let description: String
    let source: String

    enum CodingKeys: String, CodingKey {
        case tag
        case domainSuffix = "domain_suffix"
        case domain
        case ipCidr = "ip_cidr"
        case processName = "process_name"
        case outbound, description, source
    }

    var domainCount: Int {
        (domainSuffix?.count ?? 0) + (domain?.count ?? 0)
    }

    var summary: String {
        var parts: [String] = []
        if let ds = domainSuffix, !ds.isEmpty {
            parts.append("\(ds.first ?? "") 等 \(ds.count) 域名")
        }
        if let d = domain, !d.isEmpty {
            parts.append("\(d.count) 个域名")
        }
        if let ip = ipCidr, !ip.isEmpty {
            parts.append("\(ip.count) 个 IP 段")
        }
        return parts.joined(separator: ", ")
    }
}
