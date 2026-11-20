import Foundation

/// `pub:<route>` as plain paragraphs, from the network.
/// The spine's text renderer shows it; lane 3 replaces it with the real reader and its own source.
public struct PublicationTextSource: PageSource {
    nonisolated(unsafe) public static var offline: (@Sendable (String) -> ReaderDoc?)?
    public init() {}
    public func handles(_ address: String) -> Bool { address.hasPrefix("pub:") }
    public func load(_ address: String) async throws -> PageModel {
        let route = PageAddress.rest(of: address)
        let o: [String: Any]
        do {
            o = try await ChoirEnvironment.shared.client().send("GET", "/api/platform/publications/resolve", query: ["route": route],
                                                                  authenticated: false, route: "/api/platform/publications/resolve").object()
        } catch {
            // No network: the story the phone holds (the ContentStore) is read from there.
            if let d = Self.offline?(address) { return PageModel(address: address, kind: "reader", title: d.title, status: "ready", lines: d.lines) }
            throw error
        }
        let title = ((o["publication"] as? [String: Any])?["title"] as? String) ?? "Story"
        let text = ((o["payload"] as? [String: Any])?["text"] as? String) ?? ""
        return PageModel(address: address, kind: "reader", title: title, status: "ready", lines: ChoirText.paragraphs(text))
    }
}
