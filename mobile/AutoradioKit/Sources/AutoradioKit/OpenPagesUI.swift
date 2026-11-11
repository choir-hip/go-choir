import Foundation

// The data behind the pile and the launcher (lane 4): the places with a line of what is new, the recent pages, search over
// pages / places / stories, and the Undo offer after a close. The drawing is Contract (cards.contract, launcher.contract);
// nothing here is a view.
//
// Ways this can fail (written before the code, per the repo testing rule):
//  1. Search with an empty or whitespace query: no hits (the launcher shows its places instead).
//  2. Search matches the same address through two routes (an open page that is also a story): one hit, the first group wins
//     (open pages, then places, then recent, then stories).
//  3. A query with regex or SQL characters: matching is plain substring on lower-cased text, nothing is interpreted.
//  4. "Recent" lists a page that is open now, or the same address twice: both are filtered; the newest event wins.
//  5. A recent address from an older build whose scheme is unknown: it gets the default title and the "unknown" kind and is
//     dropped (it would open a "not built yet" page).
//  6. The Undo offer outlives its 5 s: `UndoOffer.isOpen(at:)` is a pure function of the clock, so a late touch cannot
//     leave it showing; a second close replaces the first (only the last closed page is restorable, as the pile keeps one).
//  7. A place line with newlines or very long text: collapsed to one line and cut at 60 characters.

public struct PlaceInfo: Equatable, Sendable {
    public var slug: String
    public var name: String
    public var address: String
    public var line: String
}

public enum Places {
    /// The launcher's places, in the plan's order, each with a one-line "what is new there". `extra` maps a slug to its
    /// line (demo mode reads `fixtures/demo/places.json`; the other lanes' real lines are theirs to supply and are
    /// blank until then: mail unread count, new files, new revisions). Paper's line is the top story's title.
    public static func rows(topStory: String?, extra: [String: String]) -> [PlaceInfo] {
        PageAddress.places.map { p in
            let slug = p.name.lowercased()
            var line = extra[slug] ?? ""
            if slug == "paper", let t = topStory, !t.isEmpty { line = t }
            return PlaceInfo(slug: slug, name: p.name, address: p.address, line: oneLine(line))
        }
    }
    static func oneLine(_ s: String) -> String {
        let flat = s.split(whereSeparator: { $0.isNewline || $0.isWhitespace }).joined(separator: " ")
        return flat.count > 60 ? String(flat.prefix(59)) + "…" : flat
    }
    public static func demoExtra() -> [String: String] {
        guard let d = Demo.fixture("places"), let o = try? JSONSerialization.jsonObject(with: d) as? [String: String] else { return [:] }
        return o
    }
}

public struct SearchHit: Equatable, Sendable {
    public var address: String
    public var title: String
    public var kind: String      // open | place | recent | story
    public var detail: String
}

public enum LauncherSearch {
    /// Search over open pages, places, recent pages and today's stories (plan: "page titles, document titles, file names and
    /// mail subjects" - the file and mail rows join when those lanes expose their own listings).
    public static func hits(query: String, open: [Page], places: [PlaceInfo], recent: [Page], stories: [StoryFacts], limit: Int = 12) -> [SearchHit] {
        let q = query.lowercased().split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
        guard !q.isEmpty else { return [] }
        func score(_ text: String) -> Int {
            let t = text.lowercased()
            if t.hasPrefix(q) { return 3 }
            if t.split(whereSeparator: { !$0.isLetter && !$0.isNumber }).contains(where: { $0.hasPrefix(q) }) { return 2 }
            return t.contains(q) ? 1 : 0
        }
        var out: [(Int, Int, SearchHit)] = []
        var seen = Set<String>()
        func add(_ group: Int, _ s: Int, _ h: SearchHit) {
            guard s > 0, !seen.contains(h.address) else { return }
            seen.insert(h.address); out.append((s, group, h))
        }
        for p in open { add(0, score(p.title), SearchHit(address: p.address, title: p.title, kind: "open", detail: "Open")) }
        for p in places { add(1, max(score(p.name), score(p.line) > 0 ? 1 : 0), SearchHit(address: p.address, title: p.name, kind: "place", detail: p.line)) }
        for p in recent { add(2, score(p.title), SearchHit(address: p.address, title: p.title, kind: "recent", detail: "Recent")) }
        for s in stories {
            let inDek = score(s.dek) > 0 ? 1 : 0
            let byEntity = s.entities.contains(where: { score($0) > 0 }) ? 1 : 0
            let sc = max(score(s.title), inDek, byEntity)
            add(3, sc, SearchHit(address: s.address, title: s.title, kind: "story", detail: s.section))
        }
        // best score first; within a score the group order (open, place, recent, story)
        return Array(out.enumerated().sorted { a, b in
            if a.element.0 != b.element.0 { return a.element.0 > b.element.0 }
            if a.element.1 != b.element.1 { return a.element.1 < b.element.1 }
            return a.offset < b.offset
        }.map { $0.element.2 }.prefix(limit))
    }
}

public enum RecentPages {
    /// Addresses of pages opened or closed lately, newest first, without those open now, one entry each.
    public static func addresses(events: [SignalEvent], excluding open: Set<String>, limit: Int = 8) -> [String] {
        var seen = open, out: [String] = []
        for e in events.sorted(by: { $0.at > $1.at }) where e.kind == SignalKind.open || e.kind == SignalKind.close {
            guard !seen.contains(e.subject), PageAddress.kind(of: e.subject) != "unknown" else { continue }
            seen.insert(e.subject); out.append(e.subject)
            if out.count == limit { break }
        }
        return out
    }
    /// A page's title for a list: a story's own title when known, else the address's default.
    public static func title(for address: String, stories: [StoryFacts]) -> String {
        if address.hasPrefix("pub:"), let s = stories.first(where: { $0.address == address }) { return s.title }
        return PageAddress.defaultTitle(address)
    }
}

/// The Undo pill after a close: open for 5 s from the close, naming what was closed.
public struct UndoOffer: Equatable, Sendable {
    public static let seconds = 5.0
    public var title: String
    public var address: String
    public var since: Double
    public init(title: String, address: String, since: Double) { self.title = title; self.address = address; self.since = since }
    public func isOpen(at now: Double) -> Bool { now >= since && now - since < Self.seconds }
}
