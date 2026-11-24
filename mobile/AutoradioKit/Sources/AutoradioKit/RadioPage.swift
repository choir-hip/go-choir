import Foundation

// What native code still reads of the Autoradio page's words: the address an entry opens and the stories it was built on (the takes pages call them). The page itself is shared code: AutoradioLab/radio-page.ts.

/// A story an entry was built on: the title and the page that opens it (its Texture, which shows its own sources).
public struct RadioBuiltOn: Codable, Sendable, Equatable {
    public var n = 0
    public var title = ""
    public var address = ""
    /// The cite's role when it is not the subject ("quoted", "mentioned"); "" for a subject.
    public var role = ""
    /// "cited v2, now v3" when the story has a newer version than the one cited (the reader opens the current version, not a pinned one).
    public var note = ""
    public init() {}
}

public enum RadioPage {
    /// The page an entry's title opens: a story, a book, a file.
    static func openAddress(_ e: RunSheetEntry) -> String {
        if let r = e.paperRoute { return "pub:" + r }
        // A desk story (kind story) names the story it tells in `source` (the first pub: cite): its title opens that story.
        if let s = e.source?.ref, s.hasPrefix("pub:"), e.kind == .story { return "pub:" + (s.dropFirst(4).split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false).first.map(String.init) ?? "") }
        if let b = e.book { return "doc:" + b.id }
        return e.filesAddress ?? ""
    }

    /// The stories an entry was built on: its cites and its source that name a story's page (`pub:<route>[@version]` or `texture:<id>`), once each, in
    /// order. The title is the label the server gave (it defaults to the story's own title). An opening, an answer or a clip with no story has none.
    public static func builtOn(_ e: RunSheetEntry, versions: [String: Int] = [:]) -> [RadioBuiltOn] {
        var seen = Set<String>(), out: [RadioBuiltOn] = []
        func add(_ ref: String, _ label: String, _ role: RunSheetRole? = nil) {
            guard ref.hasPrefix("pub:") || ref.hasPrefix("texture:") else { return }
            let address = ref.hasPrefix("pub:") ? "pub:" + (ref.dropFirst(4).split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false).first.map(String.init) ?? "") : ref
            guard address.count > 4, seen.insert(address).inserted else { return }
            var b = RadioBuiltOn()
            b.n = out.count; b.address = address
            if let role, role != .subject { b.role = role.rawValue }
            if ref.hasPrefix("pub:"), let at = ref.firstIndex(of: "@"), ref[ref.index(after: at)...].hasPrefix("v"), let cited = Int(ref[ref.index(at, offsetBy: 2)...]),
               let now = versions[String(address.dropFirst(4))], now != cited { b.note = "cited v\(cited), now v\(now)" }
            let t = label.trimmingCharacters(in: .whitespacesAndNewlines)
            b.title = t.isEmpty ? String(address.drop(while: { $0 != ":" }).dropFirst()) : t
            out.append(b)
        }
        for c in e.cites ?? [] { add(c.ref, c.label, c.role) }
        if let s = e.source { add(s.ref, s.label) }
        return out
    }
}
