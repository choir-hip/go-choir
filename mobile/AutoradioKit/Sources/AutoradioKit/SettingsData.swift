import Foundation

// "What the paper has learned" reads a LearnedProvider; the product one is `PreferencesLearned`.
// (The swipe-meaning settings are gone with the swipes, 2026-11-19.)
//
// Ways it can fail (tests in SettingsDataTests):
//  L1 The learned list shows acts from before a Reset: the list is cut at the newest "learned-reset" event.
//  L2 Reset deletes the gesture events: it only records a marker.
//  L3 A subject that is a private address is shown in full: lines use the detail text lane 4 wrote, else the bare address.

// MARK: What the paper has learned

public struct LearnedLine: Codable, Sendable, Equatable {
    public var at: Double, kind: String, text: String
    public init(at: Double, kind: String, text: String) { self.at = at; self.kind = kind; self.text = text }
}

public protocol LearnedProvider: Sendable {
    func lines() async -> [LearnedLine]
    func reset() async
}

public struct LearnedRow: Codable, Sendable, Equatable { public var line: String, kind: String, ago: String }
public struct LearnedView: Codable, Sendable, Equatable {
    public var rows: [LearnedRow] = []
    public var count = 0
    public var message = ""
    public init() {}
    public init(lines: [LearnedLine], now: Date = Date()) {
        rows = lines.map { LearnedRow(line: $0.text, kind: $0.kind, ago: Self.when($0.at, now: now)) }
        count = rows.count
        message = rows.isEmpty ? "Nothing yet. Swipe right on a story you want more of, and it shows here." : ""
    }
    static func when(_ at: Double, now: Date) -> String {
        let f = DateFormatter(); f.locale = Locale(identifier: "en_US_POSIX"); f.dateFormat = "d MMM HH:mm"
        return f.string(from: Date(timeIntervalSince1970: at))
    }
}
