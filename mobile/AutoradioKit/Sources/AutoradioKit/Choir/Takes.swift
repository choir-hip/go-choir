import Foundation

// Takes, what stays native: the story that was on air when a take began recording (`TakeResponds`, the take recorder's provenance) and where earlier
// builds kept files (`TakeFiles`). The page and the wire for publishing are shared code: AutoradioLab/takes.ts, take-session.ts; the state table is in
// docs/reference/AUTORADIO-TAKES.md.

/// The story that was on air when recording began (`responds_to`): provenance for the take.
public struct TakeResponds: Codable, Equatable, Sendable {
    public var route: String
    public var title: String
    public init(route: String, title: String) { self.route = route; self.title = title }

}

/// Where the phone keeps these (its own support folder).
enum TakeFiles {
    /// Files earlier builds left (own ids and muted voices, and the read-only copies shared code once pushed): nothing reads them now; AccountWipe removes them with the account.
    static let legacy: [URL?] = [url("takes-own.json"), url("takes-muted.json"), url("takes-copy-own.json"), url("takes-copy-muted.json")]
    static func url(_ name: String) -> URL? {
        FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first?.appendingPathComponent("Autoradio").appendingPathComponent(name)
    }
}
