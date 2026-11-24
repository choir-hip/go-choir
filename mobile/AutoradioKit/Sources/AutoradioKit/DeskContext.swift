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

public enum DeskContext {
    /// The page the server desk is sent. `pageText` is what the phone has for the page in front; `brief` what is on air and ahead.
    public static func page(front: FrontPage?, pageText: String, brief: String, retrieved: RetrievalContext?) -> DeskPage? {
        let passages = (retrieved?.isEmpty == false) ? retrieved!.text : ""
        let own = pageText.trimmingCharacters(in: .whitespacesAndNewlines)
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
    public static func turnLine(desk: String, retrieved: RetrievalContext?) -> String {
        let kinds = Set((retrieved?.items ?? []).map(\.kind)).sorted().joined(separator: ",")
        return "AUTORADIO turn desk=\(desk) retrieved=\(retrieved?.items.count ?? 0) sources=\(kinds.isEmpty ? "-" : kinds) context_chars=\(retrieved?.text.count ?? 0)"
    }
}

/// The paper's stories the server desk may add to the queue by voice (the turn request's `candidates`). The list is shared code's (AutoradioLab/paper.ts `handVoiceFront`, the app module's
/// `voiceFront` op): the front route is read there. Native holds a copy only because the voice turn engine is native and cannot ask shared code; this decides nothing. A hand-over made under an
/// older account generation is refused (`SharedGeneration`), and sign-out empties it (`AccountWipe`).
public final class VoiceFront: @unchecked Sendable {
    public static let shared = VoiceFront()
    private let lock = NSLock()
    private var rows: [(route: String, title: String)] = []
    public init() {}
    /// The first 40 as the turn takes them.
    public var candidates: [(route: String, title: String)] { lock.lock(); defer { lock.unlock() }; return Array(rows.prefix(40)) }
    @discardableResult
    public func set(gen: Int, _ list: [[String: Any]]) -> Bool {
        let next = list.compactMap { o -> (route: String, title: String)? in
            guard let r = o["route"] as? String, !r.isEmpty else { return nil }
            return (r, (o["title"] as? String) ?? "")
        }
        return SharedGeneration.admit(gen) { lock.lock(); rows = next; lock.unlock() }
    }
    public func wipe() { lock.lock(); rows = []; lock.unlock() }
}
