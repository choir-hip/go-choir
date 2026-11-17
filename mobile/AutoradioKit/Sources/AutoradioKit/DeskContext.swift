import Foundation

// What the phone tells the model about "now". Two defects found 2026-11-17 (owner: "Read my book" answered "the page is empty"):
//  1. When the page in front is the Autoradio page (or the paper's front, a launcher: not a text), `pageTitle` named THAT page, so "tell me more
//     about that" searched for the word "Autoradio" instead of what is playing, and the server desk was sent a page with no text.
//  2. The server desk (the one the listener hears last) was never sent anything the phone retrieved.
// RULES (tests in TurnContextTests):
//  - "What is on air": a content page in front (a story, book, document, mail, podcast, file) is what "that" and "this" mean; for any other page
//    the title of what the run sheet has on air is.
//  - The page sent to the server desk keeps its address; its text is the page's own text when it has one, else a brief of what is on air and ahead;
//    the retrieved passages follow in both cases. Nothing is ever sent as an empty page.
public enum OnAir {
    static let contentPrefixes = ["pub:", "doc:", "texture:", "mail:", "podcast:", "file:", "url:", "web:"]
    public static func isContent(_ address: String) -> Bool { contentPrefixes.contains { address.hasPrefix($0) } }
    public static func title(pageAddress: String, pageTitle: String, hint: String) -> String {
        if (isContent(pageAddress) || pageAddress.isEmpty), !pageTitle.isEmpty { return pageTitle }       // no address: the caller named what is on air
        return hint.isEmpty ? (isContent(pageAddress) ? pageTitle : "") : hint
    }
}

public extension RetrievalContext {
    /// The same context without the items that must stay on the phone (DeskContext.isPrivate), re-rendered; empty when nothing is left.
    func sendable() -> RetrievalContext {
        let keep = items.filter { !DeskContext.isPrivate($0) }
        guard keep.count != items.count else { return self }
        var rec = record
        rec.hits = rec.hits.filter { h in keep.contains { $0.id == h.id } }
        guard !keep.isEmpty else { return RetrievalContext(text: "", items: [], record: rec) }
        let text = RetrievalIndex.header(query: record.query, onAir: record.onAir, now: record.at) + keep.enumerated().map { RetrievalIndex.block($0.offset + 1, $0.element) }.joined()
        rec.usedChars = text.count
        return RetrievalContext(text: text, items: keep, record: rec)
    }
}

public enum DeskContext {
    /// What stays on the phone (owner, 2026-11-17: mail is kept on the phone): mail, and files that exist only here. Stories, podcasts, transcripts, books from the
    /// Choir library, Texture documents, files synced from the owner's Choir computer and the run sheet are already on the server and may be sent.
    public static func isPrivate(_ i: RetrievalItem) -> Bool { i.kind == "mail" || i.source == "Local files" }
    public static func isPrivateAddress(_ a: String) -> Bool { a.hasPrefix("mail:") }
    /// The turn is about private material: its best passage (by score) is private. The local answer then stands and the desk is not asked.
    public static func keepsLocal(_ c: RetrievalContext?) -> Bool {
        guard let best = c?.items.max(by: { $0.score < $1.score }) else { return false }
        return isPrivate(best)
    }

    /// The page the server desk is sent. `pageText` is what the phone has for the page in front; `brief` what is on air and ahead.
    public static func page(front: FrontPage?, pageText: String, brief: String, retrieved: RetrievalContext?) -> DeskPage? {
        let shown = retrieved?.sendable()
        let passages = (shown?.isEmpty == false) ? shown!.text : ""
        let privatePage = isPrivateAddress(front?.address ?? "")             // a mail message in front is not sent: not its words, not its subject
        let own = privatePage ? "" : pageText.trimmingCharacters(in: .whitespacesAndNewlines)
        let front = privatePage ? front.map { FrontPage(address: $0.address, title: "A mail message", kind: $0.kind) } : front
        guard front != nil || !brief.isEmpty || !passages.isEmpty else { return nil }
        var parts: [String] = []
        if own.isEmpty || !OnAir.isContent(front?.address ?? "") { if !brief.isEmpty { parts.append(brief) } }
        if !own.isEmpty { parts.append(own) }
        if !passages.isEmpty { parts.append("Found on the listener's phone for this question:\n" + passages) }
        let text = parts.joined(separator: "\n\n")
        guard !text.isEmpty else { return front.map { DeskPage(address: $0.address, title: $0.title, text: "") } }
        let title = (front?.title.isEmpty == false ? front!.title : "On air and ahead")
        return DeskPage(address: front?.address ?? "", title: title, text: text)
    }

    /// One console line per turn: counts and kinds only, never content.
    public static func turnLine(desk: String, retrieved: RetrievalContext?, command: String, reason: String = "") -> String {
        let kinds = Set((retrieved?.items ?? []).map(\.kind)).sorted().joined(separator: ",")
        return "AUTORADIO turn desk=\(desk)\(reason.isEmpty ? "" : " reason=" + reason) retrieved=\(retrieved?.items.count ?? 0) sources=\(kinds.isEmpty ? "-" : kinds) context_chars=\(retrieved?.text.count ?? 0) command=\(command)"
    }
}
